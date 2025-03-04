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

package pinotvisibility

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/uber/cadence/.gen/go/indexer"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	p "github.com/uber/cadence/common/persistence"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

const (
	DescendingOrder = "DESC"
	AscendingOrder  = "ASC"
	DomainID        = "DomainID"
	WorkflowID      = "WorkflowID"
	RunID           = "RunID"
	WorkflowType    = "WorkflowType"
	CloseStatus     = "CloseStatus"
	HistoryLength   = "HistoryLength"
	TaskList        = "TaskList"
	IsCron          = "IsCron"
	NumClusters     = "NumClusters"
	ShardID         = "ShardID"
	Attr            = "Attr"
	StartTime       = "StartTime"
	CloseTime       = "CloseTime"
	UpdateTime      = "UpdateTime"
	ExecutionTime   = "ExecutionTime"
	IsDeleted       = "IsDeleted"   // used for Pinot deletion/rolling upsert only, not visible to user
	EventTimeMs     = "EventTimeMs" // used for Pinot deletion/rolling upsert only, not visible to user
	Memo            = "Memo"

	// used to be micro second
	oneMicroSecondInNano = int64(time.Microsecond / time.Nanosecond)
)

type (
	pinotVisibilityStore struct {
		pinotClient         pnt.GenericClient
		producer            messaging.Producer
		logger              log.Logger
		config              *service.Config
		pinotQueryValidator *pnt.VisibilityQueryValidator
	}
)

var _ p.VisibilityStore = (*pinotVisibilityStore)(nil)

func NewPinotVisibilityStore(
	pinotClient pnt.GenericClient,
	config *service.Config,
	producer messaging.Producer,
	logger log.Logger,
) p.VisibilityStore {
	if producer == nil {
		// must be bug, check history setup
		logger.Fatal("message producer is nil")
	}
	return &pinotVisibilityStore{
		pinotClient:         pinotClient,
		producer:            producer,
		logger:              logger.WithTags(tag.ComponentPinotVisibilityManager),
		config:              config,
		pinotQueryValidator: pnt.NewPinotQueryValidator(config.ValidSearchAttributes, config.PinotOptimizedQueryColumns),
	}
}

func (v *pinotVisibilityStore) Close() {
	// Not needed for pinot, just keep for visibility store interface
}

