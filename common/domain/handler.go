// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination handler_mock.go

package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
)

var (
	errDomainUpdateTooFrequent = &types.ServiceBusyError{Message: "Domain update too frequent."}
	errInvalidDomainName       = &types.BadRequestError{Message: "Domain name can only include alphanumeric and dash characters."}
)

type (
	// Handler is the domain operation handler
	Handler interface {
		DeleteDomain(
			ctx context.Context,
			deleteRequest *types.DeleteDomainRequest,
		) error
		DeprecateDomain(
			ctx context.Context,
			deprecateRequest *types.DeprecateDomainRequest,
		) error
		DescribeDomain(
			ctx context.Context,
			describeRequest *types.DescribeDomainRequest,
		) (*types.DescribeDomainResponse, error)
		ListDomains(
			ctx context.Context,
			listRequest *types.ListDomainsRequest,
		) (*types.ListDomainsResponse, error)
		RegisterDomain(
			ctx context.Context,
			registerRequest *types.RegisterDomainRequest,
		) error
		UpdateDomain(
			ctx context.Context,
			updateRequest *types.UpdateDomainRequest,
		) (*types.UpdateDomainResponse, error)
		FailoverDomain(
			ctx context.Context,
			failoverRequest *types.FailoverDomainRequest,
		) (*types.FailoverDomainResponse, error)
		UpdateIsolationGroups(
			ctx context.Context,
			updateRequest types.UpdateDomainIsolationGroupsRequest,
		) error
		UpdateAsyncWorkflowConfiguraton(
			ctx context.Context,
			updateRequest types.UpdateDomainAsyncWorkflowConfiguratonRequest,
		) error
	}

	// handlerImpl is the domain operation handler implementation
	handlerImpl struct {
		domainManager       persistence.DomainManager
		clusterMetadata     cluster.Metadata
		domainReplicator    Replicator
		domainAttrValidator *AttrValidatorImpl
		archivalMetadata    archiver.ArchivalMetadata
		archiverProvider    provider.ArchiverProvider
		timeSource          clock.TimeSource
		config              Config
		logger              log.Logger
	}

	// Config is the domain config for domain handler
	Config struct {
		MinRetentionDays       dynamicproperties.IntPropertyFn
		MaxRetentionDays       dynamicproperties.IntPropertyFn
		RequiredDomainDataKeys dynamicproperties.MapPropertyFn
		MaxBadBinaryCount      dynamicproperties.IntPropertyFnWithDomainFilter
		FailoverCoolDown       dynamicproperties.DurationPropertyFnWithDomainFilter
		FailoverHistoryMaxSize dynamicproperties.IntPropertyFnWithDomainFilter
	}

	// FailoverEvent is the failover information to be stored for each failover event in domain data
	FailoverEvent struct {
		EventTime time.Time `json:"eventTime"`

		// active-passive domain failover
		FromCluster  string `json:"fromCluster,omitempty"`
		ToCluster    string `json:"toCluster,omitempty"`
		FailoverType string `json:"failoverType,omitempty"`

		// active-active domain failover
		FromActiveClusters types.ActiveClusters `json:"fromActiveClusters,omitempty"`
		ToActiveClusters   types.ActiveClusters `json:"toActiveClusters,omitempty"`
	}

	// FailoverHistory is the history of failovers for a domain limited by the FailoverHistoryMaxSize config
	FailoverHistory struct {
		FailoverEvents []FailoverEvent
	}
)

var _ Handler = (*handlerImpl)(nil)

// NewHandler create a new domain handler
func NewHandler(
	config Config,
	logger log.Logger,
	domainManager persistence.DomainManager,
	clusterMetadata cluster.Metadata,
	domainReplicator Replicator,
	archivalMetadata archiver.ArchivalMetadata,
	archiverProvider provider.ArchiverProvider,
	timeSource clock.TimeSource,
) Handler {
	return &handlerImpl{
		logger:              logger,
		domainManager:       domainManager,
		clusterMetadata:     clusterMetadata,
		domainReplicator:    domainReplicator,
		domainAttrValidator: newAttrValidator(clusterMetadata, int32(config.MinRetentionDays())),
		archivalMetadata:    archivalMetadata,
		archiverProvider:    archiverProvider,
		timeSource:          timeSource,
		config:              config,
	}
}

// RegisterDomain register a new domain
func (d *handlerImpl) RegisterDomain(
	ctx context.Context,
	registerRequest *types.RegisterDomainRequest,
) error {

	// cluster global domain enabled
	if !d.clusterMetadata.IsPrimaryCluster() && registerRequest.GetIsGlobalDomain() {
		return errNotPrimaryCluster
	}

	// first check if the name is already registered as the local domain
	_, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: registerRequest.GetName()})
	switch err.(type) {
	case nil:
		// domain already exists, cannot proceed
		return &types.DomainAlreadyExistsError{Message: "Domain already exists."}
	case *types.EntityNotExistsError:
		// domain does not exists, proceeds
	default:
		// other err
		return err
	}

	// input validation on domain name
	matchedRegex, err := regexp.MatchString("^[a-zA-Z0-9-]+$", registerRequest.GetName())
	if err != nil {
		return err
	}
	if !matchedRegex {
		return errInvalidDomainName
	}

	activeClusterName := d.clusterMetadata.GetCurrentClusterName()
	// input validation on cluster names
	if registerRequest.ActiveClusterName != "" {
		activeClusterName = registerRequest.GetActiveClusterName()
	}
	clusters := []*persistence.ClusterReplicationConfig{}
	for _, clusterConfig := range registerRequest.Clusters {
		clusterName := clusterConfig.GetClusterName()
		clusters = append(clusters, &persistence.ClusterReplicationConfig{ClusterName: clusterName})
	}
	clusters = cluster.GetOrUseDefaultClusters(activeClusterName, clusters)

	currentHistoryArchivalState := neverEnabledState()
	nextHistoryArchivalState := currentHistoryArchivalState
	clusterHistoryArchivalConfig := d.archivalMetadata.GetHistoryConfig()
	if clusterHistoryArchivalConfig.ClusterConfiguredForArchival() {
		archivalEvent, err := d.toArchivalRegisterEvent(
			registerRequest.HistoryArchivalStatus,
			registerRequest.GetHistoryArchivalURI(),
			clusterHistoryArchivalConfig.GetDomainDefaultStatus(),
			clusterHistoryArchivalConfig.GetDomainDefaultURI(),
		)
		if err != nil {
			return err
		}

		nextHistoryArchivalState, _, err = currentHistoryArchivalState.getNextState(archivalEvent, d.validateHistoryArchivalURI)
		if err != nil {
			return err
		}
	}

	currentVisibilityArchivalState := neverEnabledState()
	nextVisibilityArchivalState := currentVisibilityArchivalState
	clusterVisibilityArchivalConfig := d.archivalMetadata.GetVisibilityConfig()
	if clusterVisibilityArchivalConfig.ClusterConfiguredForArchival() {
		archivalEvent, err := d.toArchivalRegisterEvent(
			registerRequest.VisibilityArchivalStatus,
			registerRequest.GetVisibilityArchivalURI(),
			clusterVisibilityArchivalConfig.GetDomainDefaultStatus(),
			clusterVisibilityArchivalConfig.GetDomainDefaultURI(),
		)
		if err != nil {
			return err
		}

		nextVisibilityArchivalState, _, err = currentVisibilityArchivalState.getNextState(archivalEvent, d.validateVisibilityArchivalURI)
		if err != nil {
			return err
		}
	}

	info := &persistence.DomainInfo{
		ID:          uuid.New(),
		Name:        registerRequest.GetName(),
		Status:      persistence.DomainStatusRegistered,
		OwnerEmail:  registerRequest.GetOwnerEmail(),
		Description: registerRequest.GetDescription(),
		Data:        registerRequest.Data,
	}
	config := &persistence.DomainConfig{
		Retention:                registerRequest.GetWorkflowExecutionRetentionPeriodInDays(),
		EmitMetric:               registerRequest.GetEmitMetric(),
		HistoryArchivalStatus:    nextHistoryArchivalState.Status,
		HistoryArchivalURI:       nextHistoryArchivalState.URI,
		VisibilityArchivalStatus: nextVisibilityArchivalState.Status,
		VisibilityArchivalURI:    nextVisibilityArchivalState.URI,
		BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
	}

	activeClusters, err := d.activeClustersFromRegisterRequest(registerRequest)
	if err != nil {
		return err
	}

	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: activeClusterName,
		Clusters:          clusters,
		ActiveClusters:    activeClusters,
	}
	isGlobalDomain := registerRequest.GetIsGlobalDomain()

	if err := d.domainAttrValidator.validateDomainConfig(config); err != nil {
		return err
	}
	if isGlobalDomain {
		if err := d.domainAttrValidator.validateDomainReplicationConfigForGlobalDomain(
			replicationConfig,
		); err != nil {
			return err
		}
	} else {
		if err := d.domainAttrValidator.validateDomainReplicationConfigForLocalDomain(
			replicationConfig,
		); err != nil {
			return err
		}
	}

	failoverVersion := constants.EmptyVersion
	if registerRequest.GetIsGlobalDomain() {
		failoverVersion = d.clusterMetadata.GetNextFailoverVersion(activeClusterName, 0, registerRequest.Name)
	}

	domainRequest := &persistence.CreateDomainRequest{
		Info:              info,
		Config:            config,
		ReplicationConfig: replicationConfig,
		IsGlobalDomain:    isGlobalDomain,
		ConfigVersion:     0,
		FailoverVersion:   failoverVersion,
		LastUpdatedTime:   d.timeSource.Now().UnixNano(),
	}

	domainResponse, err := d.domainManager.CreateDomain(ctx, domainRequest)
	if err != nil {
		return err
	}

	if domainRequest.IsGlobalDomain {
		err = d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationCreate,
			domainRequest.Info,
			domainRequest.Config,
			domainRequest.ReplicationConfig,
			domainRequest.ConfigVersion,
			domainRequest.FailoverVersion,
			constants.InitialPreviousFailoverVersion,
			domainRequest.IsGlobalDomain,
		)
		if err != nil {
			return err
		}
	}

	d.logger.Info("Register domain succeeded",
		tag.WorkflowDomainName(registerRequest.GetName()),
		tag.WorkflowDomainID(domainResponse.ID),
	)

	return nil
}

