// Copyright (c) 2025 Uber Technologies, Inc.
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

package persistence

import (
	"context"
	"fmt"

	"github.com/golang/snappy"
	"github.com/google/uuid"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

type (
	// domainAuditManagerImpl implements DomainAuditManager based on DomainAuditStore and PayloadSerializer
	//
	// a future implementation which may wish to get-by-ID should use the following logic to do so
	// eventID, _ := uuid.Parse(request.EventID)
	// createdTime := time.Unix(eventID.Time().UnixTime())
	// return m.persistence.GetDomainAuditLog(ctx, &GetDomainAuditLogsRequest{
	// 	  DomainID: request.DomainID,
	// 	  OperationType: request.OperationType,
	// 	  EventID: request.EventID,
	// 	  CreatedTime: createdTime, // part of the primary key
	// })
	domainAuditManagerImpl struct {
		serializer  PayloadSerializer
		persistence DomainAuditStore
		logger      log.Logger
		timeSrc     clock.TimeSource
		dc          *DynamicConfiguration
	}
)

// NewDomainAuditManagerImpl returns new DomainAuditManager
func NewDomainAuditManagerImpl(persistence DomainAuditStore, logger log.Logger, serializer PayloadSerializer, dc *DynamicConfiguration) DomainAuditManager {
	return &domainAuditManagerImpl{
		serializer:  serializer,
		persistence: persistence,
		logger:      logger,
		timeSrc:     clock.NewRealTimeSource(),
		dc:          dc,
	}
}

func (m *domainAuditManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *domainAuditManagerImpl) Close() {
	m.persistence.Close()
}

func (m *domainAuditManagerImpl) CreateDomainAuditLog(
	ctx context.Context,
	request *CreateDomainAuditLogRequest,
) (*CreateDomainAuditLogResponse, error) {

	// validation
	eventID, err := uuid.Parse(request.EventID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event ID: %w", err)
	}
	if eventID.Version() != 7 {
		return nil, fmt.Errorf("event ID must be a UUID v7: %w", err)
	}

	encodingType := constants.EncodingTypeThriftRWSnappy

	// Serialize StateBefore using thrift+snappy
	var stateBeforeBlob *DataBlob
	if request.StateBefore != nil {
		blob, err := serializeGetDomainResponse(request.StateBefore, encodingType)
		if err != nil {
			return nil, err
		}
		stateBeforeBlob = blob
	}

	// Serialize StateAfter using thrift+snappy
	var stateAfterBlob *DataBlob
	if request.StateAfter != nil {
		blob, err := serializeGetDomainResponse(request.StateAfter, encodingType)
		if err != nil {
			return nil, err
		}
		stateAfterBlob = blob
	}

	// Get TTL from dynamic config using domain ID
	ttlDuration := m.dc.DomainAuditLogTTL(request.DomainID)

	return m.persistence.CreateDomainAuditLog(ctx, &InternalCreateDomainAuditLogRequest{
		DomainID:        request.DomainID,
		EventID:         request.EventID,
		StateBefore:     stateBeforeBlob,
		StateAfter:      stateAfterBlob,
		OperationType:   request.OperationType,
		CreatedTime:     request.CreatedTime,
		LastUpdatedTime: request.CreatedTime,
		Identity:        request.Identity,
		IdentityType:    request.IdentityType,
		Comment:         request.Comment,
		TTLSeconds:      int64(ttlDuration.Seconds()),
	})
}

func (m *domainAuditManagerImpl) GetDomainAuditLogs(
	ctx context.Context,
	request *GetDomainAuditLogsRequest,
) (*GetDomainAuditLogsResponse, error) {
	internalResp, err := m.persistence.GetDomainAuditLogs(ctx, request)
	if err != nil {
		return nil, err
	}

	// Convert internal audit logs to external format
	auditLogs := make([]*DomainAuditLog, len(internalResp.AuditLogs))
	for i, internalLog := range internalResp.AuditLogs {
		log := &DomainAuditLog{
			EventID:         internalLog.EventID,
			DomainID:        internalLog.DomainID,
			OperationType:   internalLog.OperationType,
			CreatedTime:     internalLog.CreatedTime,
			LastUpdatedTime: internalLog.LastUpdatedTime,
			Identity:        internalLog.Identity,
			IdentityType:    internalLog.IdentityType,
			Comment:         internalLog.Comment,
		}

		// Deserialize StateBefore
		if internalLog.StateBefore != nil && len(internalLog.StateBefore.Data) > 0 {
			stateBefore, err := deserializeGetDomainResponse(internalLog.StateBefore)
			if err != nil {
				return nil, err
			}
			log.StateBefore = stateBefore
		}

		// Deserialize StateAfter
		if internalLog.StateAfter != nil && len(internalLog.StateAfter.Data) > 0 {
			stateAfter, err := deserializeGetDomainResponse(internalLog.StateAfter)
			if err != nil {
				return nil, err
			}
			log.StateAfter = stateAfter
		}

		auditLogs[i] = log
	}

	return &GetDomainAuditLogsResponse{
		AuditLogs:     auditLogs,
		NextPageToken: internalResp.NextPageToken,
	}, nil
}

// serializeGetDomainResponse serializes GetDomainResponse using thrift encoding
func serializeGetDomainResponse(resp *GetDomainResponse, encodingType constants.EncodingType) (*DataBlob, error) {
	if resp == nil {
		return nil, nil
	}

	typesResp := resp.ToType()

	if typesResp.ReplicationConfiguration != nil && typesResp.ReplicationConfiguration.Clusters != nil {
		filtered := make([]*types.ClusterReplicationConfiguration, 0, len(typesResp.ReplicationConfiguration.Clusters))
		for _, cluster := range typesResp.ReplicationConfiguration.Clusters {
			if cluster != nil {
				filtered = append(filtered, cluster)
			}
		}
		typesResp.ReplicationConfiguration.Clusters = filtered
	}

	thriftResp := thrift.FromDescribeDomainResponse(typesResp)

	var data []byte
	var err error

	encoder := codec.NewThriftRWEncoder()
	switch encodingType {
	case constants.EncodingTypeThriftRW:
		data, err = encoder.Encode(thriftResp)
	case constants.EncodingTypeThriftRWSnappy:
		encoded, encodeErr := encoder.Encode(thriftResp)
		if encodeErr != nil {
			return nil, encodeErr
		}
		data = snappy.Encode(nil, encoded)
	default:
		// Fallback to ThriftRWSnappy
		encodingType = constants.EncodingTypeThriftRWSnappy
		encoded, encodeErr := encoder.Encode(thriftResp)
		if encodeErr != nil {
			return nil, encodeErr
		}
		data = snappy.Encode(nil, encoded)
	}

	if err != nil {
		return nil, err
	}

	return &DataBlob{
		Data:     data,
		Encoding: encodingType,
	}, nil
}

// deserializeGetDomainResponse deserializes GetDomainResponse from thrift format
func deserializeGetDomainResponse(blob *DataBlob) (*GetDomainResponse, error) {
	if blob == nil || len(blob.Data) == 0 {
		return nil, nil
	}

	var data []byte
	var err error

	// Decode based on encoding type
	switch blob.Encoding {
	case constants.EncodingTypeThriftRW:
		data = blob.Data
	case constants.EncodingTypeThriftRWSnappy:
		data, err = snappy.Decode(nil, blob.Data)
		if err != nil {
			return nil, err
		}
	default:
		return nil, &UnknownEncodingTypeError{encodingType: blob.Encoding}
	}

	// Decode thrift
	encoder := codec.NewThriftRWEncoder()
	var thriftResp shared.DescribeDomainResponse
	err = encoder.Decode(data, &thriftResp)
	if err != nil {
		return nil, err
	}

	// Convert from thrift to types.DescribeDomainResponse
	typesResp := thrift.ToDescribeDomainResponse(&thriftResp)

	// Convert types.DescribeDomainResponse to persistence.GetDomainResponse
	return FromDescribeDomainResponse(typesResp), nil
}