func (v *pinotVisibilityStore) GetName() string {
	return common.PinotPersistenceName
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *p.InternalRecordWorkflowExecutionStartedRequest,
) error {

	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.IsCron,
		request.NumClusters,
		-1, // represent invalid close time, means open workflow execution
		-1, // represent invalid close status, means open workflow execution
		0,  // will be updated when workflow execution updates
		request.UpdateTimestamp.UnixMilli(),
		int64(request.ShardID),
		request.SearchAttributes,
		false,
		request.Memo,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionClosed(ctx context.Context, request *p.InternalRecordWorkflowExecutionClosedRequest) error {

	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.IsCron,
		request.NumClusters,
		request.CloseTimestamp.UnixMilli(),
		*thrift.FromWorkflowExecutionCloseStatus(&request.Status),
		request.HistoryLength,
		request.UpdateTimestamp.UnixMilli(),
		int64(request.ShardID),
		request.SearchAttributes,
		false,
		request.Memo,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionUninitialized(ctx context.Context, request *p.InternalRecordWorkflowExecutionUninitializedRequest) error {

	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		"",
		-1,
		-1,
		0,
		false,
		0,
		-1, // represent invalid close time, means open workflow execution
		-1, // represent invalid close status, means open workflow execution
		0,  // will be updated when workflow execution updates
		request.UpdateTimestamp.UnixMilli(),
		request.ShardID,
		nil,
		false,
		nil,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) UpsertWorkflowExecution(ctx context.Context, request *p.InternalUpsertWorkflowExecutionRequest) error {
	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.IsCron,
		request.NumClusters,
		-1, // represent invalid close time, means open workflow execution
		-1, // represent invalid close status, means open workflow execution
		0,  // will not be used
		request.UpdateTimestamp.UnixMilli(),
		request.ShardID,
		request.SearchAttributes,
		false,
		request.Memo,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {

	msg, err := createDeleteVisibilityMessage(
		request.DomainID,
		request.WorkflowID,
		request.RunID,
		true,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {
	// verify if it is uninitialized workflow execution record
	// if it is, then call the existing delete method to delete
	query := fmt.Sprintf("StartTime = missing and DomainID = '%s' and RunID = '%s'", request.DomainID, request.RunID)

	queryRequest := &p.CountWorkflowExecutionsRequest{
		Domain: request.Domain,
		Query:  query,
	}
	resp, err := v.CountWorkflowExecutions(ctx, queryRequest)
	if err != nil {
		return err
	}
	if resp.Count > 0 {
		if err = v.DeleteWorkflowExecution(ctx, request); err != nil {
			return err
		}
	}
	return nil
}

func (v *pinotVisibilityStore) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.StartTime) && !rec.StartTime.After(request.LatestTime)
	}
	query, err := getListWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request, false)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     request,
	}
	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.CloseTime) && !rec.CloseTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request, true)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     request,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListOpenWorkflowExecutionsByType(ctx context.Context, request *p.InternalListWorkflowExecutionsByTypeRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.StartTime) && !rec.StartTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsByTypeQuery(v.pinotClient.GetTableName(), request, false)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions by type query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     &request.InternalListWorkflowExecutionsRequest,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListClosedWorkflowExecutionsByType(ctx context.Context, request *p.InternalListWorkflowExecutionsByTypeRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.CloseTime) && !rec.CloseTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsByTypeQuery(v.pinotClient.GetTableName(), request, true)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions by type query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     &request.InternalListWorkflowExecutionsRequest,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListOpenWorkflowExecutionsByWorkflowID(ctx context.Context, request *p.InternalListWorkflowExecutionsByWorkflowIDRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.StartTime) && !rec.StartTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsByWorkflowIDQuery(v.pinotClient.GetTableName(), request, false)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions by workflowID query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     &request.InternalListWorkflowExecutionsRequest,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListClosedWorkflowExecutionsByWorkflowID(ctx context.Context, request *p.InternalListWorkflowExecutionsByWorkflowIDRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.CloseTime) && !rec.CloseTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsByWorkflowIDQuery(v.pinotClient.GetTableName(), request, true)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions by workflowID query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     &request.InternalListWorkflowExecutionsRequest,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ListClosedWorkflowExecutionsByStatus(ctx context.Context, request *p.InternalListClosedWorkflowExecutionsByStatusRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.CloseTime) && !rec.CloseTime.After(request.LatestTime)
	}

	query, err := getListWorkflowExecutionsByStatusQuery(v.pinotClient.GetTableName(), request)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions by status query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          isRecordValid,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest:     &request.InternalListWorkflowExecutionsRequest,
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) GetClosedWorkflowExecution(ctx context.Context, request *p.InternalGetClosedWorkflowExecutionRequest) (*p.InternalGetClosedWorkflowExecutionResponse, error) {
	query := getGetClosedWorkflowExecutionQuery(v.pinotClient.GetTableName(), request)

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          nil,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest: &p.InternalListWorkflowExecutionsRequest{ // create a new request to avoid nil pointer exceptions
			DomainUUID:    request.DomainUUID,
			Domain:        request.Domain,
			EarliestTime:  time.Time{},
			LatestTime:    time.Time{},
			PageSize:      1,
			NextPageToken: nil,
		},
	}

	resp, err := v.pinotClient.Search(req)

	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("Pinot GetClosedWorkflowExecution failed, %v", err),
		}
	}

	return &p.InternalGetClosedWorkflowExecutionResponse{
		Execution: resp.Executions[0],
	}, nil
}

