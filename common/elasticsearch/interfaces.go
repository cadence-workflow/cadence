// Copyright (c) 2020 Uber Technologies, Inc.
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

package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	esaws "github.com/olivere/elastic/aws/v4"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	esc "github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/elasticsearch/client/os2"
	v6 "github.com/uber/cadence/common/elasticsearch/client/v6"
	v7 "github.com/uber/cadence/common/elasticsearch/client/v7"
	"github.com/uber/cadence/common/elasticsearch/query"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	p "github.com/uber/cadence/common/persistence"
)

// NewGenericClient create a ES client
func NewGenericClient(
	connectConfig *config.ElasticSearchConfig,
	logger log.Logger,
) (*ESClient, error) {
	if connectConfig.Version == "" {
		connectConfig.Version = "v6"
	}
	var tlsClient *http.Client
	var signingAWSClient *http.Client

	if connectConfig.AWSSigning.Enable {
		creds, region, err := connectConfig.AWSSigning.GetCredentials()
		if err != nil {
			return nil, fmt.Errorf("getting AWS credentials: %w", err)
		}

		signingAWSClient = esaws.NewV4SigningClient(creds, *region)
	}

	if connectConfig.TLS.Enabled {
		var err error
		tlsClient, err = buildTLSHTTPClient(connectConfig.TLS)
		if err != nil {
			return nil, err
		}
	}

	var esClient esc.Client
	var err error

	// this logger is only used for client-internal purposes, e.g. request errors, which are generally lack detailed context.
	// it is currently (unfortunately) shared between both queries and bulk-updates.
	clientLogger := logger.WithTags(tag.ComponentESVisibilityClient)

	switch connectConfig.Version {
	case "v6":
		esClient, err = v6.NewV6Client(connectConfig, clientLogger, tlsClient, signingAWSClient)
	case "v7":
		esClient, err = v7.NewV7Client(connectConfig, clientLogger, tlsClient, signingAWSClient)
	case "os2":
		esClient, err = os2.NewClient(connectConfig, clientLogger, tlsClient)
	default:
		return nil, fmt.Errorf("not supported ElasticSearch version: %v", connectConfig.Version)
	}

	if err != nil {
		return nil, err
	}

	return &ESClient{
		Client: esClient,
		Logger: logger,
	}, nil
}

type (
	// GenericClient is a generic interface for all versions of ElasticSearch clients
	GenericClient interface {
		// Search API is only for supporting various List[Open/Closed]WorkflowExecutions(ByXyz).
		// Use SearchByQuery or ScanByQuery for generic purpose searching.
		Search(ctx context.Context, request *SearchRequest) (*SearchResponse, error)
		// SearchByQuery is the generic purpose searching
		SearchByQuery(ctx context.Context, request *SearchByQueryRequest) (*SearchResponse, error)
		// SearchRaw is for searching with raw json. Returns RawResult object which is subset of ESv6 and ESv7 response
		SearchRaw(ctx context.Context, index, query string) (*RawResponse, error)
		// ScanByQuery is also generic purpose searching, but implemented with ScrollService of ElasticSearch,
		// which is more performant for pagination, but comes with some limitation of in-parallel requests.
		ScanByQuery(ctx context.Context, request *ScanByQueryRequest) (*SearchResponse, error)
		// TODO remove it in https://github.com/uber/cadence/issues/3682
		SearchForOneClosedExecution(ctx context.Context, index string, request *SearchForOneClosedExecutionRequest) (*SearchForOneClosedExecutionResponse, error)
		// CountByQuery is for returning the count of workflow executions that match the query
		CountByQuery(ctx context.Context, index, query string) (int64, error)

		// RunBulkProcessor returns a processor for adding/removing docs into ElasticSearch index
		RunBulkProcessor(ctx context.Context, p *bulk.BulkProcessorParameters) (bulk.GenericBulkProcessor, error)

		// PutMapping adds new field type to the index
		PutMapping(ctx context.Context, index, root, key, valueType string) error
		// CreateIndex creates a new index
		CreateIndex(ctx context.Context, index string) error

		IsNotFoundError(err error) bool
	}

	// SearchRequest is request for Search
	SearchRequest struct {
		Index           string
		ListRequest     *p.InternalListWorkflowExecutionsRequest
		IsOpen          bool
		Filter          IsRecordValidFilter
		MatchQuery      *query.MatchQuery
		MaxResultWindow int
	}

	// SearchByQueryRequest is request for SearchByQuery
	SearchByQueryRequest struct {
		Index           string
		Query           string
		NextPageToken   []byte
		PageSize        int
		Filter          IsRecordValidFilter
		MaxResultWindow int
	}

	// ScanByQueryRequest is request for SearchByQuery
	ScanByQueryRequest struct {
		Index         string
		Query         string
		NextPageToken []byte
		PageSize      int
	}

	// SearchResponse is a response to Search, SearchByQuery and ScanByQuery
	SearchResponse = p.InternalListWorkflowExecutionsResponse

	// SearchForOneClosedExecutionRequest is request for SearchForOneClosedExecution
	SearchForOneClosedExecutionRequest = p.InternalGetClosedWorkflowExecutionRequest

	// SearchForOneClosedExecutionResponse is response for SearchForOneClosedExecution
	SearchForOneClosedExecutionResponse = p.InternalGetClosedWorkflowExecutionResponse

	// VisibilityRecord is a struct of doc for deserialization
	VisibilityRecord struct {
		WorkflowID             string
		RunID                  string
		WorkflowType           string
		DomainID               string
		StartTime              int64
		ExecutionTime          int64
		CloseTime              int64
		CloseStatus            workflow.WorkflowExecutionCloseStatus
		HistoryLength          int64
		Memo                   []byte
		Encoding               string
		TaskList               string
		IsCron                 bool
		NumClusters            int16
		ClusterAttributeScope  string
		ClusterAttributeName   string
		UpdateTime             int64
		Attr                   map[string]interface{}
		CronSchedule           string
		ExecutionStatus        int32
		ScheduledExecutionTime int64
	}

	SearchHits struct {
		TotalHits int64
		Hits      []*p.InternalVisibilityWorkflowExecutionInfo
	}

	RawResponse struct {
		TookInMillis int64
		Hits         SearchHits
		Aggregations map[string]json.RawMessage
	}

	// IsRecordValidFilter is a function to filter visibility records
	IsRecordValidFilter func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool
)
