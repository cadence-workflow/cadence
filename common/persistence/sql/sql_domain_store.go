// Copyright (c) 2018 Uber Technologies, Inc.
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

package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

type sqlDomainStore struct {
	sqlStore
	activeClusterName string
}

// newMetadataPersistenceV2 creates an instance of sqlDomainStore
func newMetadataPersistenceV2(
	db sqlplugin.DB,
	currentClusterName string,
	logger log.Logger,
	parser serialization.Parser,
) (persistence.DomainStore, error) {
	return &sqlDomainStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
		},
		activeClusterName: currentClusterName,
	}, nil
}

func updateMetadata(ctx context.Context, tx sqlplugin.Tx, oldNotificationVersion int64) error {
	result, err := tx.UpdateDomainMetadata(ctx, &sqlplugin.DomainMetadataRow{NotificationVersion: oldNotificationVersion})
	if err != nil {
		return convertCommonErrors(tx, "updateDomainMetadata", "", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Could not verify whether domain metadata update occurred. Error: %v", err),
		}
	} else if rowsAffected != 1 {
		return &types.InternalServiceError{
			Message: "Failed to update domain metadata. <>1 rows affected.",
		}
	}

	return nil
}

func lockMetadata(ctx context.Context, tx sqlplugin.Tx) error {
	err := tx.LockDomainMetadata(ctx)
	if err != nil {
		return convertCommonErrors(tx, "lockDomainMetadata", "", err)
	}
	return nil
}

func (m *sqlDomainStore) CreateDomain(
	ctx context.Context,
	request *persistence.InternalCreateDomainRequest,
) (*persistence.CreateDomainResponse, error) {
	metadata, err := m.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}

	clusters := make([]string, len(request.ReplicationConfig.Clusters))
	for i := range clusters {
		clusters[i] = request.ReplicationConfig.Clusters[i].ClusterName
	}

	var badBinaries []byte
	badBinariesEncoding := string(constants.EncodingTypeEmpty)
	if request.Config.BadBinaries != nil {
		badBinaries = request.Config.BadBinaries.Data
		badBinariesEncoding = string(request.Config.BadBinaries.GetEncoding())
	}

	var isolationGroups []byte
	var isolationGroupsEncoding string
	if request.Config != nil && request.Config.IsolationGroups != nil {
		isolationGroups = request.Config.IsolationGroups.GetData()
		isolationGroupsEncoding = request.Config.IsolationGroups.GetEncodingString()
	}

	var asyncWorkflowsCfg []byte
	var asyncWorkflowsEncoding string
	if request.Config != nil && request.Config.AsyncWorkflowsConfig != nil {
		asyncWorkflowsCfg = request.Config.AsyncWorkflowsConfig.GetData()
		asyncWorkflowsEncoding = request.Config.AsyncWorkflowsConfig.GetEncodingString()
	}

	domainInfo := &serialization.DomainInfo{
		Name:                        request.Info.Name,
		Status:                      int32(request.Info.Status),
		Description:                 request.Info.Description,
		Owner:                       request.Info.OwnerEmail,
		Data:                        request.Info.Data,
		Retention:                   request.Config.Retention,
		EmitMetric:                  request.Config.EmitMetric,
		ArchivalBucket:              request.Config.ArchivalBucket,
		ArchivalStatus:              int16(request.Config.ArchivalStatus),
		HistoryArchivalStatus:       int16(request.Config.HistoryArchivalStatus),
		HistoryArchivalURI:          request.Config.HistoryArchivalURI,
		VisibilityArchivalStatus:    int16(request.Config.VisibilityArchivalStatus),
		VisibilityArchivalURI:       request.Config.VisibilityArchivalURI,
		ActiveClusterName:           request.ReplicationConfig.ActiveClusterName,
		Clusters:                    clusters,
		ConfigVersion:               request.ConfigVersion,
		FailoverVersion:             request.FailoverVersion,
		NotificationVersion:         metadata.NotificationVersion,
		FailoverNotificationVersion: persistence.InitialFailoverNotificationVersion,
		PreviousFailoverVersion:     constants.InitialPreviousFailoverVersion,
		LastUpdatedTimestamp:        request.LastUpdatedTime,
		BadBinaries:                 badBinaries,
		BadBinariesEncoding:         badBinariesEncoding,
		IsolationGroups:             isolationGroups,
		IsolationGroupsEncoding:     isolationGroupsEncoding,
		AsyncWorkflowConfig:         asyncWorkflowsCfg,
		AsyncWorkflowConfigEncoding: asyncWorkflowsEncoding,
	}

	blob, err := m.parser.DomainInfoToBlob(domainInfo)
	if err != nil {
		return nil, err
	}

	var resp *persistence.CreateDomainResponse
	err = m.txExecute(ctx, sqlplugin.DbDefaultShard, "CreateDomain", func(tx sqlplugin.Tx) error {
		if _, err1 := tx.InsertIntoDomain(ctx, &sqlplugin.DomainRow{
			Name:         request.Info.Name,
			ID:           serialization.MustParseUUID(request.Info.ID),
			Data:         blob.Data,
			DataEncoding: string(blob.Encoding),
			IsGlobal:     request.IsGlobalDomain,
		}); err1 != nil {
			if m.db.IsDupEntryError(err1) {
				return &types.DomainAlreadyExistsError{
					Message: fmt.Sprintf("name: %v", request.Info.Name),
				}
			}
			return err1
		}
		if err1 := lockMetadata(ctx, tx); err1 != nil {
			return err1
		}
		if err1 := updateMetadata(ctx, tx, metadata.NotificationVersion); err1 != nil {
			return err1
		}
		resp = &persistence.CreateDomainResponse{ID: request.Info.ID}
		return nil
	})
	return resp, err
}