func (v *pinotVisibilityStore) ListWorkflowExecutions(ctx context.Context, request *p.ListWorkflowExecutionsByQueryRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	checkPageSize(request)

	query, err := v.getListWorkflowExecutionsByQueryQuery(v.pinotClient.GetTableName(), request)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build list workflow executions query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          nil,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest: &p.InternalListWorkflowExecutionsRequest{
			DomainUUID:    request.DomainUUID,
			Domain:        request.Domain,
			EarliestTime:  time.Time{},
			LatestTime:    time.Time{},
			NextPageToken: request.NextPageToken,
			PageSize:      request.PageSize,
		},
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) ScanWorkflowExecutions(ctx context.Context, request *p.ListWorkflowExecutionsByQueryRequest) (*p.InternalListWorkflowExecutionsResponse, error) {
	checkPageSize(request)

	query, err := v.getListWorkflowExecutionsByQueryQuery(v.pinotClient.GetTableName(), request)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build scan workflow executions query %v", err))
		return nil, err
	}

	req := &pnt.SearchRequest{
		Query:           query,
		IsOpen:          true,
		Filter:          nil,
		MaxResultWindow: v.config.ESIndexMaxResultWindow(),
		ListRequest: &p.InternalListWorkflowExecutionsRequest{
			DomainUUID:    request.DomainUUID,
			Domain:        request.Domain,
			EarliestTime:  time.Time{},
			LatestTime:    time.Time{},
			NextPageToken: request.NextPageToken,
			PageSize:      request.PageSize,
		},
	}

	return v.pinotClient.Search(req)
}

func (v *pinotVisibilityStore) CountWorkflowExecutions(ctx context.Context, request *p.CountWorkflowExecutionsRequest) (*p.CountWorkflowExecutionsResponse, error) {
	query, err := v.getCountWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request)
	if err != nil {
		v.logger.Error(fmt.Sprintf("failed to build count workflow executions query %v", err))
		return nil, err
	}

	resp, err := v.pinotClient.CountByQuery(query)
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("CountClosedWorkflowExecutions failed, %v", err),
		}
	}

	return &p.CountWorkflowExecutionsResponse{
		Count: resp,
	}, nil
}

// a new function to create visibility message for deletion
// don't use the other function and provide some nil values because it may cause nil pointer exceptions
func createDeleteVisibilityMessage(domainID string,
	wid,
	rid string,
	isDeleted bool,
) (*indexer.PinotMessage, error) {
	m := make(map[string]interface{})
	m[DomainID] = domainID
	m[WorkflowID] = wid
	m[RunID] = rid
	m[IsDeleted] = isDeleted
	m[EventTimeMs] = time.Now().UnixMilli()
	serializedMsg, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	msg := &indexer.PinotMessage{
		WorkflowID: common.StringPtr(wid),
		Payload:    serializedMsg,
	}
	return msg, nil
}

func createVisibilityMessage(
	// common parameters
	domainID string,
	wid,
	rid string,
	workflowTypeName string,
	taskList string,
	startTimeUnixMilli int64,
	executionTimeUnixMilli int64,
	taskID int64,
	isCron bool,
	numClusters int16,
	// specific to certain status
	closeTimeUnixMilli int64, // close execution
	closeStatus workflow.WorkflowExecutionCloseStatus, // close execution
	historyLength int64, // close execution
	updateTimeUnixMilli int64, // update execution,
	shardID int64,
	rawSearchAttributes map[string][]byte,
	isDeleted bool,
	memo *p.DataBlob,
) (*indexer.PinotMessage, error) {
	m := make(map[string]interface{})
	// loop through all input parameters
	m[DomainID] = domainID
	m[WorkflowID] = wid
	m[RunID] = rid
	m[WorkflowType] = workflowTypeName
	m[TaskList] = taskList
	m[StartTime] = startTimeUnixMilli
	m[ExecutionTime] = executionTimeUnixMilli
	m[IsCron] = isCron
	m[NumClusters] = numClusters
	m[CloseTime] = closeTimeUnixMilli
	m[CloseStatus] = int(closeStatus)
	m[HistoryLength] = historyLength
	m[UpdateTime] = updateTimeUnixMilli
	m[ShardID] = shardID
	m[IsDeleted] = isDeleted
	m[EventTimeMs] = updateTimeUnixMilli // same as update time when record is upserted, could not use updateTime directly since this will be modified by Pinot

	SearchAttributes := make(map[string]interface{})
	var err error
	for key, value := range rawSearchAttributes {
		value, err = isTimeStruct(value)
		if err != nil {
			return nil, err
		}

		var val interface{}
		err = json.Unmarshal(value, &val)
		if err != nil {
			return nil, err
		}
		SearchAttributes[key] = val
	}

	if memo != nil && len(memo.Data) > 0 {
		// add memo into search attr
		marshalMemo, err := json.Marshal(memo)
		if err != nil {
			return nil, err
		}
		// string(marshalMemo) is a must-do step,
		// to give it a type so that it can be converted to a string again
		// and unmarshaled correctly in the reading side
		// Also, it doesn't matter to store in Pinot as a pure string since it's not used for search
		SearchAttributes[Memo] = string(marshalMemo)
	}

	m[Attr] = SearchAttributes
	serializedMsg, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	msg := &indexer.PinotMessage{
		WorkflowID: common.StringPtr(wid),
		Payload:    serializedMsg,
	}
	return msg, nil

}

