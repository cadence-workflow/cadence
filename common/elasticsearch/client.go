// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/elasticsearch/query"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

var _ GenericClient = (*ESClient)(nil)

type ESClient struct {
	Client client.Client
	Logger log.Logger
}

func (c *ESClient) RunBulkProcessor(ctx context.Context, p *bulk.BulkProcessorParameters) (bulk.GenericBulkProcessor, error) {
	return c.Client.RunBulkProcessor(ctx, p)
}

func (c *ESClient) CreateIndex(ctx context.Context, index string) error {
	return c.Client.CreateIndex(ctx, index)
}

func (c *ESClient) IsNotFoundError(err error) bool {
	return c.Client.IsNotFoundError(err)
}

func (c *ESClient) Search(ctx context.Context, request *SearchRequest) (*SearchResponse, error) {
	token, err := GetNextPageToken(request.ListRequest.NextPageToken)
	if err != nil {
		return nil, err
	}
	searchResult, err := c.getSearchResult(
		ctx,
		request.Index,
		request.ListRequest,
		request.MatchQuery,
		request.IsOpen,
		token,
	)

	if err != nil {
		return nil, err
	}

	return c.getListWorkflowExecutionsResponse(searchResult, token, request.ListRequest.PageSize, request.MaxResultWindow, request.Filter)
}

func (c *ESClient) SearchByQuery(ctx context.Context, request *SearchByQueryRequest) (*SearchResponse, error) {
	token, err := GetNextPageToken(request.NextPageToken)
	if err != nil {
		return nil, err
	}
	searchResult, err := c.Client.Search(ctx, request.Index, request.Query)
	if err != nil {
		return nil, err
	}

	return c.getListWorkflowExecutionsResponse(searchResult, token, request.PageSize, request.MaxResultWindow, request.Filter)
}

func (c *ESClient) SearchRaw(ctx context.Context, index, query string) (*RawResponse, error) {
	response, err := c.Client.Search(ctx, index, query)
	if err != nil {
		return nil, err
	}

	result := RawResponse{
		TookInMillis: response.TookInMillis,
		Hits: SearchHits{
			TotalHits: response.TotalHits,
			Hits:      c.esHitsToExecutions(response.Hits, nil /* no filter */),
		},
		Aggregations: response.Aggregations,
	}

	return &result, nil

}

func (c *ESClient) esHitsToExecutions(eshits *client.SearchHits, filter IsRecordValidFilter) []*p.InternalVisibilityWorkflowExecutionInfo {
	var hits = make([]*p.InternalVisibilityWorkflowExecutionInfo, 0)
	if eshits != nil && len(eshits.Hits) > 0 {
		for _, hit := range eshits.Hits {
			workflowExecutionInfo := c.convertSearchResultToVisibilityRecord(hit)
			if filter == nil || filter(workflowExecutionInfo) {
				hits = append(hits, workflowExecutionInfo)
			}
		}
	}
	return hits
}

func (c *ESClient) ScanByQuery(ctx context.Context, request *ScanByQueryRequest) (*SearchResponse, error) {
	var err error
	token, err := GetNextPageToken(request.NextPageToken)
	if err != nil {
		return nil, err
	}
	searchResult, err := c.Client.Scroll(ctx, request.Index, request.Query, token.ScrollID)

	isLastPage := false
	if err == io.EOF { // no more result
		isLastPage = true
		if err := c.Client.ClearScroll(ctx, searchResult.ScrollID); err != nil {
			c.Logger.Warn("scroll clear failed", tag.Error(err))
		}
	} else if err != nil && isNodeUnavailableError(err) { // scroll ID is no longer valid and points to a node that is unavailable. Query with fresh scroll
		c.Logger.Warn("scroll node not available for id, retrying with fresh scroll",
			tag.Dynamic("scrollID", token.ScrollID),
			tag.Error(err))
		// fresh scroll
		res, scrollErr := c.Client.Scroll(ctx, request.Index, request.Query, "")
		if scrollErr != nil {
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf("ScanByQuery failed after fresh scroll. Error: %v", err),
			}
		}
		// set fresh scroll result to search result
		searchResult = res

	} else if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("ScanByQuery failed. Error: %v", err),
		}
	}

	response := &p.InternalListWorkflowExecutionsResponse{}
	if searchResult == nil {
		return response, nil
	}

	response.Executions = c.esHitsToExecutions(searchResult.Hits, nil /* no filter */)

	if len(searchResult.Hits.Hits) == request.PageSize && !isLastPage {
		nextPageToken, err := SerializePageToken(&ElasticVisibilityPageToken{ScrollID: searchResult.ScrollID})
		if err != nil {
			return nil, err
		}
		response.NextPageToken = make([]byte, len(nextPageToken))
		copy(response.NextPageToken, nextPageToken)
	}

	return response, nil
}