func (m *sqlDomainStore) GetDomain(
	ctx context.Context,
	request *persistence.GetDomainRequest,
) (*persistence.InternalGetDomainResponse, error) {
	filter := &sqlplugin.DomainFilter{}
	switch {
	case request.Name != "" && request.ID != "":
		return nil, &types.BadRequestError{
			Message: "GetDomain operation failed.  Both ID and Name specified in request.",
		}
	case request.Name != "":
		filter.Name = &request.Name
	case request.ID != "":
		filter.ID = serialization.UUIDPtr(serialization.MustParseUUID(request.ID))
	default:
		return nil, &types.BadRequestError{
			Message: "GetDomain operation failed.  Both ID and Name are empty.",
		}
	}

	rows, err := m.db.SelectFromDomain(ctx, filter)
	if err != nil {
		switch err {
		case sql.ErrNoRows:
			// We did not return in the above for-loop because there were no rows.
			identity := request.Name
			if len(request.ID) > 0 {
				identity = request.ID
			}

			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Domain %s does not exist.", identity),
			}
		default:
			return nil, convertCommonErrors(m.db, "GetDomain", "", err)
		}
	}

	response, err := m.domainRowToGetDomainResponse(&rows[0])
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (m *sqlDomainStore) domainRowToGetDomainResponse(row *sqlplugin.DomainRow) (*persistence.InternalGetDomainResponse, error) {
	domainInfo, err := m.parser.DomainInfoFromBlob(row.Data, row.DataEncoding)
	if err != nil {
		return nil, err
	}

	clusters := make([]*persistence.ClusterReplicationConfig, len(domainInfo.Clusters))
	for i := range domainInfo.Clusters {
		clusters[i] = &persistence.ClusterReplicationConfig{ClusterName: domainInfo.Clusters[i]}
	}

	var badBinaries *persistence.DataBlob
	if domainInfo.BadBinaries != nil {
		badBinaries = persistence.NewDataBlob(domainInfo.BadBinaries, constants.EncodingType(domainInfo.GetBadBinariesEncoding()))
	}

	var isolationGroups *persistence.DataBlob
	if domainInfo.IsolationGroups != nil {
		isolationGroups = persistence.NewDataBlob(domainInfo.IsolationGroups, constants.EncodingType(domainInfo.IsolationGroupsEncoding))
	}

	var asyncWorkflowsCfg *persistence.DataBlob
	if domainInfo.AsyncWorkflowConfig != nil {
		asyncWorkflowsCfg = persistence.NewDataBlob(domainInfo.AsyncWorkflowConfig, constants.EncodingType(domainInfo.AsyncWorkflowConfigEncoding))
	}

	return &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:          row.ID.String(),
			Name:        row.Name,
			Status:      int(domainInfo.GetStatus()),
			Description: domainInfo.GetDescription(),
			OwnerEmail:  domainInfo.GetOwner(),
			Data:        domainInfo.GetData(),
		},
		Config: &persistence.InternalDomainConfig{
			Retention:                domainInfo.GetRetention(),
			EmitMetric:               domainInfo.GetEmitMetric(),
			ArchivalBucket:           domainInfo.GetArchivalBucket(),
			ArchivalStatus:           types.ArchivalStatus(domainInfo.GetArchivalStatus()),
			HistoryArchivalStatus:    types.ArchivalStatus(domainInfo.GetHistoryArchivalStatus()),
			HistoryArchivalURI:       domainInfo.GetHistoryArchivalURI(),
			VisibilityArchivalStatus: types.ArchivalStatus(domainInfo.GetVisibilityArchivalStatus()),
			VisibilityArchivalURI:    domainInfo.GetVisibilityArchivalURI(),
			BadBinaries:              badBinaries,
			IsolationGroups:          isolationGroups,
			AsyncWorkflowsConfig:     asyncWorkflowsCfg,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.GetOrUseDefaultActiveCluster(m.activeClusterName, domainInfo.GetActiveClusterName()),
			Clusters:          cluster.GetOrUseDefaultClusters(m.activeClusterName, clusters),
		},
		IsGlobalDomain:              row.IsGlobal,
		FailoverVersion:             domainInfo.GetFailoverVersion(),
		ConfigVersion:               domainInfo.GetConfigVersion(),
		NotificationVersion:         domainInfo.GetNotificationVersion(),
		FailoverNotificationVersion: domainInfo.GetFailoverNotificationVersion(),
		PreviousFailoverVersion:     domainInfo.GetPreviousFailoverVersion(),
		FailoverEndTime:             domainInfo.FailoverEndTimestamp,
		LastUpdatedTime:             domainInfo.GetLastUpdatedTimestamp(),
	}, nil
}