// check if value is time.Time type
// if it is, convert it to unixMilli
// if it isn't time, return the original value
func isTimeStruct(value []byte) ([]byte, error) {
	var time time.Time
	err := json.Unmarshal(value, &time)
	if err == nil {
		unixTime := time.UnixMilli()
		value, err = json.Marshal(unixTime)
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}

/****************************** Request Translator ******************************/

type PinotQuery struct {
	query   string
	filters PinotQueryFilter
	sorters string
	limits  string
}

type PinotQueryFilter struct {
	string
}

func NewPinotQuery(tableName string) PinotQuery {
	return PinotQuery{
		query:   fmt.Sprintf("SELECT *\nFROM %s\n", tableName),
		filters: PinotQueryFilter{},
		sorters: "",
		limits:  "",
	}
}

func NewPinotCountQuery(tableName string) PinotQuery {
	return PinotQuery{
		query:   fmt.Sprintf("SELECT COUNT(*)\nFROM %s\n", tableName),
		filters: PinotQueryFilter{},
		sorters: "",
		limits:  "",
	}
}

func (q *PinotQuery) String() string {
	return fmt.Sprintf("%s%s%s%s", q.query, q.filters.string, q.sorters, q.limits)
}

func (q *PinotQuery) concatSorter(sorter string) {
	q.sorters += sorter + "\n"
}

func (q *PinotQuery) addPinotSorter(orderBy string, order string) {
	if q.sorters == "" {
		q.sorters = "Order BY "
	} else {
		q.sorters += ", "
	}
	q.sorters += fmt.Sprintf("%s %s\n", orderBy, order)
}

func (q *PinotQuery) addOffsetAndLimits(offset int, limit int) {
	q.limits += fmt.Sprintf("LIMIT %d, %d\n", offset, limit)
}

func (f *PinotQueryFilter) checkFirstFilter() {
	if f.string == "" {
		f.string = "WHERE "
	} else {
		f.string += "AND "
	}
}

func (f *PinotQueryFilter) addEqual(obj string, val interface{}) {
	f.checkFirstFilter()
	if _, ok := val.(string); ok {
		val = fmt.Sprintf("'%s'", val)
	} else {
		val = fmt.Sprintf("%v", val)
	}

	quotedVal := fmt.Sprintf("%s", val)
	f.string += fmt.Sprintf("%s = %s\n", obj, quotedVal)
}

// addQuery adds a complete query into the filter
func (f *PinotQueryFilter) addQuery(query string) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s\n", query)
}

// addGte check object is greater than or equals to val
func (f *PinotQueryFilter) addGte(obj string, val int) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s >= %s\n", obj, fmt.Sprintf("%v", val))
}

// addLte check object is less than val
func (f *PinotQueryFilter) addLt(obj string, val interface{}) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s < %s\n", obj, fmt.Sprintf("%v", val))
}

func (f *PinotQueryFilter) addTimeRange(obj string, earliest interface{}, latest interface{}) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s BETWEEN %v AND %v\n", obj, earliest, latest)
}

func (v *pinotVisibilityStore) getCountWorkflowExecutionsQuery(tableName string, request *p.CountWorkflowExecutionsRequest) (string, error) {
	if request == nil {
		return "", nil
	}

	query := NewPinotCountQuery(tableName)

	// need to add Domain ID
	query.filters.addEqual(DomainID, request.DomainUUID)

	requestQuery := strings.TrimSpace(request.Query)

	// if customized query is empty, directly return
	if requestQuery == "" {
		return query.String(), nil
	}

	requestQuery = filterPrefix(requestQuery)

	comparExpr, _ := parseOrderBy(requestQuery)
	comparExpr, err := v.pinotQueryValidator.ValidateQuery(comparExpr)
	if err != nil {
		return "", &types.BadRequestError{Message: fmt.Sprintf("pinot query validator error: %s, query: %s", err.Error(), request.Query)}
	}

	comparExpr = filterPrefix(comparExpr)
	if comparExpr != "" {
		query.filters.addQuery(comparExpr)
	}

	return query.String(), nil
}

