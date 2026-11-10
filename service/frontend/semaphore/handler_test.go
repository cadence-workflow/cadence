package semaphore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestCreateSemaphore(t *testing.T) {
	const (
		domainName    = "domain"
		domainID      = "domain-id"
		semaphoreName = "semaphore"
	)
	request := func(capacity, bucketCapacity int32) *types.CreateSemaphoreRequest {
		return &types.CreateSemaphoreRequest{
			Domain:         domainName,
			SemaphoreName:  semaphoreName,
			Capacity:       capacity,
			BucketCapacity: bucketCapacity,
		}
	}
	stored := func(size, bucketSize int) *persistence.SemaphoreMetadata {
		return &persistence.SemaphoreMetadata{
			DomainID:      domainID,
			SemaphoreName: semaphoreName,
			Size:          size,
			BucketSize:    bucketSize,
		}
	}
	response := func(capacity, bucketCapacity int32) *types.CreateSemaphoreResponse {
		return &types.CreateSemaphoreResponse{Semaphore: &types.Semaphore{
			SemaphoreName:  semaphoreName,
			Capacity:       capacity,
			BucketCapacity: bucketCapacity,
		}}
	}
	alreadyExists := &persistence.ConditionFailedError{Msg: "already exists"}
	info := &persistence.DomainInfo{ID: domainID, Name: domainName}
	localDomain := cache.NewLocalDomainCacheEntryForTest(info, &persistence.DomainConfig{}, "active")
	globalDomain := cache.NewGlobalDomainCacheEntryForTest(info, &persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{ActiveClusterName: cluster.TestCurrentClusterName}, 1)
	// The primary cluster creates even when the domain is active in another cluster.
	standbyDomain := cache.NewGlobalDomainCacheEntryForTest(info, &persistence.DomainConfig{},
		&persistence.DomainReplicationConfig{ActiveClusterName: cluster.TestAlternativeClusterName}, 1)

	testCases := []struct {
		name    string
		request *types.CreateSemaphoreRequest
		// notPrimary runs the handler in a cluster that is not the primary.
		notPrimary bool
		// setupMocks sets up the domain cache, the metadata store and the replicator.
		setupMocks func(*cache.MockDomainCache, *persistence.MockSemaphoreMetadataManager, *domain.MockSemaphoreReplicator)
		want       *types.CreateSemaphoreResponse
		wantErr    error
	}{
		{
			name:    "when the domain is local, the semaphore metadata is created and not replicated",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), &persistence.CreateSemaphoreRequest{
					DomainID:      domainID,
					SemaphoreName: semaphoreName,
					Size:          1000,
					BucketSize:    100,
				}).Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the bucket capacity is not set, the stored default bucket capacity is returned",
			request: request(1000, 0),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), &persistence.CreateSemaphoreRequest{
					DomainID:      domainID,
					SemaphoreName: semaphoreName,
					Size:          1000,
				}).Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the domain is global, the created semaphore metadata is replicated",
			request: request(1000, 0),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
				r.EXPECT().ReplicateCreate(gomock.Any(), stored(1000, 100)).Return(nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the domain is global and replication fails, the replication error is returned",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
				r.EXPECT().ReplicateCreate(gomock.Any(), gomock.Any()).Return(&types.InternalServiceError{})
			},
			wantErr: &types.InternalServiceError{},
		},
		{
			name:    "when the semaphore metadata already exists with the same settings, the create succeeds",
			request: request(1000, 0),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, alreadyExists)
				m.EXPECT().GetSemaphore(gomock.Any(), &persistence.GetSemaphoreRequest{
					DomainID:      domainID,
					SemaphoreName: semaphoreName,
				}).Return(&persistence.GetSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the domain is global and the semaphore metadata already exists with the same settings, it is replicated again",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, alreadyExists)
				m.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
				r.EXPECT().ReplicateCreate(gomock.Any(), stored(1000, 100)).Return(nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the semaphore metadata already exists with a different capacity, a BadRequestError is returned and nothing is replicated",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, alreadyExists)
				m.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreResponse{Semaphore: stored(500, 100)}, nil)
			},
			wantErr: &types.BadRequestError{},
		},
		{
			name:    "when the semaphore metadata already exists with a different bucket capacity, a BadRequestError is returned",
			request: request(1000, 0),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, alreadyExists)
				m.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.GetSemaphoreResponse{Semaphore: stored(1000, 50)}, nil)
			},
			wantErr: &types.BadRequestError{},
		},
		{
			name:    "when reading the existing semaphore metadata fails, the read error is returned",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, alreadyExists)
				m.EXPECT().GetSemaphore(gomock.Any(), gomock.Any()).Return(nil, &types.InternalServiceError{})
			},
			wantErr: &types.InternalServiceError{},
		},
		{
			name:    "when the domain is global and creating the semaphore metadata fails, the store error is returned and nothing is replicated",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).Return(nil, &types.InternalServiceError{})
			},
			wantErr: &types.InternalServiceError{},
		},
		{
			name:    "when the domain is global and active in another cluster, the primary cluster creates and replicates the semaphore metadata",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(standbyDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
				r.EXPECT().ReplicateCreate(gomock.Any(), stored(1000, 100)).Return(nil)
			},
			want: response(1000, 100),
		},
		{
			name:       "when the domain is global and this cluster is not the primary, a BadRequestError is returned and nothing is written",
			request:    request(1000, 100),
			notPrimary: true,
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(globalDomain, nil)
			},
			wantErr: errNotPrimaryCluster,
		},
		{
			name:       "when the domain is local and this cluster is not the primary, the semaphore metadata is created and not replicated",
			request:    request(1000, 100),
			notPrimary: true,
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(localDomain, nil)
				m.EXPECT().CreateSemaphore(gomock.Any(), gomock.Any()).
					Return(&persistence.CreateSemaphoreResponse{Semaphore: stored(1000, 100)}, nil)
			},
			want: response(1000, 100),
		},
		{
			name:    "when the domain does not exist, the domain lookup error is returned",
			request: request(1000, 100),
			setupMocks: func(dc *cache.MockDomainCache, m *persistence.MockSemaphoreMetadataManager, r *domain.MockSemaphoreReplicator) {
				dc.EXPECT().GetDomain(domainName).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr: &types.EntityNotExistsError{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domainCache := cache.NewMockDomainCache(ctrl)
			semaphoreMetadataManager := persistence.NewMockSemaphoreMetadataManager(ctrl)
			semaphoreReplicator := domain.NewMockSemaphoreReplicator(ctrl)
			tc.setupMocks(domainCache, semaphoreMetadataManager, semaphoreReplicator)
			clusterMetadata := cluster.TestActiveClusterMetadata
			if tc.notPrimary {
				clusterMetadata = cluster.TestPassiveClusterMetadata
			}
			handler := NewHandler(semaphoreMetadataManager, domainCache, clusterMetadata, semaphoreReplicator)

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
