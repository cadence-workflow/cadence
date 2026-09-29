package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/client"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
)

func TestCreateSemaphore(t *testing.T) {
	const semaphoreName = "my-semaphore"
	request := func(size, bucketSize int32) *types.CreateSemaphoreRequest {
		return &types.CreateSemaphoreRequest{
			Domain:        testDomain,
			SemaphoreName: semaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}
	}
	stored := func(size, bucketSize int) *persistence.SemaphoreMetadata {
		return &persistence.SemaphoreMetadata{
			DomainID:      testDomainID,
			SemaphoreName: semaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}
	}
	semaphore := func(size, bucketSize int32) *types.CreateSemaphoreResponse {
		return &types.CreateSemaphoreResponse{Semaphore: &types.Semaphore{
			SemaphoreName: semaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}}
	}
	tests := map[string]struct {
		request  *types.CreateSemaphoreRequest
		disabled bool
		// create sets up the metadata store. A nil func expects no call.
		create  func(*persistence.MockSemaphoreMetadataManager)
		want    *types.CreateSemaphoreResponse
		wantErr error
	}{
		"nil request": {
			request: nil,
			wantErr: validate.ErrRequestNotSet,
		},
		"domain not set": {
			request: &types.CreateSemaphoreRequest{SemaphoreName: semaphoreName, Size: 10},
			wantErr: validate.ErrDomainNotSet,
		},
		"disabled for the domain": {
			request:  request(10, 0),
			disabled: true,
			wantErr:  &types.BadRequestError{},
		},
		"semaphore name not set": {
			request: &types.CreateSemaphoreRequest{Domain: testDomain, Size: 10},
			wantErr: &types.BadRequestError{},
		},
		"zero size": {
			request: request(0, 0),
			wantErr: &types.BadRequestError{},
		},
		"negative size": {
			request: request(-1, 0),
			wantErr: &types.BadRequestError{},
		},
		"negative bucket size": {
			request: request(10, -1),
			wantErr: &types.BadRequestError{},
		},
		"bucket size above the maximum": {
			request: request(10, persistence.MaxSemaphoreBucketSize+1),
			wantErr: &types.BadRequestError{},
		},
		"created": {
			request: request(1000, 100),
			create: func(m *persistence.MockSemaphoreMetadataManager) {
				m.EXPECT().CreateSemaphore(gomock.Any(), &persistence.CreateSemaphoreRequest{
					DomainID:      testDomainID,
					SemaphoreName: semaphoreName,
					Size:          1000,
					BucketSize:    100,
				}).Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: semaphore(1000, 100),
		},
		"created with the server's bucket size": {
			request: request(1000, 0),
			create: func(m *persistence.MockSemaphoreMetadataManager) {
				m.EXPECT().CreateSemaphore(gomock.Any(), &persistence.CreateSemaphoreRequest{
					DomainID:      testDomainID,
					SemaphoreName: semaphoreName,
					Size:          1000,
				}).Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: semaphore(1000, 100),
		},
		"create fails": {
			request: request(1000, 100),
			create: func(m *persistence.MockSemaphoreMetadataManager) {
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, &types.InternalServiceError{})
			},
			wantErr: &types.InternalServiceError{},
		},
		"already exists": {
			request: request(1000, 100),
			create: func(m *persistence.MockSemaphoreMetadataManager) {
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).
					Return(nil, &persistence.ConditionFailedError{Msg: "already exists"})
			},
			wantErr: &types.BadRequestError{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, ctrl, metrics.Frontend)
			defer mockResource.Finish(t)

			config := frontendcfg.NewConfig(
				dc.NewCollection(dc.NewInMemoryClient(), mockResource.Logger),
				10, false, "hostname", mockResource.Logger,
			)
			config.EnableDistributedSemaphore = dynamicproperties.GetBoolPropertyFnFilteredByDomain(!tc.disabled)
			handler := NewWorkflowHandler(mockResource, config, client.NewMockVersionChecker(ctrl), nil)

			if tc.create != nil {
				mockResource.DomainCache.EXPECT().GetDomainID(testDomain).Return(testDomainID, nil)
				tc.create(mockResource.SemaphoreMetadataMgr)
			}

			got, err := handler.CreateSemaphore(context.Background(), tc.request)
			if tc.wantErr != nil {
				assert.IsType(t, tc.wantErr, err)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCreateSemaphoreUnknownDomain(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResource := resource.NewTest(t, ctrl, metrics.Frontend)
	defer mockResource.Finish(t)

	config := frontendcfg.NewConfig(
		dc.NewCollection(dc.NewInMemoryClient(), mockResource.Logger),
		10, false, "hostname", mockResource.Logger,
	)
	config.EnableDistributedSemaphore = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	handler := NewWorkflowHandler(mockResource, config, client.NewMockVersionChecker(ctrl), nil)

	notFound := &types.EntityNotExistsError{Message: "domain not found"}
	mockResource.DomainCache.EXPECT().GetDomainID(testDomain).Return("", notFound)

	_, err := handler.CreateSemaphore(context.Background(), &types.CreateSemaphoreRequest{
		Domain:        testDomain,
		SemaphoreName: "my-semaphore",
		Size:          10,
	})
	assert.True(t, errors.Is(err, notFound))
}