func (v *pinotVisibilityStore) getListWorkflowExecutionsByQueryQuery(tableName string, request *p.ListWorkflowExecutionsByQueryRequest) (string, error) {
	if request == nil {
		return "", nil
	}

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		return "", fmt.Errorf("next page token: %w", err)
	}

	query := NewPinotQuery(tableName)

	// need to add Domain ID
	query.filters.addEqual(DomainID, request.DomainUUID)

	requestQuery := strings.TrimSpace(request.Query)

	// if customized query is empty, directly return
	if requestQuery == "" {
		query.addOffsetAndLimits(token.From, request.PageSize)
		return query.String(), nil
	}

	requestQuery = filterPrefix(requestQuery)
	if common.IsJustOrderByClause(requestQuery) {
		query.concatSorter(requestQuery)
		query.addOffsetAndLimits(token.From, request.PageSize)
		return query.String(), nil
	}

	comparExpr, orderBy := parseOrderBy(requestQuery)
	comparExpr, err = v.pinotQueryValidator.ValidateQuery(comparExpr)
	if err != nil {
		return "", &types.BadRequestError{Message: fmt.Sprintf("pinot query validator error: %s, query: %s", err.Error(), request.Query)}
	}

	comparExpr = filterPrefix(comparExpr)
	if comparExpr != "" {
		query.filters.addQuery(comparExpr)
	}
	if orderBy != "" {
		query.concatSorter(orderBy)
	}

	// MUST HAVE! because pagination wouldn't work without order by clause!
	if query.sorters == "" {
		query.addPinotSorter(StartTime, "DESC")
	}

	query.addOffsetAndLimits(token.From, request.PageSize)
	return query.String(), nil
}

func filterPrefix(query string) string {
	prefix := fmt.Sprintf("`%s.", Attr)
	postfix := "`"

	query = strings.ReplaceAll(query, prefix, "")
	return strings.ReplaceAll(query, postfix, "")
}

/*
Can have cases:
1. A = B
2. A < B
3. A > B
4. A <= B
5. A >= B
*/
func splitElement(element string) (string, string, string) {
	if element == "" {
		return "", "", ""
	}

	listLE := strings.Split(element, "<=")
	listGE := strings.Split(element, ">=")
	listE := strings.Split(element, "=")
	listL := strings.Split(element, "<")
	listG := strings.Split(element, ">")

	if len(listLE) > 1 {
		return strings.TrimSpace(listLE[0]), strings.TrimSpace(listLE[1]), "<="
	}

	if len(listGE) > 1 {
		return strings.TrimSpace(listGE[0]), strings.TrimSpace(listGE[1]), ">="
	}

	if len(listE) > 1 {
		return strings.TrimSpace(listE[0]), strings.TrimSpace(listE[1]), "="
	}

	if len(listL) > 1 {
		return strings.TrimSpace(listL[0]), strings.TrimSpace(listL[1]), "<"
	}

	if len(listG) > 1 {
		return strings.TrimSpace(listG[0]), strings.TrimSpace(listG[1]), ">"
	}

	return "", "", ""
}

/*
Order by XXX DESC
-> if startWith("Order by") -> return "", element

CustomizedString = 'cannot be used in order by'
-> if last character is ‘ or " -> return element, ""

CustomizedInt = 1 (without order by clause)
-> if !contains("Order by") -> return element, ""

CustomizedString = 'cannot be used in order by' Order by XXX DESC
-> Find the index x of last appearance of "order by" -> return element[0, x], element[x, len]

CustomizedInt = 1 Order by XXX DESC
-> Find the index x of last appearance of "order by" -> return element[0, x], element[x, len]
*/
func parseOrderBy(element string) (string, string) {
	// case 1: when order by query also passed in
	if common.IsJustOrderByClause(element) {
		return "", element
	}

	// case 2: when last element is a string
	if element[len(element)-1] == '\'' || element[len(element)-1] == '"' {
		return element, ""
	}

	// case 3: when last element doesn't contain "order by"
	if !strings.Contains(strings.ToLower(element), "order by") {
		return element, ""
	}

	// case 4: general case
	elementArray := strings.Split(element, " ")
	orderByIndex := findLastOrderBy(elementArray) // find the last appearance of "order by" is the answer
	if orderByIndex == 0 {
		return element, ""
	}
	return strings.Join(elementArray[:orderByIndex], " "), strings.Join(elementArray[orderByIndex:], " ")
}