func (m *sqlDomainStore) UpdateDomain(
	ctx context.Context,
	request *persistence.InternalUpdateDomainRequest,
) error {

	clusters := make([]string, len(request.ReplicationConfig.Clusters))
	for i := range clusters {
		clusters[i] = request.ReplicationConfig.Clusters[i].ClusterName
	}

	var badBinaries []byte
	badBinariesEncoding := string(constants.EncodingTypeEmpty)
	if request.Config.BadBinaries != nil {
		badBinaries = request.Config.BadBinaries.Data
		badBinariesEncoding = string(request.Config.BadBinaries.GetEncoding())
	}

	var isolationGroups []byte
	isolationGroupsEncoding := string(constants.EncodingTypeEmpty)
	if request.Config.IsolationGroups != nil {
		isolationGroups = request.Config.IsolationGroups.Data
		isolationGroupsEncoding = request.Config.IsolationGroups.GetEncodingString()
	}

	var asyncWorkflowsCfg []byte
	asyncWorkflowsEncoding := string(constants.EncodingTypeEmpty)
	if request.Config.AsyncWorkflowsConfig != nil {
		asyncWorkflowsCfg = request.Config.AsyncWorkflowsConfig.Data
		asyncWorkflowsEncoding = request.Config.AsyncWorkflowsConfig.GetEncodingString()
	}

	domainInfo := &serialization.DomainInfo{
		Status:                      int32(request.Info.Status),
		Description:                 request.Info.Description,
		Owner:                       request.Info.OwnerEmail,
		Data:                        request.Info.Data,
		Retention:                   request.Config.Retention,
		EmitMetric:                  request.Config.EmitMetric,
		ArchivalBucket:              request.Config.ArchivalBucket,
		ArchivalStatus:              int16(request.Config.ArchivalStatus),
		HistoryArchivalStatus:       int16(request.Config.HistoryArchivalStatus),
		HistoryArchivalURI:          request.Config.HistoryArchivalURI,
		VisibilityArchivalStatus:    int16(request.Config.VisibilityArchivalStatus),
		VisibilityArchivalURI:       request.Config.VisibilityArchivalURI,
		ActiveClusterName:           request.ReplicationConfig.ActiveClusterName,
		Clusters:                    clusters,
		ConfigVersion:               request.ConfigVersion,
		FailoverVersion:             request.FailoverVersion,
		NotificationVersion:         request.NotificationVersion,
		FailoverNotificationVersion: request.FailoverNotificationVersion,
		PreviousFailoverVersion:     request.PreviousFailoverVersion,
		FailoverEndTimestamp:        request.FailoverEndTime,
		LastUpdatedTimestamp:        request.LastUpdatedTime,
		BadBinaries:                 badBinaries,
		BadBinariesEncoding:         badBinariesEncoding,
		IsolationGroups:             isolationGroups,
		IsolationGroupsEncoding:     isolationGroupsEncoding,
		AsyncWorkflowConfig:         asyncWorkflowsCfg,
		AsyncWorkflowConfigEncoding: asyncWorkflowsEncoding,
	}

	blob, err := m.parser.DomainInfoToBlob(domainInfo)
	if err != nil {
		return err
	}

	return m.txExecute(ctx, sqlplugin.DbDefaultShard, "UpdateDomain", func(tx sqlplugin.Tx) error {
		result, err := tx.UpdateDomain(ctx, &sqlplugin.DomainRow{
			Name:         request.Info.Name,
			ID:           serialization.MustParseUUID(request.Info.ID),
			Data:         blob.Data,
			DataEncoding: string(blob.Encoding),
		})
		if err != nil {
			return err
		}
		noRowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("rowsAffected error: %v", err)
		}
		if noRowsAffected != 1 {
			return fmt.Errorf("%v rows updated instead of one", noRowsAffected)
		}
		if err := lockMetadata(ctx, tx); err != nil {
			return err
		}
		return updateMetadata(ctx, tx, request.NotificationVersion)
	})
}

