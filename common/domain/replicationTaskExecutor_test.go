// Copyright (c) 2024 Uber Technologies, Inc.
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

package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestDomainReplicationTaskExecutor_Execute(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager)
		task      *types.DomainTaskAttributes
		wantErr   bool
		errType   interface{}
	}{
		{
			name: "Validate Domain Task - Empty Task",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				// No setup required as the task itself is nil, triggering the validation error
			},
			task:    nil,
			wantErr: true,
			errType: &types.BadRequestError{},
		},
		{
			name: "Handle Create Domain Task - Valid",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainResponse{ID: "validDomainID"}, nil).
					Times(1)

				// Expect GetDomain for audit log
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{ID: "validDomainID"}).
					Return(&persistence.GetDomainResponse{
						Info: &persistence.DomainInfo{
							ID:   "validDomainID",
							Name: "validDomain",
						},
					}, nil).
					Times(1)

				// Expect audit log creation
				mockAuditManager.EXPECT().
					CreateDomainAuditLog(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainAuditLogResponse{}, nil).
					Times(1)
			},
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperationCreate.Ptr(),
				ID:              "validDomainID",
				Info: &types.DomainInfo{
					Name:        "validDomain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "A valid domain",
					OwnerEmail:  "owner@example.com",
					Data:        map[string]string{"k1": "v1"},
				},
				Config: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "test://history",
					VisibilityArchivalStatus:               types.ArchivalStatusEnabled.Ptr(),
					VisibilityArchivalURI:                  "test://visibility",
				},
				ReplicationConfig: &types.DomainReplicationConfiguration{
					ActiveClusterName: "activeClusterName",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "activeClusterName"},
						{ClusterName: "standbyClusterName"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"region1": {
										ActiveClusterName: "activeClusterName",
									},
									"region2": {
										ActiveClusterName: "standbyClusterName",
									},
								},
							},
						},
					},
				},
				ConfigVersion:   1,
				FailoverVersion: 1,
			},
			wantErr: false,
		},
		{
			name: "Handle Create Domain Task - Name UUID Collision",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				// call to GetDomain simulates a name collision by returning a different domain ID
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{Name: "collisionDomain"}).
					Return(&persistence.GetDomainResponse{Info: &persistence.DomainInfo{ID: uuid.New()}}, nil).
					Times(1)

				// Expect CreateDomain to be called, which should result in a collision error
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrNameUUIDCollision).
					Times(1)
			},
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperationCreate.Ptr(),
				ID:              uuid.New(),
				Info: &types.DomainInfo{
					Name:        "collisionDomain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "A domain with UUID collision",
					OwnerEmail:  "owner@example.com",
					Data:        map[string]string{"k1": "v1"},
				},
				Config: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "test://history",
					VisibilityArchivalStatus:               types.ArchivalStatusEnabled.Ptr(),
					VisibilityArchivalURI:                  "test://visibility",
				},
				ReplicationConfig: &types.DomainReplicationConfiguration{
					ActiveClusterName: "activeClusterName",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "activeClusterName"},
						{ClusterName: "standbyClusterName"},
					},
				},
				ConfigVersion:   1,
				FailoverVersion: 1,
			},
			wantErr: true,
			errType: &types.BadRequestError{},
		},
		{
			name: "Handle Update Domain Task - Valid Update",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&persistence.GetMetadataResponse{NotificationVersion: 123}, nil).
					Times(1)

				// Mock GetDomain to simulate domain fetch before update (using ID)
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{ID: "existingDomainID"}).
					Return(&persistence.GetDomainResponse{
						Info:              &persistence.DomainInfo{ID: "existingDomainID", Name: "existingDomainName"},
						Config:            &persistence.DomainConfig{},
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						ConfigVersion:     1,
						FailoverVersion:   50,
					}, nil).Times(1)

				// Mock UpdateDomain to simulate a successful domain update
				mockDomainManager.EXPECT().
					UpdateDomain(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Mock GetDomain after update for audit log
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{ID: "existingDomainID"}).
					Return(&persistence.GetDomainResponse{
						Info:              &persistence.DomainInfo{ID: "existingDomainID", Name: "existingDomainName"},
						Config:            &persistence.DomainConfig{},
						ReplicationConfig: &persistence.DomainReplicationConfig{},
						ConfigVersion:     2,
						FailoverVersion:   100,
					}, nil).Times(1)

				// Expect audit log creation
				mockAuditManager.EXPECT().
					CreateDomainAuditLog(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainAuditLogResponse{}, nil).
					Times(1)
			},
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperationUpdate.Ptr(),
				ID:              "existingDomainID",
				Info: &types.DomainInfo{
					Name:        "existingDomainName",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "Updated description",
					OwnerEmail:  "updatedOwner@example.com",
					Data:        map[string]string{"updatedKey": "updatedValue"},
				},
				Config: &types.DomainConfiguration{},
				ReplicationConfig: &types.DomainReplicationConfiguration{
					ActiveClusterName: "activeClusterName",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "activeClusterName"},
						{ClusterName: "standbyClusterName"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"region1": {
										ActiveClusterName: "activeClusterName",
									},
								},
							},
						},
					},
				},
				ConfigVersion:   2,
				FailoverVersion: 100,
			},
			wantErr: false,
		},
		{
			name: "Handle Invalid Domain Operation",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				// No mock setup is required as the operation should not proceed to any database calls
			},
			task: &types.DomainTaskAttributes{
				// Set up a task without a valid DomainOperation or with an unrecognized operation
				DomainOperation: nil, // Assuming this would not match any specific case
				ID:              "invalidOperationDomainID",
				Info: &types.DomainInfo{
					Name:        "invalidOperationDomain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "Domain with invalid operation",
					OwnerEmail:  "owner@example.com",
					Data:        map[string]string{"k1": "v1"},
				},
				Config:            &types.DomainConfiguration{},
				ReplicationConfig: &types.DomainReplicationConfiguration{},
			},
			wantErr: true,
			errType: &types.BadRequestError{},
		},
		{
			name: "Handle Unsupported Domain Operation",
			setupMock: func(mockDomainManager persistence.MockDomainManager, mockAuditManager persistence.MockDomainAuditManager) {
				// No mock setup is needed as the operation should immediately return an error
			},
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperation(999).Ptr(), // Assuming 999 is not a valid operation
				ID:              "someDomainID",
				Info: &types.DomainInfo{
					Name:        "someDomain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "A domain with an unsupported operation",
					OwnerEmail:  "email@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Config: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "uri://history",
					VisibilityArchivalStatus:               types.ArchivalStatusEnabled.Ptr(),
					VisibilityArchivalURI:                  "uri://visibility",
				},
				ReplicationConfig: &types.DomainReplicationConfiguration{
					ActiveClusterName: "activeCluster",
					Clusters:          []*types.ClusterReplicationConfiguration{{ClusterName: "activeCluster"}},
				},
			},
			wantErr: true,
			errType: &types.BadRequestError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainManager := persistence.NewMockDomainManager(ctrl)
			mockDomainAuditManager := persistence.NewMockDomainAuditManager(ctrl)
			mockTimeSource := clock.NewRealTimeSource()
			mockLogger := log.NewNoop()
			enableAuditLogging := func(...dynamicproperties.FilterOption) bool { return true }

			executor := NewReplicationTaskExecutor(mockDomainManager, mockDomainAuditManager, mockTimeSource, mockLogger, enableAuditLogging).(*domainReplicationTaskExecutorImpl)
			tt.setupMock(*mockDomainManager, *mockDomainAuditManager)
			err := executor.Execute(tt.task)
			if tt.wantErr {
				require.Error(t, err)
				assert.IsType(t, tt.errType, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func domainCreationTask() *types.DomainTaskAttributes {
	return &types.DomainTaskAttributes{
		DomainOperation: types.DomainOperationCreate.Ptr(),
		ID:              "testDomainID",
		Info: &types.DomainInfo{
			Name:        "testDomain",
			Status:      types.DomainStatusRegistered.Ptr(),
			Description: "This is a test domain",
			OwnerEmail:  "owner@test.com",
			Data:        map[string]string{"key1": "value1"}, // Arbitrary domain metadata
		},
		Config: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: 10,
			EmitMetric:                             true,
			HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
			HistoryArchivalURI:                     "test://history/archival",
			VisibilityArchivalStatus:               types.ArchivalStatusEnabled.Ptr(),
			VisibilityArchivalURI:                  "test://visibility/archival",
		},
		ReplicationConfig: &types.DomainReplicationConfiguration{
			ActiveClusterName: "activeClusterName",
			Clusters: []*types.ClusterReplicationConfiguration{
				{
					ClusterName: "activeClusterName",
				},
				{
					ClusterName: "standbyClusterName",
				},
			},
		},
		ConfigVersion:           1,
		FailoverVersion:         1,
		PreviousFailoverVersion: 0,
	}
}

func TestHandleDomainCreationReplicationTask(t *testing.T) {
	tests := []struct {
		name      string
		task      *types.DomainTaskAttributes
		setup     func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager)
		wantError bool
	}{
		{
			name: "Successful Domain Creation",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainResponse{ID: "testDomainID"}, nil)

				// Expect GetDomain for audit log
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{ID: "testDomainID"}).
					Return(&persistence.GetDomainResponse{
						Info: &persistence.DomainInfo{
							ID:   "testDomainID",
							Name: "testDomain",
						},
					}, nil)

				// Expect audit log creation
				mockAuditManager.EXPECT().
					CreateDomainAuditLog(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainAuditLogResponse{}, nil)
			},
			wantError: false,
		},
		{
			name: "Generic Error During Domain Creation",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, types.InternalServiceError{Message: "an internal error"}).
					Times(1)

				// Since CreateDomain failed, handleDomainCreationReplicationTask check for domain existence by name and ID
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{}). // Simulate that no domain exists with the given name/ID
					AnyTimes()
			},
			wantError: true,
		},
		{
			name: "Handle Name/UUID Collision - EntityNotExistsError",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrNameUUIDCollision).Times(1)

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{}).AnyTimes()
			},
			wantError: true,
		},
		{
			name: "Immediate Error Return from CreateDomain",
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, types.InternalServiceError{Message: "internal error"}).
					Times(1)
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrInvalidDomainStatus).
					AnyTimes()
			},
			task:      domainCreationTask(),
			wantError: true,
		},
		{
			name: "Domain Creation with Nil Status",
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperationCreate.Ptr(),
				ID:              "testDomainID",
				Info: &types.DomainInfo{
					Name: "testDomain",
					// Status is intentionally left as nil to trigger the error
				},
			},
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				// No need to set up a mock for CreateDomain as the call should not reach this point
			},
			wantError: true,
		},
		{
			name: "Domain Creation with Unrecognized Status",
			task: &types.DomainTaskAttributes{
				DomainOperation: types.DomainOperationCreate.Ptr(),
				ID:              "testDomainID",
				Info: &types.DomainInfo{
					Name:   "testDomain",
					Status: types.DomainStatus(999).Ptr(), // Assuming 999 is an unrecognized status
				},
			},
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				// As before, no need for mock setup for CreateDomain
			},
			wantError: true,
		},
		{
			name: "Unexpected Error Type from GetDomain Leads to Default Error Handling",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrInvalidDomainStatus).Times(1)

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("unexpected error")).Times(1)
			},
			wantError: true,
		},
		{
			name: "Successful GetDomain with Name/UUID Mismatch",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrNameUUIDCollision).AnyTimes()

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.GetDomainResponse{
						Info: &persistence.DomainInfo{ID: "testDomainID", Name: "mismatchName"},
					}, nil).AnyTimes()
			},
			wantError: true,
		},
		{
			name: "Handle Domain Creation with Unhandled Error",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{}).
					AnyTimes()

				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("unhandled error")).
					Times(1)
			},
			wantError: true,
		},
		{
			name: "Handle Domain Creation - Unexpected Error from GetDomain",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("test error")).Times(1)

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{}).Times(1)

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("unexpected error")).Times(1)
			},
			wantError: true,
		},
		{
			name: "Duplicate Domain Creation With Same ID and Name",
			task: domainCreationTask(),
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(nil, ErrNameUUIDCollision).Times(1)

				// Setup GetDomain to return matching ID and Name, indicating no actual conflict
				// This setup ensures that recordExists becomes true
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.GetDomainResponse{
						Info: &persistence.DomainInfo{ID: "testDomainID", Name: "testDomain"},
					}, nil).Times(2)
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockDomainManager := persistence.NewMockDomainManager(ctrl)
			mockDomainAuditManager := persistence.NewMockDomainAuditManager(ctrl)
			mockLogger := testlogger.New(t)
			mockTimeSource := clock.NewMockedTimeSourceAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) // Fixed time
			enableAuditLogging := func(...dynamicproperties.FilterOption) bool { return true }

			executor := &domainReplicationTaskExecutorImpl{
				domainManager:            mockDomainManager,
				domainAuditManager:       mockDomainAuditManager,
				logger:                   mockLogger,
				timeSource:               mockTimeSource,
				enableDomainAuditLogging: enableAuditLogging,
			}

			tt.setup(mockDomainManager, mockDomainAuditManager)
			err := executor.handleDomainCreationReplicationTask(context.Background(), tt.task)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleDomainUpdateReplicationTask(t *testing.T) {
	tests := []struct {
		name    string
		task    *types.DomainTaskAttributes
		wantErr bool
		setup   func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager)
	}{
		{
			name: "Convert Status Error",
			task: &types.DomainTaskAttributes{
				Info: &types.DomainInfo{
					Status: func() *types.DomainStatus { var ds types.DomainStatus = 999; return &ds }(), // Invalid status to trigger conversion error
				},
			},
			wantErr: true,
			setup:   func(dm *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {},
		},
		{
			name:    "Error Fetching Metadata",
			task:    domainCreationTask(),
			wantErr: true,
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(nil, errors.New("Error getting metadata while handling replication task")).
					AnyTimes()
			},
		},
		{
			name: "GetDomain Returns General Error",
			task: &types.DomainTaskAttributes{
				Info: &types.DomainInfo{
					Name: "someDomain",
				},
			},
			wantErr: true,
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("general error")).AnyTimes()

				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&persistence.GetMetadataResponse{}, nil).AnyTimes()

			},
		},
		{
			name: "GetDomain Returns EntityNotExistsError - Triggers Domain Creation",
			task: &types.DomainTaskAttributes{
				ID: "nonexistentDomainID",
				Info: &types.DomainInfo{
					Name: "nonexistentDomain",
				},
			},
			wantErr: true,
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), &persistence.GetDomainRequest{
						ID: "nonexistentDomainID",
					}).
					Return(nil, &types.EntityNotExistsError{}).AnyTimes()

				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil).
					AnyTimes()

				mockDomainManager.EXPECT().
					CreateDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateDomainResponse{
						ID: "nonexistentDomainID",
					}, nil).
					AnyTimes()
			},
		},
		{
			name: "Record Not Updated then return nil",
			task: &types.DomainTaskAttributes{
				Info: &types.DomainInfo{
					Name:   "testDomain",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				Config: &types.DomainConfiguration{
					BadBinaries: &types.BadBinaries{
						Binaries: map[string]*types.BadBinaryInfo{
							"checksum1": {
								Reason:          "reasontest",
								Operator:        "operatortest",
								CreatedTimeNano: func() *int64 { var ct int64 = 12345; return &ct }(),
							},
						},
					},
				},
			},
			wantErr: false,
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil).AnyTimes()

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.GetDomainResponse{
						Info:              &persistence.DomainInfo{ID: "testDomainID", Name: "testDomain"},
						Config:            &persistence.DomainConfig{},
						ReplicationConfig: &persistence.DomainReplicationConfig{},
					}, nil).
					AnyTimes()

				mockDomainManager.EXPECT().
					UpdateDomain(gomock.Any(), gomock.Any()).
					Return(nil).
					AnyTimes()

			},
		},
		{
			name: "Update Domain with BadBinaries Set",
			task: &types.DomainTaskAttributes{
				Info: &types.DomainInfo{
					Name: "existingDomainName",
				},
				Config: &types.DomainConfiguration{
					BadBinaries: &types.BadBinaries{
						Binaries: map[string]*types.BadBinaryInfo{},
					},
				},
			},
			wantErr: true,
			setup: func(mockDomainManager *persistence.MockDomainManager, mockAuditManager *persistence.MockDomainAuditManager) {
				mockDomainManager.EXPECT().
					GetMetadata(gomock.Any()).
					Return(&persistence.GetMetadataResponse{NotificationVersion: 1}, nil).
					AnyTimes()

				mockDomainManager.EXPECT().
					GetDomain(gomock.Any(), gomock.Any()).
					Return(&persistence.GetDomainResponse{}, nil).
					AnyTimes()

				mockDomainManager.EXPECT().
					UpdateDomain(gomock.Any(), gomock.Any()).
					Return(nil).
					AnyTimes()
			},
		},
	}
	assert := assert.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDomainManager := persistence.NewMockDomainManager(mockCtrl)
			mockDomainAuditManager := persistence.NewMockDomainAuditManager(mockCtrl)
			mockLogger := testlogger.New(t)
			mockTimeSource := clock.NewMockedTimeSourceAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) // Fixed time
			enableAuditLogging := func(...dynamicproperties.FilterOption) bool { return true }

			executor := &domainReplicationTaskExecutorImpl{
				domainManager:            mockDomainManager,
				domainAuditManager:       mockDomainAuditManager,
				logger:                   mockLogger,
				timeSource:               mockTimeSource,
				enableDomainAuditLogging: enableAuditLogging,
			}
			tt.setup(mockDomainManager, mockDomainAuditManager)

			err := executor.handleDomainUpdateReplicationTask(context.Background(), tt.task)
			if tt.wantErr {
				assert.Error(err, "Expected an error for test case: %s", tt.name)
			} else {
				assert.NoError(err, "Expected no error for test case: %s", tt.name)
			}

		})
	}
}