// ListDomains list all domains
func (d *handlerImpl) ListDomains(
	ctx context.Context,
	listRequest *types.ListDomainsRequest,
) (*types.ListDomainsResponse, error) {

	pageSize := 100
	if listRequest.GetPageSize() != 0 {
		pageSize = int(listRequest.GetPageSize())
	}

	resp, err := d.domainManager.ListDomains(ctx, &persistence.ListDomainsRequest{
		PageSize:      pageSize,
		NextPageToken: listRequest.NextPageToken,
	})

	if err != nil {
		return nil, err
	}

	domains := []*types.DescribeDomainResponse{}
	for _, domain := range resp.Domains {
		desc := &types.DescribeDomainResponse{
			IsGlobalDomain:  domain.IsGlobalDomain,
			FailoverVersion: domain.FailoverVersion,
		}
		desc.DomainInfo, desc.Configuration, desc.ReplicationConfiguration = d.createResponse(domain.Info, domain.Config, domain.ReplicationConfig)
		domains = append(domains, desc)
	}

	response := &types.ListDomainsResponse{
		Domains:       domains,
		NextPageToken: resp.NextPageToken,
	}

	return response, nil
}

// DescribeDomain describe the domain
func (d *handlerImpl) DescribeDomain(
	ctx context.Context,
	describeRequest *types.DescribeDomainRequest,
) (*types.DescribeDomainResponse, error) {

	// TODO, we should migrate the non global domain to new table, see #773
	req := &persistence.GetDomainRequest{
		Name: describeRequest.GetName(),
		ID:   describeRequest.GetUUID(),
	}
	resp, err := d.domainManager.GetDomain(ctx, req)
	if err != nil {
		return nil, err
	}

	response := &types.DescribeDomainResponse{
		IsGlobalDomain:  resp.IsGlobalDomain,
		FailoverVersion: resp.FailoverVersion,
	}
	if resp.FailoverEndTime != nil {
		response.FailoverInfo = &types.FailoverInfo{
			FailoverVersion: resp.FailoverVersion,
			// This reflects that last domain update time. If there is a domain config update, this won't be accurate.
			FailoverStartTimestamp:  resp.LastUpdatedTime,
			FailoverExpireTimestamp: *resp.FailoverEndTime,
		}
	}
	response.DomainInfo, response.Configuration, response.ReplicationConfiguration = d.createResponse(resp.Info, resp.Config, resp.ReplicationConfig)
	return response, nil
}

