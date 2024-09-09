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

package archiver

import (
	"context"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// ArchiveHistoryRequest is request to Archive workflow history
	ArchiveHistoryRequest struct {
		ShardID              int
		DomainID             string
		DomainName           string
		WorkflowID           string
		RunID                string
		BranchToken          []byte
		NextEventID          int64
		CloseFailoverVersion int64
	}

	// ArchiveExecutionRequest is request to Archive workflow execution records
	ArchiveExecutionRequest struct {
		ShardID    int
		DomainID   string
		DomainName string
		WorkflowID string
		RunID      string
	}

	// GetExecutionRequest is the request to Get archived Execution records
	GetExecutionRequest struct {
		DomainID   string
		WorkflowID string
		RunID      string
	}

	// GetHistoryRequest is the request to Get archived history
	GetHistoryRequest struct {
		DomainID             string
		WorkflowID           string
		RunID                string
		CloseFailoverVersion *int64
		NextPageToken        []byte
		PageSize             int
	}

	// GetHistoryResponse is the response of Get archived history
	GetHistoryResponse struct {
		HistoryBatches []*types.History
		NextPageToken  []byte
	}

	// GetExecutionResponse is the response of Get archived execution records
	GetExecutionResponse struct {
		MutableState *persistence.WorkflowMutableState
	}

	// HistoryBootstrapContainer contains components needed by all history Archiver implementations
	HistoryBootstrapContainer struct {
		HistoryV2Manager persistence.HistoryManager
		Logger           log.Logger
		MetricsClient    metrics.Client
		ClusterMetadata  cluster.Metadata
		DomainCache      cache.DomainCache
	}

	// ExecutionArchiver is used to get and store execution data
	ExecutionArchiver interface {
		ArchiveExecution(context.Context, URI, *ArchiveExecutionRequest, ...ArchiveOption) error
		//GetWorkflowExecutionForPersistence(context.Context, *GetExecutionRequest) (*GetExecutionResponse, error)
		//GetWorkflowExecution(context.Context, URI, *GetExecutionRequest) (*GetExecutionResponse, error)
		ValidateExecutionURI(URI) error
	}

	// Reader is used to get execution data
	ExecutionReader interface {
		GetWorkflowExecutionForPersistence(context.Context, *GetExecutionRequest) (*GetExecutionResponse, error)
		GetWorkflowExecution(context.Context, URI, *GetExecutionRequest) (*GetExecutionResponse, error)
	}

	// HistoryArchiver is used to archive history and read archived history
	HistoryArchiver interface {
		Archive(context.Context, URI, *ArchiveHistoryRequest, ...ArchiveOption) error
		// todo (david.porter) maybe delete this
		//GetWorkflowHistoryForPersistence(context.Context, *GetHistoryRequest) (*GetHistoryResponse, error)
		Get(context.Context, URI, *GetHistoryRequest) (*GetHistoryResponse, error)
		ValidateURI(URI) error
	}

	// VisibilityBootstrapContainer contains components needed by all visibility Archiver implementations
	VisibilityBootstrapContainer struct {
		Logger          log.Logger
		MetricsClient   metrics.Client
		ClusterMetadata cluster.Metadata
		DomainCache     cache.DomainCache
	}

	ExecutionBootstrapContainer struct {
		Logger          log.Logger
		MetricsClient   metrics.Client
		ClusterMetadata cluster.Metadata
		ExecutionMgr    func(shard int) (persistence.ExecutionManager, error)
		DomainCache     cache.DomainCache
	}

	// ArchiveVisibilityRequest is request to Archive single workflow visibility record
	ArchiveVisibilityRequest struct {
		DomainID           string
		DomainName         string // doesn't need to be archived
		WorkflowID         string
		RunID              string
		WorkflowTypeName   string
		StartTimestamp     int64
		ExecutionTimestamp int64
		CloseTimestamp     int64
		CloseStatus        types.WorkflowExecutionCloseStatus
		HistoryLength      int64
		Memo               *types.Memo
		SearchAttributes   map[string]string
		HistoryArchivalURI string
	}

	// QueryVisibilityRequest is the request to query archived visibility records
	QueryVisibilityRequest struct {
		DomainID      string
		PageSize      int
		NextPageToken []byte
		Query         string
	}

	// QueryVisibilityResponse is the response of querying archived visibility records
	QueryVisibilityResponse struct {
		Executions    []*types.WorkflowExecutionInfo
		NextPageToken []byte
	}

	// VisibilityArchiver is used to archive visibility and read archived visibility
	VisibilityArchiver interface {
		Archive(context.Context, URI, *ArchiveVisibilityRequest, ...ArchiveOption) error
		Query(context.Context, URI, *QueryVisibilityRequest) (*QueryVisibilityResponse, error)
		ValidateURI(URI) error
	}

	WarmStorage interface {
		ExecutionArchiver
		HistoryArchiver
	}
)
