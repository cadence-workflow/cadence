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

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
)

type semaphoreTokenManagerImpl struct {
	persistence SemaphoreTokenStore
	logger      log.Logger
	timeSrc     clock.TimeSource
}

// NewSemaphoreTokenManagerImpl returns a new SemaphoreTokenManager
func NewSemaphoreTokenManagerImpl(persistence SemaphoreTokenStore, logger log.Logger) SemaphoreTokenManager {
	return &semaphoreTokenManagerImpl{
		persistence: persistence,
		logger:      logger,
		timeSrc:     clock.NewRealTimeSource(),
	}
}

func (m *semaphoreTokenManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *semaphoreTokenManagerImpl) Close() {
	m.persistence.Close()
}

func (m *semaphoreTokenManagerImpl) SeedSemaphoreTokens(
	ctx context.Context,
	request *SeedSemaphoreTokensRequest,
) error {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return err
	}
	if len(request.TokenIDs) == 0 {
		return fmt.Errorf("TokenIDs is required")
	}
	for _, id := range request.TokenIDs {
		if id < 1 {
			return fmt.Errorf("TokenID must be positive, got %d", id)
		}
	}
	return m.persistence.SeedSemaphoreTokens(ctx, request, m.timeSrc.Now().UTC())
}

func (m *semaphoreTokenManagerImpl) GrantSemaphoreToken(
	ctx context.Context,
	request *GrantSemaphoreTokenRequest,
) (*GrantSemaphoreTokenResponse, error) {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.TokenID < 1 {
		return nil, fmt.Errorf("TokenID must be positive, got %d", request.TokenID)
	}
	if request.OwnerID == "" {
		return nil, fmt.Errorf("OwnerID is required")
	}
	return m.persistence.GrantSemaphoreToken(ctx, request, m.timeSrc.Now().UTC())
}

func (m *semaphoreTokenManagerImpl) ReleaseSemaphoreToken(
	ctx context.Context,
	request *ReleaseSemaphoreTokenRequest,
) (*ReleaseSemaphoreTokenResponse, error) {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.TokenID < 1 {
		return nil, fmt.Errorf("TokenID must be positive, got %d", request.TokenID)
	}
	if request.OwnerID == "" {
		return nil, fmt.Errorf("OwnerID is required")
	}
	applied, err := m.persistence.ReleaseSemaphoreToken(ctx, request, m.timeSrc.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &ReleaseSemaphoreTokenResponse{Applied: applied}, nil
}

func (m *semaphoreTokenManagerImpl) GetSemaphoreOwnershipByToken(
	ctx context.Context,
	request *GetSemaphoreOwnershipByTokenRequest,
) (*GetSemaphoreOwnershipByTokenResponse, error) {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.TokenID < 1 {
		return nil, fmt.Errorf("TokenID must be positive, got %d", request.TokenID)
	}
	ownership, err := m.persistence.GetSemaphoreOwnershipByToken(ctx, request)
	if err != nil {
		return nil, err
	}
	return &GetSemaphoreOwnershipByTokenResponse{Ownership: ownership}, nil
}

func (m *semaphoreTokenManagerImpl) GetSemaphoreOwnershipByOwner(
	ctx context.Context,
	request *GetSemaphoreOwnershipByOwnerRequest,
) (*GetSemaphoreOwnershipByOwnerResponse, error) {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	if request.OwnerID == "" {
		return nil, fmt.Errorf("OwnerID is required")
	}
	ownership, err := m.persistence.GetSemaphoreOwnershipByOwner(ctx, request)
	if err != nil {
		return nil, err
	}
	return &GetSemaphoreOwnershipByOwnerResponse{Ownership: ownership}, nil
}

func (m *semaphoreTokenManagerImpl) ScanSemaphoreBucket(
	ctx context.Context,
	request *ScanSemaphoreBucketRequest,
) (*ScanSemaphoreBucketResponse, error) {
	if err := validateSemaphoreBucket(request.DomainID, request.SemaphoreName, request.Bucket); err != nil {
		return nil, err
	}
	return m.persistence.ScanSemaphoreBucket(ctx, request)
}

func validateSemaphoreBucket(domainID, semaphoreName string, bucket int) error {
	if domainID == "" {
		return fmt.Errorf("DomainID is required")
	}
	if semaphoreName == "" {
		return fmt.Errorf("SemaphoreName is required")
	}
	if bucket < 0 {
		return fmt.Errorf("Bucket must not be negative, got %d", bucket)
	}
	return nil
}
