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

package nosql

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

type nosqlDomainStore struct {
	nosqlStore
	currentClusterName string
}

// newNoSQLDomainStore is used to create an instance of DomainStore implementation
func newNoSQLDomainStore(
	cfg config.ShardedNoSQL,
	currentClusterName string,
	logger log.Logger,
	dc *persistence.DynamicConfiguration,
) (persistence.DomainStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlDomainStore{
		nosqlStore:         shardedStore.GetDefaultShard(),
		currentClusterName: currentClusterName,
	}, nil
}

// CreateDomain create a domain
// Cassandra does not support conditional updates across multiple tables.  For this reason we have to first insert into
// 'Domains' table and then do a conditional insert into domains_by_name table.  If the conditional write fails we
// delete the orphaned entry from domains table.  There is a chance delete entry could fail and we never delete the
// orphaned entry from domains table.  We might need a background job to delete those orphaned record.
func (m *nosqlDomainStore) CreateDomain(
	ctx context.Context,
	request *persistence.InternalCreateDomainRequest,
) (*persistence.CreateDomainResponse, error) {
	row := &nosqlplugin.DomainRow{
		Info:                        request.Info,
		Config:                      request.Config,
		ReplicationConfig:           request.ReplicationConfig,
		ConfigVersion:               request.ConfigVersion,
		FailoverVersion:             request.FailoverVersion,
		FailoverNotificationVersion: persistence.InitialFailoverNotificationVersion,
		PreviousFailoverVersion:     common.InitialPreviousFailoverVersion,
		FailoverEndTime:             nil,
		IsGlobalDomain:              request.IsGlobalDomain,
		LastUpdatedTime:             request.LastUpdatedTime,
	}

	err := m.db.InsertDomain(ctx, row)

	if err != nil {
		if _, ok := err.(*types.DomainAlreadyExistsError); ok {
			return nil, err
		}
		return nil, convertCommonErrors(m.db, "CreateDomain", err)
	}

	return &persistence.CreateDomainResponse{ID: request.Info.ID}, nil
}

func (m *nosqlDomainStore) UpdateDomain(
	ctx context.Context,
	request *persistence.InternalUpdateDomainRequest,
) error {
	row := &nosqlplugin.DomainRow{
		Info:                        request.Info,
		Config:                      request.Config,
		ReplicationConfig:           request.ReplicationConfig,
		ConfigVersion:               request.ConfigVersion,
		FailoverVersion:             request.FailoverVersion,
		FailoverNotificationVersion: request.FailoverNotificationVersion,
		PreviousFailoverVersion:     request.PreviousFailoverVersion,
		FailoverEndTime:             request.FailoverEndTime,
		NotificationVersion:         request.NotificationVersion,
		LastUpdatedTime:             request.LastUpdatedTime,
	}

	err := m.db.UpdateDomain(ctx, row)
	if err != nil {
		return convertCommonErrors(m.db, "UpdateDomain", err)
	}

	return nil
}

