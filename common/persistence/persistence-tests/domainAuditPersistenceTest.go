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
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
)

type (
	DomainAuditPersistenceSuite struct {
		*TestBase
		*require.Assertions
	}
)

func (s *DomainAuditPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

func (s *DomainAuditPersistenceSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *DomainAuditPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

func (s *DomainAuditPersistenceSuite) TestCreateAndGetDomainAuditLog() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	eventID := generateUUIDv7().String()
	now := time.Now().UTC()

	createReq := &persistence.CreateDomainAuditLogRequest{
		DomainID:      domainID,
		EventID:       eventID,
		OperationType: persistence.DomainAuditOperationTypeCreate,
		CreatedTime:   now,
		Identity:      "test-user",
		IdentityType:  "user",
		Comment:       "Test domain creation",
		StateAfter: &persistence.GetDomainResponse{
			Info: &persistence.DomainInfo{
				ID:          domainID,
				Name:        "test-domain",
				Status:      persistence.DomainStatusRegistered,
				Description: "Test domain",
			},
		},
	}

	createResp, err := manager.CreateDomainAuditLog(ctx, createReq)
	s.NoError(err)
	s.NotNil(createResp)
	s.Equal(eventID, createResp.EventID)

	getReq := &persistence.GetDomainAuditLogsRequest{
		DomainID:      domainID,
		OperationType: persistence.DomainAuditOperationTypeCreate,
		PageSize:      10,
	}

	getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
	s.NoError(err)
	s.NotNil(getResp)
	s.Len(getResp.AuditLogs, 1)

	auditLog := getResp.AuditLogs[0]
	s.Equal(eventID, auditLog.EventID)
	s.Equal(domainID, auditLog.DomainID)
	s.Equal(persistence.DomainAuditOperationTypeCreate, auditLog.OperationType)
	s.Equal("test-user", auditLog.Identity)
	s.Equal("user", auditLog.IdentityType)
	s.Equal("Test domain creation", auditLog.Comment)
	s.Nil(auditLog.StateBefore)
	s.NotNil(auditLog.StateAfter)
	s.Equal(domainID, auditLog.StateAfter.Info.ID)
	s.Equal("test-domain", auditLog.StateAfter.Info.Name)
}

func (s *DomainAuditPersistenceSuite) TestCreateMultipleAuditLogs() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		eventID := generateUUIDv7().String()
		createReq := &persistence.CreateDomainAuditLogRequest{
			DomainID:      domainID,
			EventID:       eventID,
			OperationType: persistence.DomainAuditOperationTypeUpdate,
			CreatedTime:   now.Add(time.Duration(i) * time.Millisecond),
			Identity:      "test-user",
			IdentityType:  "user",
			Comment:       "Update operation",
		}

		_, err := manager.CreateDomainAuditLog(ctx, createReq)
		s.NoError(err)
		time.Sleep(2 * time.Millisecond)
	}

	getReq := &persistence.GetDomainAuditLogsRequest{
		DomainID:      domainID,
		OperationType: persistence.DomainAuditOperationTypeUpdate,
		PageSize:      10,
	}

	getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
	s.NoError(err)
	s.NotNil(getResp)
	s.Len(getResp.AuditLogs, 5)
}

func (s *DomainAuditPersistenceSuite) TestGetDomainAuditLogsPagination() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	now := time.Now().UTC()
	numLogs := 10

	for i := 0; i < numLogs; i++ {
		eventID := generateUUIDv7().String()
		createReq := &persistence.CreateDomainAuditLogRequest{
			DomainID:      domainID,
			EventID:       eventID,
			OperationType: persistence.DomainAuditOperationTypeUpdate,
			CreatedTime:   now.Add(time.Duration(i) * time.Millisecond),
			Identity:      "test-user",
			IdentityType:  "user",
		}

		_, err := manager.CreateDomainAuditLog(ctx, createReq)
		s.NoError(err)
		time.Sleep(2 * time.Millisecond)
	}

	pageSize := 3
	var allLogs []*persistence.DomainAuditLog
	var nextPageToken []byte

	for {
		getReq := &persistence.GetDomainAuditLogsRequest{
			DomainID:      domainID,
			OperationType: persistence.DomainAuditOperationTypeUpdate,
			PageSize:      pageSize,
			NextPageToken: nextPageToken,
		}

		getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
		s.NoError(err)
		s.NotNil(getResp)

		allLogs = append(allLogs, getResp.AuditLogs...)

		if len(getResp.NextPageToken) == 0 {
			break
		}
		nextPageToken = getResp.NextPageToken
	}

	s.Len(allLogs, numLogs)
}

