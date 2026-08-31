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

package persistencetests

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
)

type (
	SemaphoreMetadataPersistenceSuite struct {
		*TestBase
		*require.Assertions
	}
)

func (s *SemaphoreMetadataPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *SemaphoreMetadataPersistenceSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *SemaphoreMetadataPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

func (s *SemaphoreMetadataPersistenceSuite) TestCreateAndGetSemaphore() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreMetadataManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()

	createResp, err := manager.CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          100,
		BucketSize:    10,
	})
	s.NoError(err)
	s.NotNil(createResp)
	s.Equal(domainID, createResp.Semaphore.DomainID)
	s.Equal(semaphoreName, createResp.Semaphore.SemaphoreName)
	s.Equal(100, createResp.Semaphore.Size)
	s.Equal(10, createResp.Semaphore.BucketSize)
	s.False(createResp.Semaphore.CreatedTime.IsZero())

	getResp, err := manager.GetSemaphore(ctx, &persistence.GetSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
	})
	s.NoError(err)
	s.NotNil(getResp)
	s.Equal(domainID, getResp.Semaphore.DomainID)
	s.Equal(semaphoreName, getResp.Semaphore.SemaphoreName)
	s.Equal(100, getResp.Semaphore.Size)
	s.Equal(10, getResp.Semaphore.BucketSize)
}

func (s *SemaphoreMetadataPersistenceSuite) TestCreateSemaphoreDefaultBucketSize() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreMetadataManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()

	createResp, err := manager.CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          500,
	})
	s.NoError(err)
	s.Equal(persistence.DefaultSemaphoreBucketSize, createResp.Semaphore.BucketSize)
}

func (s *SemaphoreMetadataPersistenceSuite) TestCreateSemaphoreConflict() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreMetadataManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	semaphoreName := "sem-" + uuid.NewString()

	req := &persistence.CreateSemaphoreRequest{
		DomainID:      domainID,
		SemaphoreName: semaphoreName,
		Size:          100,
		BucketSize:    10,
	}

	_, err = manager.CreateSemaphore(ctx, req)
	s.NoError(err)

	// second create for the same (domainID, semaphoreName) must fail with a condition failure
	_, err = manager.CreateSemaphore(ctx, req)
	s.Error(err)
	_, ok := err.(*persistence.ConditionFailedError)
	s.True(ok, "expected ConditionFailedError, got %T: %v", err, err)
}

func (s *SemaphoreMetadataPersistenceSuite) TestGetSemaphoreNotFound() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreMetadataManager()
	s.NoError(err)
	defer manager.Close()

	_, err = manager.GetSemaphore(ctx, &persistence.GetSemaphoreRequest{
		DomainID:      uuid.NewString(),
		SemaphoreName: "does-not-exist",
	})
	s.Error(err)
}

func (s *SemaphoreMetadataPersistenceSuite) TestListSemaphores() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.PersistenceFactory.NewSemaphoreMetadataManager()
	s.NoError(err)
	defer manager.Close()

	domainID := uuid.NewString()
	numSemaphores := 10

	for i := 0; i < numSemaphores; i++ {
		_, err := manager.CreateSemaphore(ctx, &persistence.CreateSemaphoreRequest{
			DomainID:      domainID,
			SemaphoreName: "sem-" + uuid.NewString(),
			Size:          100,
			BucketSize:    10,
		})
		s.NoError(err)
	}

	pageSize := 3
	var all []*persistence.SemaphoreMetadata
	var nextPageToken []byte
	for {
		listResp, err := manager.ListSemaphores(ctx, &persistence.ListSemaphoresRequest{
			DomainID:      domainID,
			PageSize:      pageSize,
			NextPageToken: nextPageToken,
		})
		s.NoError(err)
		s.NotNil(listResp)
		all = append(all, listResp.Semaphores...)
		if len(listResp.NextPageToken) == 0 {
			break
		}
		nextPageToken = listResp.NextPageToken
	}
	s.Len(all, numSemaphores)
}