func (m *nosqlDomainStore) GetDomain(
	ctx context.Context,
	request *persistence.GetDomainRequest,
) (*persistence.InternalGetDomainResponse, error) {
	if len(request.ID) > 0 && len(request.Name) > 0 {
		return nil, &types.BadRequestError{
			Message: "GetDomain operation failed.  Both ID and Name specified in request.",
		}
	} else if len(request.ID) == 0 && len(request.Name) == 0 {
		return nil, &types.BadRequestError{
			Message: "GetDomain operation failed.  Both ID and Name are empty.",
		}
	}
	var domainName *string
	var domainID *string
	if len(request.ID) > 0 {
		domainID = common.StringPtr(request.ID)
	} else {
		domainName = common.StringPtr(request.Name)
	}

	handleError := func(name, ID string, err error) error {
		identity := name
		if len(ID) > 0 {
			identity = ID
		}
		if m.db.IsNotFoundError(err) {
			return &types.EntityNotExistsError{
				Message: fmt.Sprintf("Domain %s does not exist.", identity),
			}
		}
		return convertCommonErrors(m.db, "GetDomain", err)
	}

	row, err := m.db.SelectDomain(ctx, domainID, domainName)

	if err != nil {
		return nil, handleError(request.Name, request.ID, err)
	}

	if row.Info.Data == nil {
		row.Info.Data = map[string]string{}
	}
	row.ReplicationConfig.ActiveClusterName = cluster.GetOrUseDefaultActiveCluster(m.currentClusterName, row.ReplicationConfig.ActiveClusterName)
	row.ReplicationConfig.Clusters = cluster.GetOrUseDefaultClusters(m.currentClusterName, row.ReplicationConfig.Clusters)

	return &persistence.InternalGetDomainResponse{
		Info:                        row.Info,
		Config:                      row.Config,
		ReplicationConfig:           row.ReplicationConfig,
		IsGlobalDomain:              row.IsGlobalDomain,
		ConfigVersion:               row.ConfigVersion,
		FailoverVersion:             row.FailoverVersion,
		FailoverNotificationVersion: row.FailoverNotificationVersion,
		PreviousFailoverVersion:     row.PreviousFailoverVersion,
		FailoverEndTime:             row.FailoverEndTime,
		NotificationVersion:         row.NotificationVersion,
		LastUpdatedTime:             row.LastUpdatedTime,
	}, nil
}

func (m *nosqlDomainStore) ListDomains(
	ctx context.Context,
	request *persistence.ListDomainsRequest,
) (*persistence.InternalListDomainsResponse, error) {
	rows, nextPageToken, err := m.db.SelectAllDomains(ctx, request.PageSize, request.NextPageToken)
	if err != nil {
		return nil, convertCommonErrors(m.db, "ListDomains", err)
	}
	var domains []*persistence.InternalGetDomainResponse
	for _, row := range rows {
		if row.Info.Data == nil {
			row.Info.Data = map[string]string{}
		}
		row.ReplicationConfig.ActiveClusterName = cluster.GetOrUseDefaultActiveCluster(m.currentClusterName, row.ReplicationConfig.ActiveClusterName)
		row.ReplicationConfig.Clusters = cluster.GetOrUseDefaultClusters(m.currentClusterName, row.ReplicationConfig.Clusters)

		domains = append(domains, &persistence.InternalGetDomainResponse{
			Info:                        row.Info,
			Config:                      row.Config,
			ReplicationConfig:           row.ReplicationConfig,
			IsGlobalDomain:              row.IsGlobalDomain,
			ConfigVersion:               row.ConfigVersion,
			FailoverVersion:             row.FailoverVersion,
			FailoverNotificationVersion: row.FailoverNotificationVersion,
			PreviousFailoverVersion:     row.PreviousFailoverVersion,
			FailoverEndTime:             row.FailoverEndTime,
			NotificationVersion:         row.NotificationVersion,
			LastUpdatedTime:             row.LastUpdatedTime,
		})
	}

	return &persistence.InternalListDomainsResponse{
		Domains:       domains,
		NextPageToken: nextPageToken,
	}, nil
}

func (m *nosqlDomainStore) DeleteDomain(
	ctx context.Context,
	request *persistence.DeleteDomainRequest,
) error {
	if err := m.db.DeleteDomain(ctx, &request.ID, nil); err != nil {
		return convertCommonErrors(m.db, "DeleteDomain", err)
	}

	return nil
}

func (m *nosqlDomainStore) DeleteDomainByName(
	ctx context.Context,
	request *persistence.DeleteDomainByNameRequest,
) error {
	if err := m.db.DeleteDomain(ctx, nil, &request.Name); err != nil {
		return convertCommonErrors(m.db, "DeleteDomainByName", err)
	}

	return nil
}

func (m *nosqlDomainStore) GetMetadata(
	ctx context.Context,
) (*persistence.GetMetadataResponse, error) {
	notificationVersion, err := m.db.SelectDomainMetadata(ctx)
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetMetadata", err)
	}
	return &persistence.GetMetadataResponse{NotificationVersion: notificationVersion}, nil
}