func (c *ESClient) SearchForOneClosedExecution(ctx context.Context, index string, request *SearchForOneClosedExecutionRequest) (*SearchForOneClosedExecutionResponse, error) {
	matchDomainQuery := query.NewMatchQuery(DomainID, request.DomainUUID)
	existClosedStatusQuery := query.NewExistsQuery(CloseStatus)
	matchWorkflowIDQuery := query.NewMatchQuery(WorkflowID, request.Execution.GetWorkflowID())
	boolQuery := query.NewBoolQuery().Must(matchDomainQuery).Must(existClosedStatusQuery).Must(matchWorkflowIDQuery)
	rid := request.Execution.GetRunID()
	if rid != "" {
		matchRunIDQuery := query.NewMatchQuery(RunID, rid)
		boolQuery = boolQuery.Must(matchRunIDQuery)
	}

	qb := query.NewBuilder()
	body, err := qb.Query(boolQuery).String()

	if err != nil {
		return nil, fmt.Errorf("getting body from query builder: %w", err)
	}

	searchResult, err := c.Client.Search(ctx, index, body)
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("SearchForOneClosedExecution failed. Error: %v", err),
		}
	}

	response := &p.InternalGetClosedWorkflowExecutionResponse{}
	actualHits := searchResult.Hits.Hits
	if len(actualHits) == 0 {
		return response, nil
	}
	response.Execution = c.convertSearchResultToVisibilityRecord(actualHits[0])

	return response, nil
}

func (c *ESClient) CountByQuery(ctx context.Context, index, query string) (int64, error) {
	return c.Client.Count(ctx, index, query)
}

func (c *ESClient) PutMapping(ctx context.Context, index, root, key, valueType string) error {
	mapping := buildPutMappingBody(root, key, valueType)
	body, err := json.Marshal(mapping)
	if err != nil {
		return err
	}

	return c.Client.PutMapping(ctx, index, string(body))
}

func (c *ESClient) getListWorkflowExecutionsResponse(
	searchHits *client.Response,
	token *ElasticVisibilityPageToken,
	pageSize int,
	maxResultWindow int,
	isRecordValid func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool,
) (*p.InternalListWorkflowExecutionsResponse, error) {

	response := &p.InternalListWorkflowExecutionsResponse{}
	numOfActualHits := len(searchHits.Hits.Hits)
	response.Executions = make([]*p.InternalVisibilityWorkflowExecutionInfo, 0)
	for i := 0; i < numOfActualHits; i++ {
		workflowExecutionInfo := c.convertSearchResultToVisibilityRecord(searchHits.Hits.Hits[i])
		if isRecordValid == nil || isRecordValid(workflowExecutionInfo) {
			// for old APIs like ListOpenWorkflowExecutions, we added 1 ms to range query to overcome ES limitation
			// (see getSearchResult function), but manually dropped records beyond request range here.
			response.Executions = append(response.Executions, workflowExecutionInfo)
		}
	}

	if numOfActualHits == pageSize { // this means the response is not the last page
		var nextPageToken []byte
		var err error

		// ES Search API support pagination using From and PageSize, but has limit that From+PageSize cannot exceed a threshold
		// to retrieve deeper pages, use ES SearchAfter
		if searchHits.TotalHits <= int64(maxResultWindow-pageSize) { // use ES Search From+Size
			nextPageToken, err = SerializePageToken(&ElasticVisibilityPageToken{From: token.From + numOfActualHits})
		} else { // use ES Search After
			var sortVal interface{}
			sortVal = searchHits.Sort[0]
			tieBreaker := searchHits.Sort[1].(string)
			nextPageToken, err = SerializePageToken(&ElasticVisibilityPageToken{SortValue: sortVal, TieBreaker: tieBreaker})
		}
		if err != nil {
			return nil, err
		}

		response.NextPageToken = make([]byte, len(nextPageToken))
		copy(response.NextPageToken, nextPageToken)
	}

	return response, nil
}