// UpdateDomain update the domain
func (d *handlerImpl) UpdateDomain(
	ctx context.Context,
	updateRequest *types.UpdateDomainRequest,
) (*types.UpdateDomainResponse, error) {

	// must get the metadata (notificationVersion) first
	// this version can be regarded as the lock on the v2 domain table
	// and since we do not know which table will return the domain afterwards
	// this call has to be made
	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	notificationVersion := metadata.NotificationVersion
	currentDomainState, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()})
	if err != nil {
		return nil, err
	}
	// Check if domain is deprecated
	if currentDomainState.Info != nil && currentDomainState.Info.Status == persistence.DomainStatusDeprecated {
		return nil, errDomainDeprecated
	}

	// todo (david.porter) remove this and push the deepcopy into each of the branches
	getResponse := currentDomainState.DeepCopy()

	info := getResponse.Info
	config := getResponse.Config
	replicationConfig := getResponse.ReplicationConfig
	wasActiveActive := replicationConfig.IsActiveActive()
	configVersion := getResponse.ConfigVersion
	failoverVersion := getResponse.FailoverVersion
	failoverNotificationVersion := getResponse.FailoverNotificationVersion
	isGlobalDomain := getResponse.IsGlobalDomain
	gracefulFailoverEndTime := getResponse.FailoverEndTime
	currentActiveCluster := replicationConfig.ActiveClusterName
	previousFailoverVersion := getResponse.PreviousFailoverVersion
	lastUpdatedTime := time.Unix(0, getResponse.LastUpdatedTime)

	if !isGlobalDomain {
		return d.updateLocalDomain(ctx, updateRequest, getResponse, notificationVersion)
	}
	// todo (active-active) refactor the rest of this method to remove all branching for global domain variations
	// and the split between domain update and failover

	// whether history archival config changed
	historyArchivalConfigChanged := false
	// whether visibility archival config changed
	visibilityArchivalConfigChanged := false
	// whether active cluster is changed
	activeClusterChanged := false
	// whether anything other than active cluster is changed
	configurationChanged := false

	// Update history archival state
	historyArchivalConfigChanged, err = d.updateHistoryArchivalState(config, updateRequest)
	if err != nil {
		return nil, err
	}

	// Update visibility archival state
	visibilityArchivalConfigChanged, err = d.updateVisibilityArchivalState(config, updateRequest)
	if err != nil {
		return nil, err
	}

	// Update domain info
	info, domainInfoChanged := d.updateDomainInfo(
		updateRequest,
		info,
	)

	// Update domain config
	config, domainConfigChanged, err := d.updateDomainConfiguration(
		updateRequest.GetName(),
		config,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	// Update domain bad binary
	config, deleteBinaryChanged, err := d.updateDeleteBadBinary(
		config,
		updateRequest.DeleteBadBinary,
	)
	if err != nil {
		return nil, err
	}

	// Update replication config
	replicationConfig, replicationConfigChanged, activeClusterChanged, err := d.updateReplicationConfig(
		getResponse.Info.Name,
		replicationConfig,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	// Handle graceful failover request
	if updateRequest.FailoverTimeoutInSeconds != nil {
		gracefulFailoverEndTime, previousFailoverVersion, err = d.handleGracefulFailover(
			updateRequest,
			replicationConfig,
			currentActiveCluster,
			gracefulFailoverEndTime,
			failoverVersion,
			activeClusterChanged,
			isGlobalDomain,
		)
		if err != nil {
			return nil, err
		}
	}

	configurationChanged = historyArchivalConfigChanged || visibilityArchivalConfigChanged || domainInfoChanged || domainConfigChanged || deleteBinaryChanged || replicationConfigChanged

	if err = d.domainAttrValidator.validateDomainConfig(config); err != nil {
		return nil, err
	}

	err = d.validateDomainReplicationConfigForUpdateDomain(replicationConfig, isGlobalDomain, configurationChanged, activeClusterChanged)
	if err != nil {
		return nil, err
	}

	if configurationChanged || activeClusterChanged {
		now := d.timeSource.Now()
		// Check the failover cool down time
		if lastUpdatedTime.Add(d.config.FailoverCoolDown(info.Name)).After(now) {
			d.logger.Debugf("Domain was last updated at %v, failoverCoolDown: %v, current time: %v.", lastUpdatedTime, d.config.FailoverCoolDown(info.Name), now)
			return nil, errDomainUpdateTooFrequent
		}

		// set the versions
		if configurationChanged {
			configVersion++
		}

		if activeClusterChanged && isGlobalDomain {
			var failoverType constants.FailoverType = constants.FailoverTypeGrace

			// Force failover cleans graceful failover state
			if updateRequest.FailoverTimeoutInSeconds == nil {
				failoverType = constants.FailoverTypeForce
				gracefulFailoverEndTime = nil
				previousFailoverVersion = constants.InitialPreviousFailoverVersion
			}

			// Cases:
			// 1. active-passive domain's ActiveClusterName is changed
			// 2. active-passive domain is being migrated to active-active
			// 3. active-active domain's ActiveClusters is changed
			isActiveActive := replicationConfig.IsActiveActive()

			// case 1. active-passive domain's ActiveClusterName is changed
			if !wasActiveActive && !isActiveActive {
				failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
					replicationConfig.ActiveClusterName,
					failoverVersion,
					updateRequest.Name,
				)

				d.logger.Debug("active-passive domain failover",
					tag.WorkflowDomainName(info.Name),
					tag.Dynamic("failover-version", failoverVersion),
					tag.Dynamic("failover-type", failoverType),
				)

				err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
					now,
					failoverType,
					&currentActiveCluster,
					updateRequest.ActiveClusterName,
					nil,
					nil,
				))
				if err != nil {
					d.logger.Warn("failed to update failover history", tag.Error(err))
				}
			}

			// case 2. active-passive domain is being migrated to active-active
			if !wasActiveActive && isActiveActive {
				// for active-passive to active-active migration,
				// we increment failover version so top level failoverVersion is updated and domain data is replicated.
				failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
					replicationConfig.ActiveClusterName,
					failoverVersion+1, //todo: (active-active): Let's review if we need to increment
					// this for cluster-attr failover changes. It may not be necessary to increment
					updateRequest.Name,
				)

				d.logger.Debug("active-passive domain is being migrated to active-active",
					tag.WorkflowDomainName(info.Name),
					tag.Dynamic("failover-version", failoverVersion),
					tag.Dynamic("failover-type", failoverType),
				)

				err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
					now,
					failoverType,
					&currentActiveCluster,
					updateRequest.ActiveClusterName,
					nil,
					replicationConfig.ActiveClusters,
				))
				if err != nil {
					d.logger.Warn("failed to update failover history", tag.Error(err))
				}
			}

			// case 3. active-active domain's ActiveClusters is changed
			if wasActiveActive && isActiveActive {
				failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
					replicationConfig.ActiveClusterName,
					failoverVersion+1, //todo: (active-active): Let's review if we need to increment
					// this for cluster-attr failover changes. It may not be necessary to increment
					updateRequest.Name,
				)

				d.logger.Debug("active-active domain failover",
					tag.WorkflowDomainName(info.Name),
					tag.Dynamic("failover-version", failoverVersion),
					tag.Dynamic("failover-type", failoverType),
				)

				err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
					now,
					failoverType,
					&currentActiveCluster,
					updateRequest.ActiveClusterName,
					currentDomainState.ReplicationConfig.GetActiveClusters(),
					replicationConfig.ActiveClusters,
				))
				if err != nil {
					d.logger.Warn("failed to update failover history", tag.Error(err))
				}
			}

			failoverNotificationVersion = notificationVersion
		}

		lastUpdatedTime = now

		updateReq := createUpdateRequest(
			info,
			config,
			replicationConfig,
			configVersion,
			failoverVersion,
			failoverNotificationVersion,
			gracefulFailoverEndTime,
			previousFailoverVersion,
			lastUpdatedTime,
			notificationVersion,
		)

		err = d.domainManager.UpdateDomain(ctx, &updateReq)
		if err != nil {
			return nil, err
		}
	}
	// todo (david.porter) remove this - all domains at this point are global
	// leaving during the refactor just for clarity
	if isGlobalDomain {
		if err = d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationUpdate,
			info,
			config,
			replicationConfig,
			configVersion,
			failoverVersion,
			previousFailoverVersion,
			isGlobalDomain,
		); err != nil {
			return nil, err
		}
	}
	response := &types.UpdateDomainResponse{
		IsGlobalDomain:  isGlobalDomain,
		FailoverVersion: failoverVersion,
	}
	response.DomainInfo, response.Configuration, response.ReplicationConfiguration = d.createResponse(info, config, replicationConfig)

	d.logger.Info("Update domain succeeded",
		tag.WorkflowDomainName(info.Name),
		tag.WorkflowDomainID(info.ID),
	)
	return response, nil
}

