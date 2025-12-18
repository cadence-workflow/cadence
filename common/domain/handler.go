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

	guuid "github.com/google/uuid"

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
		domainAuditManager  persistence.DomainAuditManager
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
		MinRetentionDays         dynamicproperties.IntPropertyFn
		MaxRetentionDays         dynamicproperties.IntPropertyFn
		RequiredDomainDataKeys   dynamicproperties.MapPropertyFn
		MaxBadBinaryCount        dynamicproperties.IntPropertyFnWithDomainFilter
		FailoverCoolDown         dynamicproperties.DurationPropertyFnWithDomainFilter
		FailoverHistoryMaxSize   dynamicproperties.IntPropertyFnWithDomainFilter
		EnableDomainAuditLogging dynamicproperties.BoolPropertyFn
	}

	// FailoverEvent is the failover information to be stored for each failover event in domain data
	FailoverEvent struct {
		EventTime time.Time `json:"eventTime"`

		// active-passive domain failover
		FromCluster  string `json:"fromCluster,omitempty"`
		ToCluster    string `json:"toCluster,omitempty"`
		FailoverType string `json:"failoverType,omitempty"`
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
	domainAuditManager persistence.DomainAuditManager,
	clusterMetadata cluster.Metadata,
	domainReplicator Replicator,
	archivalMetadata archiver.ArchivalMetadata,
	archiverProvider provider.ArchiverProvider,
	timeSource clock.TimeSource,
) Handler {
	return &handlerImpl{
		logger:              logger,
		domainManager:       domainManager,
		domainAuditManager:  domainAuditManager,
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

	eventID, err := guuid.NewV7()
	if err != nil {
		return err
	}

	info := &persistence.DomainInfo{
		ID:          eventID.String(),
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

	// Construct GetDomainResponse for audit log
	domainStateAfterCreate := &persistence.GetDomainResponse{
		Info:              domainRequest.Info,
		Config:            domainRequest.Config,
		ReplicationConfig: domainRequest.ReplicationConfig,
		IsGlobalDomain:    domainRequest.IsGlobalDomain,
		ConfigVersion:     domainRequest.ConfigVersion,
		FailoverVersion:   domainRequest.FailoverVersion,
		LastUpdatedTime:   domainRequest.LastUpdatedTime,
	}

	err = d.updateDomainAuditLog(ctx, nil, domainStateAfterCreate, persistence.DomainAuditOperationTypeCreate, "domain created")
	if err != nil {
		return err
	}

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

	isGlobalDomain := currentDomainState.IsGlobalDomain

	if !isGlobalDomain {
		return d.updateLocalDomain(ctx, updateRequest, currentDomainState, notificationVersion)
	}

	if updateRequest.IsAFailoverRequest() {
		return d.handleFailoverRequest(ctx, updateRequest, currentDomainState, notificationVersion)
	}

	return d.updateGlobalDomainConfiguration(ctx, updateRequest, currentDomainState, notificationVersion)
}

// All domain updates are throttled by the cool down time (incorrecty called 'failover' cool down).
// The guard is an anti-flapping measure.
func (d *handlerImpl) ensureUpdateOrFailoverCooldown(currentDomainState *persistence.GetDomainResponse) error {
	lastUpdatedTime := time.Unix(0, currentDomainState.LastUpdatedTime)
	now := d.timeSource.Now()
	if lastUpdatedTime.Add(d.config.FailoverCoolDown(currentDomainState.Info.Name)).After(now) {
		d.logger.Debugf("Domain was last updated at %v, failoverCoolDown: %v, current time: %v.", lastUpdatedTime, d.config.FailoverCoolDown(currentDomainState.Info.Name), now)
		return errDomainUpdateTooFrequent
	}
	return nil
}

// For global domains only, this is assumed to be invoked when
// the incoming request is either specifying an active-cluster parameter to update
// or active_clusters in the case of a AA domain
func (d *handlerImpl) handleFailoverRequest(ctx context.Context,
	updateRequest *types.UpdateDomainRequest,
	currentState *persistence.GetDomainResponse,
	notificationVersion int64,
) (*types.UpdateDomainResponse, error) {

	// intendedDomainState will be modified
	// into the intended shape by the functions here
	intendedDomainState := currentState.DeepCopy()

	isGlobalDomain := currentState.IsGlobalDomain

	currentActiveCluster := currentState.ReplicationConfig.ActiveClusterName
	wasActiveActive := currentState.ReplicationConfig.IsActiveActive()
	now := d.timeSource.Now()

	// by default, we assume failovers are of type force
	failoverType := constants.FailoverTypeForce

	var activeClusterChanged bool
	var configurationChanged bool

	// will be set to the domain notification version after the update
	intendedDomainState.FailoverNotificationVersion = types.UndefinedFailoverVersion
	// not used except for graceful failover requests, but specifically set to -1
	// so as to be explicitly undefined
	intendedDomainState.PreviousFailoverVersion = constants.InitialPreviousFailoverVersion

	// by default, we assume a force failover and that any preexisting graceful failover state is invalidated
	// if there's a duration of failover time to occur (such as in graceful failover) this will be re-set.
	// But if there's an existing graceful failover and a subsequent force
	// we want to ensure that it'll be ended immmediately.
	intendedDomainState.FailoverEndTime = nil

	// Update replication config
	replicationCfg, replicationConfigChanged, activeClusterChanged, err := d.updateReplicationConfig(
		currentState.Info.Name,
		intendedDomainState.ReplicationConfig,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}
	if !activeClusterChanged && !replicationConfigChanged {
		return nil, errInvalidFailoverNoChangeDetected
	}
	intendedDomainState.ReplicationConfig = replicationCfg

	err = d.ensureUpdateOrFailoverCooldown(currentState)
	if err != nil {
		return nil, err
	}

	// if the failover 'graceful' - as indicated as having a FailoverTimeoutInSeconds,
	// then we set some additional parameters for the graceful failover
	if updateRequest.FailoverTimeoutInSeconds != nil {
		gracefulFailoverEndTime, previousFailoverVersion, err := d.handleGracefulFailover(
			updateRequest,
			intendedDomainState.ReplicationConfig,
			currentActiveCluster,
			currentState.FailoverEndTime,
			currentState.FailoverVersion,
			activeClusterChanged,
			isGlobalDomain,
		)
		if err != nil {
			return nil, err
		}
		failoverType = constants.FailoverTypeGrace
		intendedDomainState.FailoverEndTime = gracefulFailoverEndTime
		intendedDomainState.PreviousFailoverVersion = previousFailoverVersion
	}

	// replication config is a subset of config,
	configurationChanged = replicationConfigChanged

	if err = d.domainAttrValidator.validateDomainConfig(intendedDomainState.Config); err != nil {
		return nil, err
	}

	err = d.validateDomainReplicationConfigForFailover(intendedDomainState.ReplicationConfig, configurationChanged, activeClusterChanged)
	if err != nil {
		return nil, err
	}

	// increment the in the configuration fencing token to ensure that configurations
	// are applied in order
	if configurationChanged {
		intendedDomainState.ConfigVersion++
	}

	intendedDomainState.FailoverVersion = d.clusterMetadata.GetNextFailoverVersion(
		intendedDomainState.ReplicationConfig.ActiveClusterName,
		currentState.FailoverVersion,
		updateRequest.Name,
	)

	// this is an intended failover step, to capture the current cluster's domain configuration
	// (represented by the notification-version counter) and set it when running failover to allow
	// systems like graceful failover to dedup failover processes and callbacks
	intendedDomainState.FailoverNotificationVersion = notificationVersion

	isActiveActive := intendedDomainState.ReplicationConfig.IsActiveActive()

	if wasActiveActive || isActiveActive {
		// if the domain was ever active-active, we bump the failover-version at the domain level
		// to ensure that it gets incremented every single time, even though the failover may be only
		// at the cluster-attribute level (ie the FailoverVersion may just go up by the failover-increment
		// but otherwise be a noop - the actual active cluster may not have changed)).
		//
		// The reason for this is caution and simplicity at the time of writing:
		// It's harmless to bump and this simplifies thinking about failover events
		// through out the rest of the Cadence codebase. We can reliably assume that every failover
		// event that has meaningful changes only needs to subscribe to this fencing token.
		// to detect changes.
		//
		// Any parts of the system (domain-callbacks, domain cache etc) that watch for
		// failover events and may trigger some action or cache invalidation will be watching
		// the domain-level failver counter for changes. Therefore, by bumping it even for
		// cases where it isn't changing, we ensure all these other subprocesses will
		// take
		intendedDomainState.FailoverVersion = d.clusterMetadata.GetNextFailoverVersion(
			intendedDomainState.ReplicationConfig.ActiveClusterName,
			currentState.FailoverVersion+1,
			updateRequest.Name,
		)
	}

	// the domain-data table is only updated for active-passive domain failovers
	// as a historical backwards compatibility measure.
	// Going forward, any history use-cases should rely on the FailoverHistory endpoint which
	// supports all failover types and more than a handful of entries.
	if !wasActiveActive && !isActiveActive {
		err = updateFailoverHistoryInDomainData(intendedDomainState.Info, d.config, NewFailoverEvent(
			now,
			failoverType,
			&currentActiveCluster,
			updateRequest.ActiveClusterName,
		))
		if err != nil {
			d.logger.Warn("failed to update failover history", tag.Error(err))
		}
	}

	updateReq := createUpdateRequest(
		intendedDomainState.Info,
		intendedDomainState.Config,
		intendedDomainState.ReplicationConfig,
		intendedDomainState.ConfigVersion,
		intendedDomainState.FailoverVersion,
		intendedDomainState.FailoverNotificationVersion,
		intendedDomainState.FailoverEndTime,
		intendedDomainState.PreviousFailoverVersion,
		now,
		notificationVersion,
	)

	err = d.domainManager.UpdateDomain(ctx, &updateReq)
	if err != nil {
		return nil, err
	}
	if err = d.domainReplicator.HandleTransmissionTask(
		ctx,
		types.DomainOperationUpdate,
		intendedDomainState.Info,
		intendedDomainState.Config,
		intendedDomainState.ReplicationConfig,
		intendedDomainState.ConfigVersion,
		intendedDomainState.FailoverVersion,
		intendedDomainState.PreviousFailoverVersion,
		isGlobalDomain,
	); err != nil {
		return nil, err
	}
	response := &types.UpdateDomainResponse{
		IsGlobalDomain:  isGlobalDomain,
		FailoverVersion: intendedDomainState.FailoverVersion,
	}
	response.DomainInfo, response.Configuration, response.ReplicationConfiguration = d.createResponse(intendedDomainState.Info, intendedDomainState.Config, intendedDomainState.ReplicationConfig)

	err = d.updateDomainAuditLog(ctx, currentState, intendedDomainState, persistence.DomainAuditOperationTypeFailover, "domain failover")
	if err != nil {
		return nil, err
	}

	d.logger.Info("Failover request succeeded",
		tag.WorkflowDomainName(intendedDomainState.Info.Name),
		tag.WorkflowDomainID(intendedDomainState.Info.ID),
	)
	return response, nil
}

func (d *handlerImpl) updateDomainAuditLog(ctx context.Context,
	currentState *persistence.GetDomainResponse,
	intendedDomainState *persistence.GetDomainResponse,
	operationType persistence.DomainAuditOperationType,
	comment string,
) error {

	if d.domainAuditManager == nil {
		return nil
	}

	if !d.config.EnableDomainAuditLogging() {
		return nil
	}

	// Must be a UUID v7, since we need a time value as well
	eventID, err := guuid.NewV7()
	if err != nil {
		return err
	}
	// the creation time is used in the database as a partition but passed around
	// embedded in the eventUUID for ergonomics. This means that users wishing
	// to get an audit entry by ID do not need to know the creation time in advance
	// since it's embedded in the sorting-values of the UUID.
	creationTime := time.Unix(eventID.Time().UnixTime())

	_, err = d.domainAuditManager.CreateDomainAuditLog(ctx, &persistence.CreateDomainAuditLogRequest{
		DomainID:      intendedDomainState.GetInfo().GetID(),
		EventID:       eventID.String(),
		CreatedTime:   creationTime,
		StateBefore:   currentState,
		StateAfter:    intendedDomainState,
		OperationType: operationType,
		Comment:       comment,
	})
	if err != nil {
		d.logger.Error("Failed to create domain audit log",
			tag.WorkflowDomainID(intendedDomainState.GetInfo().GetID()),
			tag.Error(err),
		)
		// Log the error but don't fail the operation - audit logging is best effort
		// to avoid breaking critical domain operations
	}
	return nil
}

// updateGlobalDomainConfiguration handles the update of a global domain configuration
// this excludes failover/active_cluster/active_clusters updates. They are grouped under
// forms of failover
func (d *handlerImpl) updateGlobalDomainConfiguration(ctx context.Context,
	updateRequest *types.UpdateDomainRequest,
	currentDomainState *persistence.GetDomainResponse,
	notificationVersion int64,
) (*types.UpdateDomainResponse, error) {

	// intendedDomainState will be modified
	// into the intended shape by the functions here
	intendedDomainState := currentDomainState.DeepCopy()

	configVersion := currentDomainState.ConfigVersion
	failoverVersion := currentDomainState.FailoverVersion
	isGlobalDomain := currentDomainState.IsGlobalDomain

	now := d.timeSource.Now()

	// whether history archival config changed
	historyArchivalConfigChanged := false
	// whether visibility archival config changed
	visibilityArchivalConfigChanged := false
	// whether active cluster is changed
	activeClusterChanged := false
	// whether anything other than active cluster is changed
	configurationChanged := false

	// Update history archival state
	historyArchivalConfigChanged, err := d.updateHistoryArchivalState(intendedDomainState.Config, updateRequest)
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

	// Update replication config
	replicationConfig, replicationConfigChanged, activeClusterChanged, err := d.updateReplicationConfig(
		intendedDomainState.Info.Name,
		intendedDomainState.ReplicationConfig,
		updateRequest,
	)
	if err != nil {
		return nil, err
	}

	configurationChanged = historyArchivalConfigChanged || visibilityArchivalConfigChanged || domainInfoChanged || domainConfigChanged || deleteBinaryChanged || replicationConfigChanged

	if err = d.domainAttrValidator.validateDomainConfig(config); err != nil {
		return nil, err
	}

	err = d.validateGlobalDomainReplicationConfigForUpdateDomain(replicationConfig, configurationChanged, activeClusterChanged)
	if err != nil {
		return nil, err
	}

	err = d.ensureUpdateOrFailoverCooldown(currentDomainState)
	if err != nil {
		return nil, err
	}

	if configurationChanged || activeClusterChanged {

		// set the versions
		if configurationChanged {
			configVersion++
		}

		updateReq := createUpdateRequest(
			info,
			config,
			replicationConfig,
			configVersion,
			failoverVersion,
			currentDomainState.FailoverNotificationVersion,
			intendedDomainState.FailoverEndTime,
			intendedDomainState.PreviousFailoverVersion,
			now,
			notificationVersion,
		)

		err = d.domainManager.UpdateDomain(ctx, &updateReq)
		if err != nil {
			return nil, err
		}
	}
	if err = d.domainReplicator.HandleTransmissionTask(
		ctx,
		types.DomainOperationUpdate,
		info,
		config,
		replicationConfig,
		configVersion,
		failoverVersion,
		intendedDomainState.PreviousFailoverVersion,
		isGlobalDomain,
	); err != nil {
		return nil, err
	}
	response := &types.UpdateDomainResponse{
		IsGlobalDomain:  isGlobalDomain,
		FailoverVersion: failoverVersion,
	}
	response.DomainInfo, response.Configuration, response.ReplicationConfiguration = d.createResponse(info, config, replicationConfig)

	// Construct GetDomainResponse for audit log with the final updated values
	domainStateAfterUpdate := &persistence.GetDomainResponse{
		Info:                        info,
		Config:                      config,
		ReplicationConfig:           replicationConfig,
		IsGlobalDomain:              isGlobalDomain,
		ConfigVersion:               configVersion,
		FailoverVersion:             failoverVersion,
		FailoverNotificationVersion: intendedDomainState.FailoverNotificationVersion,
		PreviousFailoverVersion:     intendedDomainState.PreviousFailoverVersion,
		FailoverEndTime:             intendedDomainState.FailoverEndTime,
		LastUpdatedTime:             now.UnixNano(),
		NotificationVersion:         notificationVersion,
	}

	err = d.updateDomainAuditLog(ctx, currentDomainState, domainStateAfterUpdate, persistence.DomainAuditOperationTypeUpdate, "domain updated")
	if err != nil {
		return nil, err
	}

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

		updateReq := createUpdateRequest(
			info,
			config,
			intendedDomainState.ReplicationConfig,
			configVersion,
			intendedDomainState.FailoverVersion,
			intendedDomainState.FailoverNotificationVersion,
			intendedDomainState.FailoverEndTime,
			intendedDomainState.PreviousFailoverVersion,
			now,
			notificationVersion,
		)

		err = d.domainManager.UpdateDomain(ctx, &updateReq)
		if err != nil {
			return nil, err
		}

		err = d.updateDomainAuditLog(ctx, currentState, intendedDomainState, persistence.DomainAuditOperationTypeUpdate, "domain updated")
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

	metadata, err := d.domainManager.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	notificationVersion := metadata.NotificationVersion

	currentDomainState, err := d.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{Name: failoverRequest.GetDomainName()})
	if err != nil {
		return nil, err
	}

	err = d.validateDomainFailoverRequest(failoverRequest, currentDomainState)
	if err != nil {
		return nil, err
	}

	response, err := d.handleFailoverRequest(
		ctx,
		failoverRequest.ToUpdateDomainRequest(),
		currentDomainState,
		notificationVersion,
	)
	if err != nil {
		return nil, err
	}
	return response.ToFailoverDomainResponse(), nil
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

	err = d.updateDomainAuditLog(ctx, getResponse, nil, persistence.DomainAuditOperationTypeDelete, "domain deleted")
	if err != nil {
		return err
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

	domainStateAfterDeprecate := &persistence.GetDomainResponse{
		Info: getResponse.Info,
	}
	err = d.updateDomainAuditLog(ctx, getResponse, domainStateAfterDeprecate, persistence.DomainAuditOperationTypeDeprecate, "domain deprecated")
	if err != nil {
		return err
	}
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

	err = d.updateDomainAuditLog(ctx, currentDomainConfig, currentDomainConfig, persistence.DomainAuditOperationTypeUpdate, "async workflow queue config update")
	if err != nil {
		return err
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
	previousFailoverVersion := failoverVersion

	return &endTime, previousFailoverVersion, nil
}

func (d *handlerImpl) validateGlobalDomainReplicationConfigForUpdateDomain(
	replicationConfig *persistence.DomainReplicationConfig,
	configurationChanged bool,
	activeClusterChanged bool,
) error {
	var err error
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
	return nil
}

// validateDomainFailoverRequest is the handler method for the FailoverDomain
// handler. It can receive request to failover domains in a variety of combinations
// including invalid ones.
func (d *handlerImpl) validateDomainFailoverRequest(
	request *types.FailoverDomainRequest,
	currentDomainState *persistence.GetDomainResponse,
) error {
	if request == nil {
		return &types.BadRequestError{Message: "Request cannot be nil"}
	}

	if request.DomainName == "" {
		return &types.BadRequestError{Message: "DomainName cannot be empty"}
	}

	if !currentDomainState.IsGlobalDomain {
		return errLocalDomainsCannotFailover
	}

	if request.ActiveClusters == nil && request.DomainActiveClusterName == nil {
		return &types.BadRequestError{Message: "Domain's ActiveClusterName or ActiveClusters must be set to failover the domain"}
	}
	return nil
}

// validateDomainReplicationConfigForFailover is to check if the replication config
// is valid and sane for failovers. It is only for global domains
func (d *handlerImpl) validateDomainReplicationConfigForFailover(
	replicationConfig *persistence.DomainReplicationConfig,
	configurationChanged bool,
	activeClusterChanged bool,
) error {
	// todo (add any additional failover validation here)
	return d.validateGlobalDomainReplicationConfigForUpdateDomain(replicationConfig, configurationChanged, activeClusterChanged)
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