func (m *sqlDomainStore) DeleteDomain(
	ctx context.Context,
	request *persistence.DeleteDomainRequest,
) error {
	return m.txExecute(ctx, sqlplugin.DbDefaultShard, "DeleteDomain", func(tx sqlplugin.Tx) error {
		_, err := tx.DeleteFromDomain(ctx, &sqlplugin.DomainFilter{ID: serialization.UUIDPtr(serialization.MustParseUUID(request.ID))})
		return err
	})
}

func (m *sqlDomainStore) DeleteDomainByName(
	ctx context.Context,
	request *persistence.DeleteDomainByNameRequest,
) error {
	return m.txExecute(ctx, sqlplugin.DbDefaultShard, "DeleteDomainByName", func(tx sqlplugin.Tx) error {
		_, err := tx.DeleteFromDomain(ctx, &sqlplugin.DomainFilter{Name: &request.Name})
		return err
	})
}

func (m *sqlDomainStore) GetMetadata(
	ctx context.Context,
) (*persistence.GetMetadataResponse, error) {
	row, err := m.db.SelectFromDomainMetadata(ctx)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetMetadata", "", err)
	}
	return &persistence.GetMetadataResponse{NotificationVersion: row.NotificationVersion}, nil
}

func (m *sqlDomainStore) ListDomains(
	ctx context.Context,
	request *persistence.ListDomainsRequest,
) (*persistence.InternalListDomainsResponse, error) {
	var pageToken *serialization.UUID
	if request.NextPageToken != nil {
		token := serialization.UUID(request.NextPageToken)
		pageToken = &token
	}
	rows, err := m.db.SelectFromDomain(ctx, &sqlplugin.DomainFilter{
		GreaterThanID: pageToken,
		PageSize:      &request.PageSize,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return &persistence.InternalListDomainsResponse{}, nil
		}
		return nil, convertCommonErrors(m.db, "ListDomains", "Failed to get domain rows.", err)
	}

	var domains []*persistence.InternalGetDomainResponse
	for _, row := range rows {
		resp, err := m.domainRowToGetDomainResponse(&row)
		if err != nil {
			return nil, err
		}
		domains = append(domains, resp)
	}

	resp := &persistence.InternalListDomainsResponse{Domains: domains}
	if len(rows) >= request.PageSize {
		resp.NextPageToken = rows[len(rows)-1].ID
	}

	return resp, nil
}