func (d *handlerImpl) updateLocalDomain(ctx context.Context,
	updateRequest *types.UpdateDomainRequest,
	currentState *persistence.GetDomainResponse,
	notificationVersion int64,
) (*types.UpdateDomainResponse, error) {

	err := d.domainAttrValidator.validateLocalDomainUpdateRequest(updateRequest)
	if err != nil {
		return nil, err
	}

	// whether history archival config changed
	historyArchivalConfigChanged := false
	// whether visibility archival config changed
	visibilityArchivalConfigChanged := false
	// whether anything other than active cluster is changed
	configurationChanged := false

	intendedDomainState := currentState.DeepCopy()

	configVersion := currentState.ConfigVersion

	now := d.timeSource.Now()

	lastUpdatedTime := time.Unix(0, currentState.LastUpdatedTime)

	// Update history archival state
	historyArchivalConfigChanged, err = d.updateHistoryArchivalState(intendedDomainState.Config, updateRequest)
	if err != nil {
		return nil, err
	}

	// Update visibility archival state
	visibilityArchivalConfigChanged, err = d.updateVisibilityArchivalState(intendedDomainState.Config, updateRequest)
	if err != nil {
		return nil, err
	}

	// Update domain info
	info, domainInfoChanged := d.updateDomainInfo(
		updateRequest,
		intendedDomainState.Info,
	)

	// Update domain config
	config, domainConfigChanged, err := d.updateDomainConfiguration(
		updateRequest.GetName(),
		intendedDomainState.Config,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	// Update domain bad binary
	config, deleteBinaryChanged, err := d.updateDeleteBadBinary(
		config,
		updateRequest.DeleteBadBinary,
	)
	if err != nil {
		return nil, err
	}

	configurationChanged = historyArchivalConfigChanged || visibilityArchivalConfigChanged || domainInfoChanged || domainConfigChanged || deleteBinaryChanged

	if err = d.domainAttrValidator.validateDomainConfig(config); err != nil {
		return nil, err
	}

	if err = d.domainAttrValidator.validateDomainReplicationConfigForLocalDomain(
		intendedDomainState.ReplicationConfig,
	); err != nil {
		return nil, err
	}

	if configurationChanged {
		// set the versions
		if configurationChanged {
			configVersion = intendedDomainState.ConfigVersion + 1
		}

		lastUpdatedTime = now

		updateReq := createUpdateRequest(
			info,
			config,
			intendedDomainState.ReplicationConfig,
			configVersion,
			intendedDomainState.FailoverVersion,
			intendedDomainState.FailoverNotificationVersion,
			intendedDomainState.FailoverEndTime,
			intendedDomainState.PreviousFailoverVersion,
			lastUpdatedTime,
			notificationVersion,
		)

		err = d.domainManager.UpdateDomain(ctx, &updateReq)
		if err != nil {
			return nil, err
		}
	}
	response := &types.UpdateDomainResponse{
		IsGlobalDomain:  false,
		FailoverVersion: intendedDomainState.FailoverVersion,
	}
	response.DomainInfo, response.Configuration, response.ReplicationConfiguration = d.createResponse(info, config, intendedDomainState.ReplicationConfig)

	return response, nil
}

// FailoverDomain handles failover of the domain to a different cluster
func (d *handlerImpl) FailoverDomain(
	ctx context.Context,
	failoverRequest *types.FailoverDomainRequest,
) (*types.FailoverDomainResponse, error) {

	// must get the metadata (notificationVersion) first
	// this version can be regarded as the lock on the v2 domain table
	// and since we do not know which table will return the domain afterwards
	// this call has to be made
	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	notificationVersion := metadata.NotificationVersion
	getResponse, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: failoverRequest.GetDomainName()})
	if err != nil {
		return nil, err
	}

	info := getResponse.Info
	config := getResponse.Config
	replicationConfig := getResponse.ReplicationConfig
	wasActiveActive := replicationConfig.IsActiveActive()
	configVersion := getResponse.ConfigVersion
	failoverVersion := getResponse.FailoverVersion
	failoverNotificationVersion := getResponse.FailoverNotificationVersion
	isGlobalDomain := getResponse.IsGlobalDomain
	gracefulFailoverEndTime := getResponse.FailoverEndTime
	currentActiveCluster := replicationConfig.ActiveClusterName
	currentActiveClusters := replicationConfig.ActiveClusters.DeepCopy()
	previousFailoverVersion := getResponse.PreviousFailoverVersion
	lastUpdatedTime := time.Unix(0, getResponse.LastUpdatedTime)

	updateRequest := &types.UpdateDomainRequest{
		Name:              failoverRequest.DomainName,
		ActiveClusterName: failoverRequest.DomainActiveClusterName,
	}

	// Update replication config
	replicationConfig, replicationConfigChanged, activeClusterChanged, err := d.updateReplicationConfig(
		getResponse.Info.Name,
		replicationConfig,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	// Handle graceful failover request
	if updateRequest.FailoverTimeoutInSeconds != nil {
		gracefulFailoverEndTime, previousFailoverVersion, err = d.handleGracefulFailover(
			updateRequest,
			replicationConfig,
			currentActiveCluster,
			gracefulFailoverEndTime,
			failoverVersion,
			activeClusterChanged,
			isGlobalDomain,
		)
		if err != nil {
			return nil, err
		}
	}

	err = d.validateDomainReplicationConfigForUpdateDomain(replicationConfig, isGlobalDomain, replicationConfigChanged, activeClusterChanged)
	if err != nil {
		return nil, err
	}

	now := d.timeSource.Now()
	// Check the failover cool down time
	if lastUpdatedTime.Add(d.config.FailoverCoolDown(info.Name)).After(now) {
		d.logger.Debugf("Domain was last updated at %v, failoverCoolDown: %v, current time: %v.", lastUpdatedTime, d.config.FailoverCoolDown(info.Name), now)
		return nil, errDomainUpdateTooFrequent
	}

	// set the version
	if replicationConfigChanged {
		configVersion++
	}

	if activeClusterChanged && isGlobalDomain {
		var failoverType constants.FailoverType = constants.FailoverTypeGrace

		// Force failover cleans graceful failover state
		if updateRequest.FailoverTimeoutInSeconds == nil {
			failoverType = constants.FailoverTypeForce
			gracefulFailoverEndTime = nil
			previousFailoverVersion = constants.InitialPreviousFailoverVersion
		}

		// Cases:
		// 1. active-passive domain's ActiveClusterName is changed
		// 2. active-passive domain is being migrated to active-active
		// 3. active-active domain's ActiveClusters is changed
		isActiveActive := replicationConfig.IsActiveActive()

		// case 1. active-passive domain's ActiveClusterName is changed
		if !wasActiveActive && !isActiveActive {
			failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
				replicationConfig.ActiveClusterName,
				failoverVersion,
				updateRequest.Name,
			)

			d.logger.Debug("active-passive domain failover",
				tag.WorkflowDomainName(info.Name),
				tag.Dynamic("failover-version", failoverVersion),
				tag.Dynamic("failover-type", failoverType),
			)

			err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
				now,
				failoverType,
				&currentActiveCluster,
				updateRequest.ActiveClusterName,
				nil,
				nil,
			))
			if err != nil {
				d.logger.Warn("failed to update failover history", tag.Error(err))
			}
		}

		// case 2. active-passive domain is being migrated to active-active
		if !wasActiveActive && isActiveActive {
			// for active-passive to active-active migration,
			// we increment failover version so top level failoverVersion is updated and domain data is replicated.

			failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
				replicationConfig.ActiveClusterName,
				failoverVersion+1, //todo: (active-active): Let's review if we need to increment
				// this for cluster-attr failover changes. It may not be necessary to increment
				updateRequest.Name,
			)

			d.logger.Debug("active-passive domain is being migrated to active-active",
				tag.WorkflowDomainName(info.Name),
				tag.Dynamic("failover-version", failoverVersion),
				tag.Dynamic("failover-type", failoverType),
			)

			err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
				now,
				failoverType,
				&currentActiveCluster,
				updateRequest.ActiveClusterName,
				nil,
				replicationConfig.ActiveClusters,
			))
			if err != nil {
				d.logger.Warn("failed to update failover history", tag.Error(err))
			}
		}

		// case 3. active-active domain's ActiveClusters is changed
		if wasActiveActive && isActiveActive {
			// top level failover version is not used for task versions for active-active domains but we still increment it
			// to indicate there was a change in replication config
			failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
				replicationConfig.ActiveClusterName,
				failoverVersion+1, //todo: (active-active): Let's review if we need to increment
				// this for cluster-attr failover changes. It may not be necessary to increment
				updateRequest.Name,
			)

			d.logger.Debug("active-active domain failover",
				tag.WorkflowDomainName(info.Name),
				tag.Dynamic("failover-version", failoverVersion),
				tag.Dynamic("failover-type", failoverType),
			)

			err = updateFailoverHistoryInDomainData(info, d.config, NewFailoverEvent(
				now,
				failoverType,
				&currentActiveCluster,
				nil,
				currentActiveClusters,
				replicationConfig.ActiveClusters,
			))
			if err != nil {
				d.logger.Warn("failed to update failover history", tag.Error(err))
			}
		}

		failoverNotificationVersion = notificationVersion
	}

	lastUpdatedTime = now

	updateReq := createUpdateRequest(
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		failoverNotificationVersion,
		gracefulFailoverEndTime,
		previousFailoverVersion,
		lastUpdatedTime,
		notificationVersion,
	)

	err = d.domainManager.UpdateDomain(ctx, &updateReq)
	if err != nil {
		d.logger.Info("Update domain's replication configs failed",
			tag.WorkflowDomainName(info.Name),
			tag.WorkflowDomainID(info.ID),
		)
		return nil, err
	}

	if isGlobalDomain {
		if err = d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationUpdate,
			info,
			config,
			replicationConfig,
			configVersion,
			failoverVersion,
			previousFailoverVersion,
			isGlobalDomain,
		); err != nil {
			return nil, err
		}
	}

	domainInfo, configuration, replicationConfiguration := d.createResponse(info, config, replicationConfig)

	d.logger.Info("Update domain's replication configs succeeded",
		tag.WorkflowDomainName(info.Name),
		tag.WorkflowDomainID(info.ID),
	)
	return &types.FailoverDomainResponse{
		DomainInfo:               domainInfo,
		Configuration:            configuration,
		ReplicationConfiguration: replicationConfiguration,
		FailoverVersion:          failoverVersion,
		IsGlobalDomain:           isGlobalDomain,
	}, nil
}

