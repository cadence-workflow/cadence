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
	"fmt"
	"regexp"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
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
		MinRetentionDays       dynamicconfig.IntPropertyFn
		MaxRetentionDays       dynamicconfig.IntPropertyFn
		RequiredDomainDataKeys dynamicconfig.MapPropertyFn
		MaxBadBinaryCount      dynamicconfig.IntPropertyFnWithDomainFilter
		FailoverCoolDown       dynamicconfig.DurationPropertyFnWithDomainFilter
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
	replicationConfig := &persistence.DomainReplicationConfig{
		ActiveClusterName: activeClusterName,
		Clusters:          clusters,
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

	failoverVersion := common.EmptyVersion
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
			common.InitialPreviousFailoverVersion,
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
	getResponse, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: updateRequest.GetName()})
	if err != nil {
		return nil, err
	}

	info := getResponse.Info
	config := getResponse.Config
	replicationConfig := getResponse.ReplicationConfig
	configVersion := getResponse.ConfigVersion
	failoverVersion := getResponse.FailoverVersion
	failoverNotificationVersion := getResponse.FailoverNotificationVersion
	isGlobalDomain := getResponse.IsGlobalDomain
	gracefulFailoverEndTime := getResponse.FailoverEndTime
	currentActiveCluster := replicationConfig.ActiveClusterName
	previousFailoverVersion := getResponse.PreviousFailoverVersion
	lastUpdatedTime := time.Unix(0, getResponse.LastUpdatedTime)

	// whether history archival config changed
	historyArchivalConfigChanged := false
	// whether visibility archival config changed
	visibilityArchivalConfigChanged := false
	// whether active cluster is changed
	activeClusterChanged := false

	// Whether isolation-groups are changed. There's semantic API difference between the
	// domain isolation-group update API which is a flat upsert and this handler API which
	// relies on modifying specified values only, and leaving the rest as-is.
	// for callers of the Domain API wishing to update isolation groups,
	// To distinguish between the operation to remove all drains and
	// take no action, the UpdateDomain handler uses the presence or nil-ness of the IsolationGroup
	// field to indicate either drain removal (where the struct is present), and to take no action
	// where the field is nil.
	//
	// ie, for the purposes of this handler:
	// h.UpdateDomain(ctx, UpdateDomainRequest{
	//   IsolationGroups: nil,                              // take no action for isolation groups
	// })
	// h.UpdateDomain(ctx, UpdateDomainRequest{
	//   IsolationGroups: IsolationGroupConfiguration{},    // remove all isolation groups for domain
	// })
	config, isolationGroupConfigurationChanged, err := d.getIsolationGroupStatus(config, updateRequest)
	// whether anything other than active cluster is changed
	configurationChanged := false

	// Update history archival state
	historyArchivalState, historyArchivalConfigChanged, err := d.getHistoryArchivalState(
		config,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}
	if historyArchivalConfigChanged {
		config.HistoryArchivalStatus = historyArchivalState.Status
		config.HistoryArchivalURI = historyArchivalState.URI
	}

	// Update visibility archival state
	visibilityArchivalState, visibilityArchivalConfigChanged, err := d.getVisibilityArchivalState(
		config,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}
	if visibilityArchivalConfigChanged {
		config.VisibilityArchivalStatus = visibilityArchivalState.Status
		config.VisibilityArchivalURI = visibilityArchivalState.URI
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
		replicationConfig,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	// Handle graceful failover request
	if updateRequest.FailoverTimeoutInSeconds != nil {
		// must update active cluster on a global domain
		if !activeClusterChanged || !isGlobalDomain {
			return nil, errInvalidGracefulFailover
		}
		// must start with the passive -> active cluster
		if replicationConfig.ActiveClusterName != d.clusterMetadata.GetCurrentClusterName() {
			return nil, errCannotDoGracefulFailoverFromCluster
		}
		if replicationConfig.ActiveClusterName == currentActiveCluster {
			return nil, errGracefulFailoverInActiveCluster
		}
		// cannot have concurrent failover
		if gracefulFailoverEndTime != nil {
			return nil, errOngoingGracefulFailover
		}
		endTime := d.timeSource.Now().Add(time.Duration(updateRequest.GetFailoverTimeoutInSeconds()) * time.Second).UnixNano()
		gracefulFailoverEndTime = &endTime
		previousFailoverVersion = failoverVersion
	}

	configurationChanged = historyArchivalConfigChanged || visibilityArchivalConfigChanged || domainInfoChanged || domainConfigChanged || deleteBinaryChanged || replicationConfigChanged || isolationGroupConfigurationChanged

	if err := d.domainAttrValidator.validateDomainConfig(config); err != nil {
		return nil, err
	}
	if isGlobalDomain {
		if err := d.domainAttrValidator.validateDomainReplicationConfigForGlobalDomain(
			replicationConfig,
		); err != nil {
			return nil, err
		}

		if configurationChanged && activeClusterChanged {
			return nil, errCannotDoDomainFailoverAndUpdate
		}

		if !activeClusterChanged && !d.clusterMetadata.IsPrimaryCluster() {
			return nil, errNotPrimaryCluster
		}
	} else {
		if err := d.domainAttrValidator.validateDomainReplicationConfigForLocalDomain(
			replicationConfig,
		); err != nil {
			return nil, err
		}
	}

	if configurationChanged || activeClusterChanged {
		now := d.timeSource.Now()
		// Check the failover cool down time
		if lastUpdatedTime.Add(d.config.FailoverCoolDown(info.Name)).After(now) {
			return nil, errDomainUpdateTooFrequent
		}

		// set the versions
		if configurationChanged {
			configVersion++
		}
		if activeClusterChanged && isGlobalDomain {
			// Force failover cleans graceful failover state
			if updateRequest.FailoverTimeoutInSeconds == nil {
				// force failover cleanup graceful failover state
				gracefulFailoverEndTime = nil
				previousFailoverVersion = common.InitialPreviousFailoverVersion
			}
			failoverVersion = d.clusterMetadata.GetNextFailoverVersion(
				replicationConfig.ActiveClusterName,
				failoverVersion,
				updateRequest.Name,
			)
			failoverNotificationVersion = notificationVersion
		}
		lastUpdatedTime = now
		updateReq := &persistence.UpdateDomainRequest{
			Info:                        info,
			Config:                      config,
			ReplicationConfig:           replicationConfig,
			ConfigVersion:               configVersion,
			FailoverVersion:             failoverVersion,
			FailoverNotificationVersion: failoverNotificationVersion,
			FailoverEndTime:             gracefulFailoverEndTime,
			PreviousFailoverVersion:     previousFailoverVersion,
			LastUpdatedTime:             lastUpdatedTime.UnixNano(),
			NotificationVersion:         notificationVersion,
		}
		err = d.domainManager.UpdateDomain(ctx, updateReq)
		if err != nil {
			return nil, err
		}
	}

	if isGlobalDomain {
		if err := d.domainReplicator.HandleTransmissionTask(
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

	updateReq := &persistence.UpdateDomainRequest{
		Info:                        getResponse.Info,
		Config:                      getResponse.Config,
		ReplicationConfig:           getResponse.ReplicationConfig,
		ConfigVersion:               getResponse.ConfigVersion,
		FailoverVersion:             getResponse.FailoverVersion,
		FailoverNotificationVersion: getResponse.FailoverNotificationVersion,
		FailoverEndTime:             getResponse.FailoverEndTime,
		PreviousFailoverVersion:     getResponse.PreviousFailoverVersion,
		LastUpdatedTime:             d.timeSource.Now().UnixNano(),
		NotificationVersion:         notificationVersion,
	}
	err = d.domainManager.UpdateDomain(ctx, updateReq)
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
		config.BadBinaries = d.mergeBadBinaries(config.BadBinaries.Binaries, updateRequest.BadBinaries.Binaries, time.Now().UnixNano())
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

func (d *handlerImpl) updateReplicationConfig(
	config *persistence.DomainReplicationConfig,
	updateRequest *types.UpdateDomainRequest,
) (*persistence.DomainReplicationConfig, bool, bool, error) {

	clusterUpdated := false
	activeClusterUpdated := false
	if len(updateRequest.Clusters) != 0 {
		clusterUpdated = true
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
		activeClusterUpdated = true
		config.ActiveClusterName = *updateRequest.ActiveClusterName
	}
	return config, clusterUpdated, activeClusterUpdated, nil
}

func (d *handlerImpl) getIsolationGroupStatus(
	incomingCfg *persistence.DomainConfig,
	updateRequest *types.UpdateDomainRequest,
) (config *persistence.DomainConfig, isolationGroupsChanged bool, err error) {

	if updateRequest == nil || updateRequest.IsolationGroupConfiguration == nil {
		return incomingCfg, false, nil
	}

	if incomingCfg == nil {
		return &persistence.DomainConfig{
			IsolationGroups: *updateRequest.IsolationGroupConfiguration,
		}, true, nil
	}

	// upsert with whatever is present in the request always
	incomingCfg.IsolationGroups = *updateRequest.IsolationGroupConfiguration
	return incomingCfg, true, nil
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