func (c *ESClient) convertSearchResultToVisibilityRecord(hit *client.SearchHit) *p.InternalVisibilityWorkflowExecutionInfo {
	var source *VisibilityRecord
	err := json.Unmarshal(hit.Source, &source)
	if err != nil { // log and skip error
		return nil
	}

	record := &p.InternalVisibilityWorkflowExecutionInfo{
		DomainID:               source.DomainID,
		WorkflowType:           source.WorkflowType,
		WorkflowID:             source.WorkflowID,
		RunID:                  source.RunID,
		TypeName:               source.WorkflowType,
		StartTime:              time.Unix(0, source.StartTime),
		ExecutionTime:          time.Unix(0, source.ExecutionTime),
		Memo:                   p.NewDataBlob(source.Memo, constants.EncodingType(source.Encoding)),
		TaskList:               source.TaskList,
		IsCron:                 source.IsCron,
		NumClusters:            source.NumClusters,
		ClusterAttributeScope:  source.ClusterAttributeScope,
		ClusterAttributeName:   source.ClusterAttributeName,
		SearchAttributes:       source.Attr,
		CronSchedule:           source.CronSchedule,
		ExecutionStatus:        types.WorkflowExecutionStatus(source.ExecutionStatus),
		ScheduledExecutionTime: time.Unix(0, source.ScheduledExecutionTime),
	}
	if source.UpdateTime != 0 {
		record.UpdateTime = time.Unix(0, source.UpdateTime)
	}
	if source.CloseTime != 0 {
		record.CloseTime = time.Unix(0, source.CloseTime)
		record.Status = thrift.ToWorkflowExecutionCloseStatus(&source.CloseStatus)
		record.HistoryLength = source.HistoryLength
	}

	return record
}

func (c *ESClient) getSearchResult(
	ctx context.Context,
	index string,
	request *p.InternalListWorkflowExecutionsRequest,
	matchQuery *query.MatchQuery,
	isOpen bool,
	token *ElasticVisibilityPageToken,
) (*client.Response, error) {

	// always match domain id
	boolQuery := query.NewBoolQuery().Must(query.NewMatchQuery(DomainID, request.DomainUUID))

	if matchQuery != nil {
		boolQuery = boolQuery.Must(matchQuery)
	}
	// ElasticSearch v6 is unable to precisely compare time, have to manually add resolution 1ms to time range.
	// Also has to use string instead of int64 to avoid data conversion issue,
	// 9223372036854775807 to 9223372036854776000 (long overflow)
	if request.LatestTime.UnixNano() > math.MaxInt64-oneMicroSecondInNano { // prevent latestTime overflow
		request.LatestTime = time.Unix(0, math.MaxInt64-oneMicroSecondInNano)
	}
	if request.EarliestTime.UnixNano() < math.MinInt64+oneMicroSecondInNano { // prevent earliestTime overflow
		request.EarliestTime = time.Unix(0, math.MinInt64+oneMicroSecondInNano)
	}

	var rangeQueryField string
	existClosedStatusQuery := query.NewExistsQuery(CloseStatus)
	if isOpen {
		rangeQueryField = StartTime
		boolQuery = boolQuery.MustNot(existClosedStatusQuery)
	} else {
		boolQuery = boolQuery.Must(existClosedStatusQuery)
		rangeQueryField = CloseTime
	}

	earliestTimeStr := strconv.FormatInt(request.EarliestTime.UnixNano()-oneMicroSecondInNano, 10)
	latestTimeStr := strconv.FormatInt(request.LatestTime.UnixNano()+oneMicroSecondInNano, 10)
	rangeQuery := query.NewRangeQuery(rangeQueryField).Gte(earliestTimeStr).Lte(latestTimeStr)
	boolQuery = boolQuery.Filter(rangeQuery)

	qb := query.NewBuilder()
	qb.Query(boolQuery).From(token.From).Size(request.PageSize)

	qb.Sortby(
		query.NewFieldSort(rangeQueryField).Desc(),
		query.NewFieldSort(RunID).Desc(),
	)

	if ShouldSearchAfter(token) {
		qb.SearchAfter(token.SortValue, token.TieBreaker)
	}

	body, err := qb.String()
	if err != nil {
		return nil, fmt.Errorf("getting body from query builder: %w", err)
	}

	return c.Client.Search(ctx, index, body)
}

func buildPutMappingBody(root, key, valueType string) map[string]interface{} {
	body := make(map[string]interface{})
	if len(root) != 0 {
		body["properties"] = map[string]interface{}{
			root: map[string]interface{}{
				"properties": map[string]interface{}{
					key: map[string]interface{}{
						"type": valueType,
					},
				},
			},
		}
	} else {
		body["properties"] = map[string]interface{}{
			key: map[string]interface{}{
				"type": valueType,
			},
		}
	}
	return body
}

// Helper to check if error is node unavailable error from OpenSearch
func isNodeUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "node") && strings.Contains(errMsg, "not available")
}