// DeleteDomain deletes a domain
func (d *handlerImpl) DeleteDomain(
	ctx context.Context,
	deleteRequest *types.DeleteDomainRequest,
) error {
	getResponse, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: deleteRequest.GetName()})
	if err != nil {
		return err
	}
	isGlobalDomain := getResponse.IsGlobalDomain
	if isGlobalDomain && !d.clusterMetadata.IsPrimaryCluster() {
		return errNotPrimaryCluster
	}

	deleteReq := &persistence.DeleteDomainByNameRequest{
		Name: getResponse.Info.Name,
	}
	err = d.domainManager.DeleteDomainByName(ctx, deleteReq)
	if err != nil {
		return err
	}

	if isGlobalDomain {
		if err := d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationDelete,
			getResponse.Info,
			getResponse.Config,
			getResponse.ReplicationConfig,
			getResponse.ConfigVersion,
			getResponse.FailoverVersion,
			getResponse.PreviousFailoverVersion,
			isGlobalDomain,
		); err != nil {
			return fmt.Errorf("unable to delete a domain in replica cluster: %v", err)
		}
	}

	d.logger.Info("Delete domain succeeded",
		tag.WorkflowDomainName(getResponse.Info.Name),
		tag.WorkflowDomainID(getResponse.Info.ID),
	)
	return nil
}

// DeprecateDomain deprecates a domain
func (d *handlerImpl) DeprecateDomain(
	ctx context.Context,
	deprecateRequest *types.DeprecateDomainRequest,
) error {

	// must get the metadata (notificationVersion) first
	// this version can be regarded as the lock on the v2 domain table
	// and since we do not know which table will return the domain afterwards
	// this call has to be made
	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return err
	}
	notificationVersion := metadata.NotificationVersion
	getResponse, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: deprecateRequest.GetName()})
	if err != nil {
		return err
	}

	isGlobalDomain := getResponse.IsGlobalDomain
	if isGlobalDomain && !d.clusterMetadata.IsPrimaryCluster() {
		return errNotPrimaryCluster
	}
	getResponse.ConfigVersion = getResponse.ConfigVersion + 1
	getResponse.Info.Status = persistence.DomainStatusDeprecated

	updateReq := createUpdateRequest(
		getResponse.Info,
		getResponse.Config,
		getResponse.ReplicationConfig,
		getResponse.ConfigVersion,
		getResponse.FailoverVersion,
		getResponse.FailoverNotificationVersion,
		getResponse.FailoverEndTime,
		getResponse.PreviousFailoverVersion,
		d.timeSource.Now(),
		notificationVersion,
	)

	err = d.domainManager.UpdateDomain(ctx, &updateReq)
	if err != nil {
		return err
	}

	if isGlobalDomain {
		if err := d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationUpdate,
			getResponse.Info,
			getResponse.Config,
			getResponse.ReplicationConfig,
			getResponse.ConfigVersion,
			getResponse.FailoverVersion,
			getResponse.PreviousFailoverVersion,
			isGlobalDomain,
		); err != nil {
			return err
		}
	}

	d.logger.Info("DeprecateDomain domain succeeded",
		tag.WorkflowDomainName(getResponse.Info.Name),
		tag.WorkflowDomainID(getResponse.Info.ID),
	)
	return nil
}

// UpdateIsolationGroups is used for draining and undraining of isolation-groups for a domain.
// Like the isolation-group API, this controller expects Upsert semantics for
// isolation-groups and does not modify any other domain information.
//
// Isolation-groups are regional in their configuration scope, so it's expected that this upsert
// includes configuration for both clusters every time.
//
// The update is handled like other domain updates in that they expected to be replicated. So
// unlike the global isolation-group API it shouldn't be necessary to call
func (d *handlerImpl) UpdateIsolationGroups(
	ctx context.Context,
	updateRequest types.UpdateDomainIsolationGroupsRequest,
) error {

	// must get the metadata (notificationVersion) first
	// this version can be regarded as the lock on the v2 domain table
	// and since we do not know which table will return the domain afterwards
	// this call has to be made
	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return err
	}
	notificationVersion := metadata.NotificationVersion

	if updateRequest.IsolationGroups == nil {
		return fmt.Errorf("invalid request, isolationGroup configuration must be set: Got: %v", updateRequest)
	}

	currentDomainConfig, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.Domain})
	if err != nil {
		return err
	}
	if currentDomainConfig.Config == nil {
		return fmt.Errorf("unable to load config for domain as expected")
	}

	configVersion := currentDomainConfig.ConfigVersion
	lastUpdatedTime := time.Unix(0, currentDomainConfig.LastUpdatedTime)

	// Check the failover cool down time
	if lastUpdatedTime.Add(d.config.FailoverCoolDown(currentDomainConfig.Info.Name)).After(d.timeSource.Now()) {
		return errDomainUpdateTooFrequent
	}

	if !d.clusterMetadata.IsPrimaryCluster() && currentDomainConfig.IsGlobalDomain {
		return errNotPrimaryCluster
	}

	configVersion++
	lastUpdatedTime = d.timeSource.Now()

	// Mutate the domain config to perform the isolation-group update
	currentDomainConfig.Config.IsolationGroups = updateRequest.IsolationGroups

	updateReq := createUpdateRequest(
		currentDomainConfig.Info,
		currentDomainConfig.Config,
		currentDomainConfig.ReplicationConfig,
		configVersion,
		currentDomainConfig.FailoverVersion,
		currentDomainConfig.FailoverNotificationVersion,
		currentDomainConfig.FailoverEndTime,
		currentDomainConfig.PreviousFailoverVersion,
		lastUpdatedTime,
		notificationVersion,
	)

	err = d.domainManager.UpdateDomain(ctx, &updateReq)
	if err != nil {
		return err
	}

	if currentDomainConfig.IsGlobalDomain {
		// One might reasonably wonder what value there is in replication of isolation-group information - info which is
		// regional and therefore of no value to the other region?
		// Probably not a lot, in and of itself, however, the isolation-group information is stored
		// in the domain configuration fields in the domain tables. Access and updates to those records is
		// done through a replicated mechanism with explicit versioning and conflict resolution.
		// Therefore, in order to avoid making an already complex mechanisim much more difficult to understand,
		// the data is replicated in the same way so as to try and make things less confusing when both codepaths
		// are updating the table:
		// - versions like the confiugration version are updated in the same manner
		// - the last-updated timestamps are updated in the same manner
		if err := d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationUpdate,
			currentDomainConfig.Info,
			currentDomainConfig.Config,
			currentDomainConfig.ReplicationConfig,
			configVersion,
			currentDomainConfig.FailoverVersion,
			currentDomainConfig.PreviousFailoverVersion,
			currentDomainConfig.IsGlobalDomain,
		); err != nil {
			return err
		}
	}

	d.logger.Info("isolation group update succeeded",
		tag.WorkflowDomainName(currentDomainConfig.Info.Name),
		tag.WorkflowDomainID(currentDomainConfig.Info.ID),
	)
	return nil
}