func findLastOrderBy(list []string) int {
	for i := len(list) - 2; i >= 0; i-- {
		if strings.Contains(list[i], "\"") || strings.Contains(list[i], "'") {
			return 0 // means order by is inside a string
		}

		if strings.ToLower(list[i]) == "order" && strings.ToLower(list[i+1]) == "by" {
			return i
		}
	}
	return 0
}

func getListWorkflowExecutionsQuery(tableName string, request *p.InternalListWorkflowExecutionsRequest, isClosed bool) (string, error) {
	if request == nil {
		return "", nil
	}

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		return "", fmt.Errorf("next page token: %w", err)
	}

	from := token.From
	pageSize := request.PageSize

	query := NewPinotQuery(tableName)
	query.filters.addEqual(DomainID, request.DomainUUID)

	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, 0)
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, 0)
		query.filters.addEqual(CloseTime, -1)
	}

	query.addPinotSorter(StartTime, DescendingOrder)
	query.addOffsetAndLimits(from, pageSize)

	return query.String(), nil
}

func getListWorkflowExecutionsByTypeQuery(tableName string, request *p.InternalListWorkflowExecutionsByTypeRequest, isClosed bool) (string, error) {
	if request == nil {
		return "", nil
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addEqual(WorkflowType, request.WorkflowTypeName)
	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, 0)
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, 0)
		query.filters.addEqual(CloseTime, -1)
	}

	query.addPinotSorter(StartTime, DescendingOrder)

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		return "", fmt.Errorf("next page token: %w", err)
	}

	from := token.From
	pageSize := request.PageSize
	query.addOffsetAndLimits(from, pageSize)

	return query.String(), nil
}

func getListWorkflowExecutionsByWorkflowIDQuery(tableName string, request *p.InternalListWorkflowExecutionsByWorkflowIDRequest, isClosed bool) (string, error) {
	if request == nil {
		return "", nil
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addEqual(WorkflowID, request.WorkflowID)
	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, 0)
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) // convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, 0)
		query.filters.addEqual(CloseTime, -1)
	}

	query.addPinotSorter(StartTime, DescendingOrder)

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		return "", fmt.Errorf("next page token: %w", err)
	}

	from := token.From
	pageSize := request.PageSize
	query.addOffsetAndLimits(from, pageSize)

	return query.String(), nil
}

func getListWorkflowExecutionsByStatusQuery(tableName string, request *p.InternalListClosedWorkflowExecutionsByStatusRequest) (string, error) {
	if request == nil {
		return "", nil
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)

	status := 0
	switch request.Status.String() {
	case "COMPLETED":
		status = 0
	case "FAILED":
		status = 1
	case "CANCELED":
		status = 2
	case "TERMINATED":
		status = 3
	case "CONTINUED_AS_NEW":
		status = 4
	case "TIMED_OUT":
		status = 5
	}

	query.filters.addEqual(CloseStatus, status)
	query.filters.addTimeRange(CloseTime, request.EarliestTime.UnixMilli(), request.LatestTime.UnixMilli()) // convert Unix Time to miliseconds

	query.addPinotSorter(StartTime, DescendingOrder)

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		return "", fmt.Errorf("next page token: %w", err)
	}

	from := token.From
	pageSize := request.PageSize
	query.addOffsetAndLimits(from, pageSize)

	return query.String(), nil
}

func getGetClosedWorkflowExecutionQuery(tableName string, request *p.InternalGetClosedWorkflowExecutionRequest) string {
	if request == nil {
		return ""
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addGte(CloseStatus, 0)
	query.filters.addEqual(WorkflowID, request.Execution.GetWorkflowID())

	rid := request.Execution.GetRunID()
	if rid != "" {
		query.filters.addEqual(RunID, rid)
	}

	return query.String()
}

func checkPageSize(request *p.ListWorkflowExecutionsByQueryRequest) {
	if request.PageSize == 0 {
		request.PageSize = 1000
	}
}