func (s *DomainAuditPersistenceSuite) TestGetDomainAuditLogsByOperationType() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		eventID := generateUUIDv7().String()
		createReq := &persistence.CreateDomainAuditLogRequest{
			DomainID:      domainID,
			EventID:       eventID,
			OperationType: persistence.DomainAuditOperationTypeCreate,
			CreatedTime:   now.Add(time.Duration(i) * time.Millisecond),
			Identity:      "test-user",
			IdentityType:  "user",
		}
		_, err := manager.CreateDomainAuditLog(ctx, createReq)
		s.NoError(err)
		time.Sleep(2 * time.Millisecond)
	}

	for i := 0; i < 5; i++ {
		eventID := generateUUIDv7().String()
		createReq := &persistence.CreateDomainAuditLogRequest{
			DomainID:      domainID,
			EventID:       eventID,
			OperationType: persistence.DomainAuditOperationTypeUpdate,
			CreatedTime:   now.Add(time.Duration(i+10) * time.Millisecond),
			Identity:      "test-user",
			IdentityType:  "user",
		}
		_, err := manager.CreateDomainAuditLog(ctx, createReq)
		s.NoError(err)
		time.Sleep(2 * time.Millisecond)
	}

	getReq := &persistence.GetDomainAuditLogsRequest{
		DomainID:      domainID,
		OperationType: persistence.DomainAuditOperationTypeCreate,
		PageSize:      10,
	}

	getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
	s.NoError(err)
	s.NotNil(getResp)
	s.Len(getResp.AuditLogs, 3)
	for _, log := range getResp.AuditLogs {
		s.Equal(persistence.DomainAuditOperationTypeCreate, log.OperationType)
	}
}

func (s *DomainAuditPersistenceSuite) TestGetDomainAuditLogsByTimeRange() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	baseTime := time.Now().UTC()

	for i := 0; i < 10; i++ {
		eventID := generateUUIDv7().String()
		createdTime := baseTime.Add(time.Duration(i) * time.Second)
		createReq := &persistence.CreateDomainAuditLogRequest{
			DomainID:      domainID,
			EventID:       eventID,
			OperationType: persistence.DomainAuditOperationTypeUpdate,
			CreatedTime:   createdTime,
			Identity:      "test-user",
			IdentityType:  "user",
		}
		_, err := manager.CreateDomainAuditLog(ctx, createReq)
		s.NoError(err)
		time.Sleep(2 * time.Millisecond)
	}

	minTime := baseTime.Add(3 * time.Second)
	maxTime := baseTime.Add(7 * time.Second)

	getReq := &persistence.GetDomainAuditLogsRequest{
		DomainID:       domainID,
		OperationType:  persistence.DomainAuditOperationTypeUpdate,
		MinCreatedTime: &minTime,
		MaxCreatedTime: &maxTime,
		PageSize:       10,
	}

	getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
	s.NoError(err)
	s.NotNil(getResp)
	s.True(len(getResp.AuditLogs) >= 4 && len(getResp.AuditLogs) <= 5)

	for _, log := range getResp.AuditLogs {
		s.True(!log.CreatedTime.Truncate(time.Millisecond).Before(minTime.Truncate(time.Millisecond)))
		s.True(!log.CreatedTime.Truncate(time.Millisecond).After(maxTime.Truncate(time.Millisecond)))
	}
}

func (s *DomainAuditPersistenceSuite) TestDomainAuditLogWithStateBefore() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	manager, err := s.ExecutionMgrFactory.NewDomainAuditManager()
	s.NoError(err)
	s.NotNil(manager)
	defer manager.Close()

	domainID := uuid.NewString()
	eventID := generateUUIDv7().String()
	now := time.Now().UTC()

	createReq := &persistence.CreateDomainAuditLogRequest{
		DomainID:      domainID,
		EventID:       eventID,
		OperationType: persistence.DomainAuditOperationTypeUpdate,
		CreatedTime:   now,
		Identity:      "test-user",
		IdentityType:  "user",
		Comment:       "Update domain description",
		StateBefore: &persistence.GetDomainResponse{
			Info: &persistence.DomainInfo{
				ID:          domainID,
				Name:        "test-domain",
				Status:      persistence.DomainStatusRegistered,
				Description: "Old description",
			},
		},
		StateAfter: &persistence.GetDomainResponse{
			Info: &persistence.DomainInfo{
				ID:          domainID,
				Name:        "test-domain",
				Status:      persistence.DomainStatusRegistered,
				Description: "New description",
			},
		},
	}

	_, err = manager.CreateDomainAuditLog(ctx, createReq)
	s.NoError(err)

	getReq := &persistence.GetDomainAuditLogsRequest{
		DomainID:      domainID,
		OperationType: persistence.DomainAuditOperationTypeUpdate,
		PageSize:      10,
	}

	getResp, err := manager.GetDomainAuditLogs(ctx, getReq)
	s.NoError(err)
	s.NotNil(getResp)
	s.Len(getResp.AuditLogs, 1)

	auditLog := getResp.AuditLogs[0]
	s.NotNil(auditLog.StateBefore)
	s.NotNil(auditLog.StateAfter)
	s.Equal("Old description", auditLog.StateBefore.Info.Description)
	s.Equal("New description", auditLog.StateAfter.Info.Description)
}

func generateUUIDv7() uuid.UUID {
	id, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	now := time.Now()
	unixMillis := now.UnixMilli()

	id[0] = byte(unixMillis >> 40)
	id[1] = byte(unixMillis >> 32)
	id[2] = byte(unixMillis >> 24)
	id[3] = byte(unixMillis >> 16)
	id[4] = byte(unixMillis >> 8)
	id[5] = byte(unixMillis)

	id[6] = (id[6] & 0x0f) | 0x70
	id[8] = (id[8] & 0x3f) | 0x80

	return id
}