func (d *handlerImpl) UpdateAsyncWorkflowConfiguraton(
	ctx context.Context,
	updateRequest types.UpdateDomainAsyncWorkflowConfiguratonRequest,
) error {
	// must get the metadata (notificationVersion) first
	// this version can be regarded as the lock on the v2 domain table
	// and since we do not know which table will return the domain afterwards
	// this call has to be made
	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return err
	}
	notificationVersion := metadata.NotificationVersion

	currentDomainConfig, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.Domain})
	if err != nil {
		return err
	}
	if currentDomainConfig.Config == nil {
		return fmt.Errorf("unable to load config for domain as expected")
	}

	configVersion := currentDomainConfig.ConfigVersion
	lastUpdatedTime := time.Unix(0, currentDomainConfig.LastUpdatedTime)

	// Check the failover cool down time
	if lastUpdatedTime.Add(d.config.FailoverCoolDown(currentDomainConfig.Info.Name)).After(d.timeSource.Now()) {
		return errDomainUpdateTooFrequent
	}

	if !d.clusterMetadata.IsPrimaryCluster() && currentDomainConfig.IsGlobalDomain {
		return errNotPrimaryCluster
	}

	configVersion++
	lastUpdatedTime = d.timeSource.Now()

	// Mutate the domain config to perform the async wf config update
	if updateRequest.Configuration == nil {
		// this is a delete request so empty all the fields
		currentDomainConfig.Config.AsyncWorkflowConfig = types.AsyncWorkflowConfiguration{}
	} else {
		currentDomainConfig.Config.AsyncWorkflowConfig = *updateRequest.Configuration
	}

	d.logger.Debug("async workflow queue config update", tag.Dynamic("config", currentDomainConfig))

	updateReq := createUpdateRequest(
		currentDomainConfig.Info,
		currentDomainConfig.Config,
		currentDomainConfig.ReplicationConfig,
		configVersion,
		currentDomainConfig.FailoverVersion,
		currentDomainConfig.FailoverNotificationVersion,
		currentDomainConfig.FailoverEndTime,
		currentDomainConfig.PreviousFailoverVersion,
		lastUpdatedTime,
		notificationVersion,
	)

	err = d.domainManager.UpdateDomain(ctx, &updateReq)
	if err != nil {
		return err
	}

	if currentDomainConfig.IsGlobalDomain {
		// One might reasonably wonder what value there is in replication of isolation-group information - info which is
		// regional and therefore of no value to the other region?
		// Probably not a lot, in and of itself, however, the isolation-group information is stored
		// in the domain configuration fields in the domain tables. Access and updates to those records is
		// done through a replicated mechanism with explicit versioning and conflict resolution.
		// Therefore, in order to avoid making an already complex mechanisim much more difficult to understand,
		// the data is replicated in the same way so as to try and make things less confusing when both codepaths
		// are updating the table:
		// - versions like the confiugration version are updated in the same manner
		// - the last-updated timestamps are updated in the same manner
		if err := d.domainReplicator.HandleTransmissionTask(
			ctx,
			types.DomainOperationUpdate,
			currentDomainConfig.Info,
			currentDomainConfig.Config,
			currentDomainConfig.ReplicationConfig,
			configVersion,
			currentDomainConfig.FailoverVersion,
			currentDomainConfig.PreviousFailoverVersion,
			currentDomainConfig.IsGlobalDomain,
		); err != nil {
			return err
		}
	}

	d.logger.Info("async workflow queue config update succeeded",
		tag.WorkflowDomainName(currentDomainConfig.Info.Name),
		tag.WorkflowDomainID(currentDomainConfig.Info.ID),
	)
	return nil
}

func (d *handlerImpl) createResponse(
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	replicationConfig *persistence.DomainReplicationConfig,
) (*types.DomainInfo, *types.DomainConfiguration, *types.DomainReplicationConfiguration) {

	infoResult := &types.DomainInfo{
		Name:        info.Name,
		Status:      getDomainStatus(info),
		Description: info.Description,
		OwnerEmail:  info.OwnerEmail,
		Data:        info.Data,
		UUID:        info.ID,
	}

	configResult := &types.DomainConfiguration{
		EmitMetric:                             config.EmitMetric,
		WorkflowExecutionRetentionPeriodInDays: config.Retention,
		HistoryArchivalStatus:                  config.HistoryArchivalStatus.Ptr(),
		HistoryArchivalURI:                     config.HistoryArchivalURI,
		VisibilityArchivalStatus:               config.VisibilityArchivalStatus.Ptr(),
		VisibilityArchivalURI:                  config.VisibilityArchivalURI,
		BadBinaries:                            &config.BadBinaries,
		IsolationGroups:                        &config.IsolationGroups,
		AsyncWorkflowConfig:                    &config.AsyncWorkflowConfig,
	}

	clusters := []*types.ClusterReplicationConfiguration{}
	for _, cluster := range replicationConfig.Clusters {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: cluster.ClusterName,
		})
	}

	replicationConfigResult := &types.DomainReplicationConfiguration{
		ActiveClusterName: replicationConfig.ActiveClusterName,
		Clusters:          clusters,
		ActiveClusters:    replicationConfig.ActiveClusters,
	}

	return infoResult, configResult, replicationConfigResult
}

func (d *handlerImpl) mergeBadBinaries(
	old map[string]*types.BadBinaryInfo,
	new map[string]*types.BadBinaryInfo,
	createTimeNano int64,
) types.BadBinaries {

	if old == nil {
		old = map[string]*types.BadBinaryInfo{}
	}
	for k, v := range new {
		v.CreatedTimeNano = common.Int64Ptr(createTimeNano)
		old[k] = v
	}
	return types.BadBinaries{
		Binaries: old,
	}
}

func (d *handlerImpl) mergeDomainData(
	old map[string]string,
	new map[string]string,
) map[string]string {

	if old == nil {
		old = map[string]string{}
	}
	for k, v := range new {
		old[k] = v
	}
	return old
}

func (d *handlerImpl) toArchivalRegisterEvent(
	status *types.ArchivalStatus,
	URI string,
	defaultStatus types.ArchivalStatus,
	defaultURI string,
) (*ArchivalEvent, error) {

	event := &ArchivalEvent{
		status:     status,
		URI:        URI,
		defaultURI: defaultURI,
	}
	if event.status == nil {
		event.status = defaultStatus.Ptr()
	}
	if err := event.validate(); err != nil {
		return nil, err
	}
	return event, nil
}

func (d *handlerImpl) toArchivalUpdateEvent(
	status *types.ArchivalStatus,
	URI string,
	defaultURI string,
) (*ArchivalEvent, error) {

	event := &ArchivalEvent{
		status:     status,
		URI:        URI,
		defaultURI: defaultURI,
	}
	if err := event.validate(); err != nil {
		return nil, err
	}
	return event, nil
}

func (d *handlerImpl) validateHistoryArchivalURI(URIString string) error {
	URI, err := archiver.NewURI(URIString)
	if err != nil {
		return err
	}

	archiver, err := d.archiverProvider.GetHistoryArchiver(URI.Scheme(), service.Frontend)
	if err != nil {
		return err
	}

	return archiver.ValidateURI(URI)
}

func (d *handlerImpl) validateVisibilityArchivalURI(URIString string) error {
	URI, err := archiver.NewURI(URIString)
	if err != nil {
		return err
	}

	archiver, err := d.archiverProvider.GetVisibilityArchiver(URI.Scheme(), service.Frontend)
	if err != nil {
		return err
	}

	return archiver.ValidateURI(URI)
}

func (d *handlerImpl) getHistoryArchivalState(
	config *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (*ArchivalState, bool, error) {

	currentHistoryArchivalState := &ArchivalState{
		Status: config.HistoryArchivalStatus,
		URI:    config.HistoryArchivalURI,
	}
	clusterHistoryArchivalConfig := d.archivalMetadata.GetHistoryConfig()

	if clusterHistoryArchivalConfig.ClusterConfiguredForArchival() {
		archivalEvent, err := d.toArchivalUpdateEvent(
			updateRequest.HistoryArchivalStatus,
			updateRequest.GetHistoryArchivalURI(),
			clusterHistoryArchivalConfig.GetDomainDefaultURI(),
		)
		if err != nil {
			return currentHistoryArchivalState, false, err
		}
		return currentHistoryArchivalState.getNextState(archivalEvent, d.validateHistoryArchivalURI)
	}
	return currentHistoryArchivalState, false, nil
}

func (d *handlerImpl) updateHistoryArchivalState(
	config *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (bool, error) {
	historyArchivalState, changed, err := d.getHistoryArchivalState(config, updateRequest)
	if err != nil {
		return false, err
	}

	if changed {
		config.HistoryArchivalStatus = historyArchivalState.Status
		config.HistoryArchivalURI = historyArchivalState.URI
	}

	return changed, nil
}

func (d *handlerImpl) getVisibilityArchivalState(
	config *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (*ArchivalState, bool, error) {
	currentVisibilityArchivalState := &ArchivalState{
		Status: config.VisibilityArchivalStatus,
		URI:    config.VisibilityArchivalURI,
	}
	clusterVisibilityArchivalConfig := d.archivalMetadata.GetVisibilityConfig()
	if clusterVisibilityArchivalConfig.ClusterConfiguredForArchival() {
		archivalEvent, err := d.toArchivalUpdateEvent(
			updateRequest.VisibilityArchivalStatus,
			updateRequest.GetVisibilityArchivalURI(),
			clusterVisibilityArchivalConfig.GetDomainDefaultURI(),
		)
		if err != nil {
			return currentVisibilityArchivalState, false, err
		}
		return currentVisibilityArchivalState.getNextState(archivalEvent, d.validateVisibilityArchivalURI)
	}
	return currentVisibilityArchivalState, false, nil
}

func (d *handlerImpl) updateVisibilityArchivalState(
	config *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (bool, error) {
	visibilityArchivalState, changed, err := d.getVisibilityArchivalState(
		config,
		updateRequest,
	)
	if err != nil {
		return false, err
	}

	if changed {
		config.VisibilityArchivalStatus = visibilityArchivalState.Status
		config.VisibilityArchivalURI = visibilityArchivalState.URI
	}

	return changed, nil
}

func (d *handlerImpl) updateDomainInfo(
	updateRequest *types.UpdateDomainRequest,
	currentDomainInfo *persistence.DomainInfo,
) (*persistence.DomainInfo, bool) {

	isDomainUpdated := false
	if updateRequest.Description != nil {
		isDomainUpdated = true
		currentDomainInfo.Description = *updateRequest.Description
	}
	if updateRequest.OwnerEmail != nil {
		isDomainUpdated = true
		currentDomainInfo.OwnerEmail = *updateRequest.OwnerEmail
	}
	if updateRequest.Data != nil {
		isDomainUpdated = true
		// only do merging
		currentDomainInfo.Data = d.mergeDomainData(currentDomainInfo.Data, updateRequest.Data)
	}
	return currentDomainInfo, isDomainUpdated
}

func (d *handlerImpl) updateDomainConfiguration(
	domainName string,
	config *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (*persistence.DomainConfig, bool, error) {

	isConfigChanged := false
	if updateRequest.EmitMetric != nil {
		isConfigChanged = true
		config.EmitMetric = *updateRequest.EmitMetric
	}
	if updateRequest.WorkflowExecutionRetentionPeriodInDays != nil {
		isConfigChanged = true
		config.Retention = *updateRequest.WorkflowExecutionRetentionPeriodInDays
	}
	if updateRequest.BadBinaries != nil {
		maxLength := d.config.MaxBadBinaryCount(domainName)
		// only do merging
		config.BadBinaries = d.mergeBadBinaries(config.BadBinaries.Binaries, updateRequest.BadBinaries.Binaries, d.timeSource.Now().UnixNano())
		if len(config.BadBinaries.Binaries) > maxLength {
			return config, isConfigChanged, &types.BadRequestError{
				Message: fmt.Sprintf("Total resetBinaries cannot exceed the max limit: %v", maxLength),
			}
		}
	}
	return config, isConfigChanged, nil
}

func (d *handlerImpl) updateDeleteBadBinary(
	config *persistence.DomainConfig,
	deleteBadBinary *string,
) (*persistence.DomainConfig, bool, error) {

	if deleteBadBinary != nil {
		_, ok := config.BadBinaries.Binaries[*deleteBadBinary]
		if !ok {
			return config, false, &types.BadRequestError{
				Message: fmt.Sprintf("Bad binary checksum %v doesn't exists.", *deleteBadBinary),
			}
		}
		delete(config.BadBinaries.Binaries, *deleteBadBinary)
		return config, true, nil
	}
	return config, false, nil
}

// updateReplicationConfig is the function which takes the input request and current state and edits and returns it
// to the desired state by merging the request values with the current state.
// replicationConfigChanged being turned on will trigger an increment in the configVersion.
// activeClusterChanged indicates a failover is happening and a failover version is to be incremented
func (d *handlerImpl) updateReplicationConfig(
	domainName string,
	config *persistence.DomainReplicationConfig,
	updateRequest *types.UpdateDomainRequest,
) (
	mutatedCfg *persistence.DomainReplicationConfig,
	replicationConfigChanged bool,
	activeClusterChanged bool,
	err error,
) {

	if len(updateRequest.Clusters) != 0 {
		replicationConfigChanged = true
		clustersNew := []*persistence.ClusterReplicationConfig{}
		for _, clusterConfig := range updateRequest.Clusters {
			clustersNew = append(clustersNew, &persistence.ClusterReplicationConfig{
				ClusterName: clusterConfig.GetClusterName(),
			})
		}

		if err := d.domainAttrValidator.validateDomainReplicationConfigClustersDoesNotRemove(
			config.Clusters,
			clustersNew,
		); err != nil {
			d.logger.Warn("removing replica clusters from domain replication group", tag.Error(err))
		}
		config.Clusters = clustersNew
	}

	if updateRequest.ActiveClusterName != nil {
		activeClusterChanged = true
		config.ActiveClusterName = *updateRequest.ActiveClusterName
	}

	err = d.domainAttrValidator.validateActiveActiveDomainReplicationConfig(updateRequest.ActiveClusters)
	if err != nil {
		return nil, false, false, err
	}

	if updateRequest != nil && updateRequest.ActiveClusters != nil && updateRequest.ActiveClusters.AttributeScopes != nil {
		result, isCh := d.buildActiveActiveClusterScopesFromUpdateRequest(updateRequest, config, domainName)
		if isCh {

			if config.ActiveClusters == nil {
				config.ActiveClusters = &types.ActiveClusters{
					AttributeScopes: result.AttributeScopes,
				}
			}

			config.ActiveClusters.AttributeScopes = result.AttributeScopes
			activeClusterChanged = true
			replicationConfigChanged = true
		}
	}

	return config, replicationConfigChanged, activeClusterChanged, nil
}

func (d *handlerImpl) handleGracefulFailover(
	updateRequest *types.UpdateDomainRequest,
	replicationConfig *persistence.DomainReplicationConfig,
	currentActiveCluster string,
	gracefulFailoverEndTime *int64,
	failoverVersion int64,
	activeClusterChanged bool,
	isGlobalDomain bool,
) (*int64, int64, error) {
	// must update active cluster on a global domain
	if !activeClusterChanged || !isGlobalDomain || replicationConfig.IsActiveActive() {
		return nil, 0, errInvalidGracefulFailover
	}
	// must start with the passive -> active cluster
	if replicationConfig.ActiveClusterName != d.clusterMetadata.GetCurrentClusterName() {
		return nil, 0, errCannotDoGracefulFailoverFromCluster
	}
	if replicationConfig.ActiveClusterName == currentActiveCluster {
		return nil, 0, errGracefulFailoverInActiveCluster
	}
	// cannot have concurrent failover
	if gracefulFailoverEndTime != nil {
		return nil, 0, errOngoingGracefulFailover
	}
	endTime := d.timeSource.Now().Add(time.Duration(updateRequest.GetFailoverTimeoutInSeconds()) * time.Second).UnixNano()
	gracefulFailoverEndTime = &endTime
	previousFailoverVersion := failoverVersion

	return gracefulFailoverEndTime, previousFailoverVersion, nil
}

func (d *handlerImpl) validateDomainReplicationConfigForUpdateDomain(
	replicationConfig *persistence.DomainReplicationConfig,
	isGlobalDomain bool,
	configurationChanged bool,
	activeClusterChanged bool,
) error {
	var err error
	if isGlobalDomain {
		if err = d.domainAttrValidator.validateDomainReplicationConfigForGlobalDomain(
			replicationConfig,
		); err != nil {
			return err
		}

		if configurationChanged && activeClusterChanged && !replicationConfig.IsActiveActive() {
			return errCannotDoDomainFailoverAndUpdate
		}

		if !activeClusterChanged && !d.clusterMetadata.IsPrimaryCluster() {
			return errNotPrimaryCluster
		}
	} else {
		if err = d.domainAttrValidator.validateDomainReplicationConfigForLocalDomain(
			replicationConfig,
		); err != nil {
			return err
		}
	}

	return nil
}

func (d *handlerImpl) activeClustersFromRegisterRequest(registerRequest *types.RegisterDomainRequest) (*types.ActiveClusters, error) {
	if !registerRequest.GetIsGlobalDomain() || registerRequest.ActiveClusters == nil {
		// local or active-passive domain
		return nil, nil
	}

	clusters := d.clusterMetadata.GetAllClusterInfo()

	activeClustersScopes := make(map[string]types.ClusterAttributeScope)

	// Handle AttributeScopes from the request
	if registerRequest.ActiveClusters != nil && registerRequest.ActiveClusters.AttributeScopes != nil {
		for scope, scopeData := range registerRequest.ActiveClusters.AttributeScopes {
			newScopeData := types.ClusterAttributeScope{
				ClusterAttributes: make(map[string]types.ActiveClusterInfo),
			}
			for attribute, clusterInfo := range scopeData.ClusterAttributes {
				clusterMetadata, ok := clusters[clusterInfo.ActiveClusterName]
				if !ok {
					return nil, &types.BadRequestError{
						Message: fmt.Sprintf("Cluster %v not found. Domain cannot be registered in this cluster for scope %q and attribute %q", clusterInfo.ActiveClusterName, scope, attribute),
					}
				}
				newScopeData.ClusterAttributes[attribute] = types.ActiveClusterInfo{
					ActiveClusterName: clusterInfo.ActiveClusterName,
					FailoverVersion:   clusterMetadata.InitialFailoverVersion,
				}
			}
			activeClustersScopes[scope] = newScopeData
		}
	}

	if len(activeClustersScopes) == 0 {
		return nil, nil
	}

	return &types.ActiveClusters{
		AttributeScopes: activeClustersScopes,
	}, nil
}

func getDomainStatus(info *persistence.DomainInfo) *types.DomainStatus {
	switch info.Status {
	case persistence.DomainStatusRegistered:
		v := types.DomainStatusRegistered
		return &v
	case persistence.DomainStatusDeprecated:
		v := types.DomainStatusDeprecated
		return &v
	case persistence.DomainStatusDeleted:
		v := types.DomainStatusDeleted
		return &v
	}

	return nil
}

// Maps fields onto an updateDomain Request
// it's really important that this explicitly calls out each field to ensure no fields get missed or dropped
func createUpdateRequest(
	info *persistence.DomainInfo,
	config *persistence.DomainConfig,
	replicationConfig *persistence.DomainReplicationConfig,
	configVersion int64,
	failoverVersion int64,
	failoverNotificationVersion int64,
	failoverEndTime *int64,
	previousFailoverVersion int64,
	lastUpdatedTime time.Time,
	notificationVersion int64,
) persistence.UpdateDomainRequest {
	return persistence.UpdateDomainRequest{
		Info:                        info,
		Config:                      config,
		ReplicationConfig:           replicationConfig,
		ConfigVersion:               configVersion,
		FailoverVersion:             failoverVersion,
		FailoverNotificationVersion: failoverNotificationVersion,
		FailoverEndTime:             failoverEndTime,
		PreviousFailoverVersion:     previousFailoverVersion,
		LastUpdatedTime:             lastUpdatedTime.UnixNano(),
		NotificationVersion:         notificationVersion,
	}
}

func updateFailoverHistoryInDomainData(
	info *persistence.DomainInfo,
	config Config,
	failoverEvent FailoverEvent,
) error {
	data := info.Data
	if info.Data == nil {
		data = make(map[string]string)
	}

	var failoverHistory []FailoverEvent
	_ = json.Unmarshal([]byte(data[constants.DomainDataKeyForFailoverHistory]), &failoverHistory)

	failoverHistory = append([]FailoverEvent{failoverEvent}, failoverHistory...)

	// Truncate the history to the max size
	failoverHistoryJSON, err := json.Marshal(failoverHistory[:min(config.FailoverHistoryMaxSize(info.Name), len(failoverHistory))])
	if err != nil {
		return err
	}
	data[constants.DomainDataKeyForFailoverHistory] = string(failoverHistoryJSON)

	info.Data = data

	return nil
}

func NewFailoverEvent(
	eventTime time.Time,
	failoverType constants.FailoverType,
	fromCluster *string,
	toCluster *string,
	fromActiveClusters *types.ActiveClusters,
	toActiveClusters *types.ActiveClusters,
) FailoverEvent {
	res := FailoverEvent{
		EventTime:    eventTime,
		FailoverType: failoverType.String(),
	}
	if fromCluster != nil {
		res.FromCluster = *fromCluster
	}
	if toCluster != nil {
		res.ToCluster = *toCluster
	}
	if fromActiveClusters != nil {
		res.FromActiveClusters = *fromActiveClusters
	}
	if toActiveClusters != nil {
		res.ToActiveClusters = *toActiveClusters
	}
	return res
}

func (d *handlerImpl) buildActiveActiveClusterScopesFromUpdateRequest(updateRequest *types.UpdateDomainRequest, config *persistence.DomainReplicationConfig, domainName string) (out *types.ActiveClusters, isChanged bool) {
	var existing *types.ActiveClusters
	if config != nil && config.ActiveClusters != nil {
		existing = config.ActiveClusters
	}

	if updateRequest.ActiveClusters == nil || updateRequest.ActiveClusters.AttributeScopes == nil {
		return existing, false
	}

	// ensure a failover version is set for the incoming request
	for scope, scopeData := range updateRequest.ActiveClusters.AttributeScopes {
		for attribute, activeCluster := range scopeData.ClusterAttributes {

			currentFailoverVersion := types.UndefinedFailoverVersion
			if config != nil && config.ActiveClusters != nil {
				fo, err := config.ActiveClusters.GetFailoverVersionForAttribute(scope, attribute)
				if err == nil {
					currentFailoverVersion = fo
				}
			}
			nextFailoverVersion := d.clusterMetadata.GetNextFailoverVersion(activeCluster.ActiveClusterName, currentFailoverVersion, domainName)

			activeCluster.FailoverVersion = nextFailoverVersion
			scopeData.ClusterAttributes[attribute] = activeCluster
		}
	}

	// if there's no existing active cluster info, use what's in the request
	if existing == nil {
		return updateRequest.ActiveClusters, true
	}

	// if, on the other hand, there are existing active clusters, merge them with the incoming request
	result, isChanged := mergeActiveActiveScopes(config.ActiveClusters, updateRequest.ActiveClusters)
	if isChanged {
		return result, isChanged
	}

	return config.ActiveClusters, false
}
