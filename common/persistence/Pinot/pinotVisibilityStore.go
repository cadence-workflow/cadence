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

package pinotVisibility

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
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
	pinotPersistenceName = "pinot"
	DescendingOrder      = "DESC"
	AcendingOrder        = "ASC"
	DocID                = "DocID"
	DomainID             = "DomainID"
	WorkflowID           = "WorkflowID"
	RunID                = "RunID"
	WorkflowType         = "WorkflowType"
	CloseStatus          = "CloseStatus"
	HistoryLength        = "HistoryLength"
	TaskList             = "TaskList"
	IsCron               = "IsCron"
	NumClusters          = "NumClusters"
	ShardID              = "ShardID"
	Attr                 = "Attr"
	StartTime            = "StartTime"
	CloseTime            = "CloseTime"
	UpdateTime           = "UpdateTime"
	ExecutionTime        = "ExecutionTime"
	Encoding             = "Encoding"
	LikeStatement        = "%s LIKE '%%%s%%'"

	// used to be micro second
	oneMicroSecondInNano = int64(time.Microsecond / time.Nanosecond)
)

type (
	pinotVisibilityStore struct {
		pinotClient pnt.GenericClient
		producer    messaging.Producer
		logger      log.Logger
		config      *service.Config
	}

	visibilityMessage struct {
		DocID         string `json:"DocID,omitempty"`
		DomainID      string `json:"DomainID,omitempty"`
		WorkflowID    string `json:"WorkflowID,omitempty"`
		RunID         string `json:"RunID,omitempty"`
		WorkflowType  string `json:"WorkflowType,omitempty"`
		TaskList      string `json:"TaskList,omitempty"`
		StartTime     int64  `json:"StartTime,omitempty"`
		ExecutionTime int64  `json:"ExecutionTime,omitempty"`
		TaskID        int64  `json:"TaskID,omitempty"`
		IsCron        bool   `json:"IsCron,omitempty"`
		NumClusters   int64  `json:"NumClusters,omitempty"`
		UpdateTime    int64  `json:"UpdateTime,omitempty"` // update execution,
		ShardID       int64  `json:"ShardID,omitempty"`
		// specific to certain status
		CloseTime     int64 `json:"CloseTime,omitempty"`     // close execution
		CloseStatus   int64 `json:"CloseStatus"`             // close execution
		HistoryLength int64 `json:"HistoryLength,omitempty"` // close execution
	}
)

var _ p.VisibilityStore = (*pinotVisibilityStore)(nil)

func NewPinotVisibilityStore(
	pinotClient pnt.GenericClient,
	config *service.Config,
	producer messaging.Producer,
	logger log.Logger,
) p.VisibilityStore {
	return &pinotVisibilityStore{
		pinotClient: pinotClient,
		producer:    producer,
		logger:      logger.WithTags(tag.ComponentPinotVisibilityManager),
		config:      config,
	}
}

func (v *pinotVisibilityStore) Close() {
	return // TODO: need to double check what is close trace do. Does it close the client?
}

func (v *pinotVisibilityStore) GetName() string {
	return pinotPersistenceName
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *p.InternalRecordWorkflowExecutionStartedRequest,
) error {
	v.checkProducer()

	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.Memo.Data,
		request.Memo.GetEncoding(),
		request.IsCron,
		request.NumClusters,
		-1,                                  // represent invalid close time, means open workflow execution
		-1,                                  // represent invalid close status, means open workflow execution
		0,                                   // will not be used
		request.UpdateTimestamp.UnixMilli(), // will be updated when workflow execution updates
		int64(request.ShardID),
		request.SearchAttributes,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionClosed(ctx context.Context, request *p.InternalRecordWorkflowExecutionClosedRequest) error {
	v.checkProducer()

	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.Memo.Data,
		request.Memo.GetEncoding(),
		request.IsCron,
		request.NumClusters,
		request.CloseTimestamp.UnixMilli(),
		*thrift.FromWorkflowExecutionCloseStatus(&request.Status),
		request.HistoryLength,
		request.UpdateTimestamp.UnixMilli(),
		int64(request.ShardID),
		request.SearchAttributes,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) RecordWorkflowExecutionUninitialized(ctx context.Context, request *p.InternalRecordWorkflowExecutionUninitializedRequest) error {
	v.checkProducer()
	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		"",
		-1,
		-1,
		0,
		nil,
		"",
		false,
		0,
		-1, // represent invalid close time, means open workflow execution
		-1, // represent invalid close status, means open workflow execution
		0,
		request.UpdateTimestamp.UnixMilli(),
		request.ShardID,
		nil,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) UpsertWorkflowExecution(ctx context.Context, request *p.InternalUpsertWorkflowExecutionRequest) error {
	v.checkProducer()
	msg, err := createVisibilityMessage(
		request.DomainUUID,
		request.WorkflowID,
		request.RunID,
		request.WorkflowTypeName,
		request.TaskList,
		request.StartTimestamp.UnixMilli(),
		request.ExecutionTimestamp.UnixMilli(),
		request.TaskID,
		request.Memo.Data,
		request.Memo.GetEncoding(),
		request.IsCron,
		request.NumClusters,
		-1, // represent invalid close time, means open workflow execution
		-1, // represent invalid close status, means open workflow execution
		0,  // will not be used
		request.UpdateTimestamp.UnixMilli(),
		request.ShardID,
		request.SearchAttributes,
	)

	if err != nil {
		return err
	}

	return v.producer.Publish(ctx, msg)
}

func (v *pinotVisibilityStore) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	isRecordValid := func(rec *p.InternalVisibilityWorkflowExecutionInfo) bool {
		return !request.EarliestTime.After(rec.StartTime) && !rec.StartTime.After(request.LatestTime)
	}
	query := getListWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request, false)

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

	query := getListWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request, true)

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

	query := getListWorkflowExecutionsByTypeQuery(v.pinotClient.GetTableName(), request, false)

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

	query := getListWorkflowExecutionsByTypeQuery(v.pinotClient.GetTableName(), request, true)

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

	query := getListWorkflowExecutionsByWorkflowIDQuery(v.pinotClient.GetTableName(), request, false)

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

	query := getListWorkflowExecutionsByWorkflowIDQuery(v.pinotClient.GetTableName(), request, true)

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

	query := getListWorkflowExecutionsByStatusQuery(v.pinotClient.GetTableName(), request)

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

	query := v.getListWorkflowExecutionsByQueryQuery(v.pinotClient.GetTableName(), request, v.config.ValidSearchAttributes())
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

	query := v.getListWorkflowExecutionsByQueryQuery(v.pinotClient.GetTableName(), request, v.config.ValidSearchAttributes())

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
	query := v.getCountWorkflowExecutionsQuery(v.pinotClient.GetTableName(), request, v.config.ValidSearchAttributes())

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

func (v *pinotVisibilityStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {
	return &types.BadRequestError{Message: "Operation is not supported. Pinot doesn't support this operation so far."}
}

func (v *pinotVisibilityStore) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {
	// temporary: not implemented, only implemented for ES
	return &types.BadRequestError{Message: "Operation is not supported. Pinot doesn't support this operation so far."}
}

func (v *pinotVisibilityStore) checkProducer() {
	if v.producer == nil {
		// must be bug, check history setup
		panic("message producer is nil")
	}
}

func createVisibilityMessage(
	// common parameters
	domainID string,
	wid,
	rid string,
	workflowTypeName string,
	taskList string,
	startTimeUnixNano int64,
	executionTimeUnixNano int64,
	taskID int64,
	memo []byte,
	encoding common.EncodingType,
	isCron bool,
	numClusters int16,
	// specific to certain status
	closeTimeUnixNano int64, // close execution
	closeStatus workflow.WorkflowExecutionCloseStatus, // close execution
	historyLength int64, // close execution
	updateTimeUnixNano int64, // update execution,
	shardID int64,
	rawSearchAttributes map[string][]byte,
) (*indexer.PinotMessage, error) {
	m := make(map[string]interface{})
	//loop through all input parameters
	m[DocID] = wid + "-" + rid
	m[DomainID] = domainID
	m[WorkflowID] = wid
	m[RunID] = rid
	m[WorkflowType] = workflowTypeName
	m[TaskList] = taskList
	m[StartTime] = startTimeUnixNano
	m[ExecutionTime] = executionTimeUnixNano
	m[IsCron] = isCron
	m[NumClusters] = numClusters
	m[CloseTime] = closeTimeUnixNano
	m[CloseStatus] = int(closeStatus)
	m[HistoryLength] = historyLength
	m[UpdateTime] = updateTimeUnixNano
	m[ShardID] = shardID

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
		m[key] = val
	}

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

func (q *PinotQuery) addLimits(limit int) {
	q.limits += fmt.Sprintf("LIMIT %d\n", limit)
}

func (q *PinotQuery) addOffsetAndLimits(offset int, limit int) {
	q.limits += fmt.Sprintf("LIMIT %d, %d\n", offset, limit)
}

type PinotQueryFilter struct {
	string
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
	quotedVal := fmt.Sprintf("'%s'", val)
	f.string += fmt.Sprintf("%s = %s\n", obj, quotedVal)
}

// addQuery adds a complete query into the filter
func (f *PinotQueryFilter) addQuery(query string) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s\n", query)
}

// addGte check object is greater than or equals to val
func (f *PinotQueryFilter) addGte(obj string, val interface{}) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s >= %s\n", obj, val)
}

// addLte check object is less than val
func (f *PinotQueryFilter) addLt(obj string, val interface{}) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s < %s\n", obj, val)
}

func (f *PinotQueryFilter) addTimeRange(obj string, earliest interface{}, latest interface{}) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s BETWEEN %v AND %v\n", obj, earliest, latest)
}

func (f *PinotQueryFilter) addPartialMatch(key string, val string) {
	f.checkFirstFilter()
	f.string += fmt.Sprintf("%s\n", getPartialFormatString(key, val))
}

func getPartialFormatString(key string, val string) string {
	return fmt.Sprintf(LikeStatement, key, val)
}

func (v *pinotVisibilityStore) getCountWorkflowExecutionsQuery(tableName string, request *p.CountWorkflowExecutionsRequest, validMap map[string]interface{}) string {
	if request == nil {
		return ""
	}

	query := NewPinotCountQuery(tableName)

	// need to add Domain ID
	query.filters.addEqual(DomainID, request.DomainUUID)

	requestQuery := strings.TrimSpace(request.Query)

	// if customized query is empty, directly return
	if requestQuery == "" {
		return query.String()
	}

	requestQuery = filterPrefix(requestQuery)

	// when customized query is not empty
	if common.IsJustOrderByClause(requestQuery) {
		query.concatSorter(requestQuery)
	} else { // check if it has a complete customized query
		query = v.constructQueryWithCustomizedQuery(requestQuery, validMap, query)
	}

	return query.String()
}

func (v *pinotVisibilityStore) getListWorkflowExecutionsByQueryQuery(tableName string, request *p.ListWorkflowExecutionsByQueryRequest, validMap map[string]interface{}) string {
	if request == nil {
		return ""
	}

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		panic(fmt.Sprintf("deserialize next page token error: %s", err))
	}

	query := NewPinotQuery(tableName)

	// need to add Domain ID
	query.filters.addEqual(DomainID, request.DomainUUID)

	requestQuery := strings.TrimSpace(request.Query)

	// if customized query is empty, directly return
	if requestQuery == "" {
		query.addOffsetAndLimits(token.From, request.PageSize)
		return query.String()
	}

	requestQuery = filterPrefix(requestQuery)

	// when customized query is not empty
	if common.IsJustOrderByClause(requestQuery) {
		query.concatSorter(requestQuery)
	} else { // check if it has a complete customized query
		query = v.constructQueryWithCustomizedQuery(requestQuery, validMap, query)
	}

	// MUST HAVE! because pagination wouldn't work without order by clause!
	if query.sorters == "" {
		query.addPinotSorter("CloseTime", "DESC")
	}

	query.addOffsetAndLimits(token.From, request.PageSize)
	return query.String()
}

func filterPrefix(query string) string {
	prefix := fmt.Sprintf("`%s.", Attr)
	postfix := "`"

	query = strings.ReplaceAll(query, prefix, "")
	return strings.ReplaceAll(query, postfix, "")
}

func (v *pinotVisibilityStore) constructQueryWithCustomizedQuery(requestQuery string, validMap map[string]interface{}, query PinotQuery) PinotQuery {
	// checks every case of 'and'
	reg := regexp.MustCompile("(?i)( and )")
	queryList := reg.Split(requestQuery, -1)
	var orderBy string

	for index, element := range queryList {
		element := strings.TrimSpace(element)
		// special case: when element is the last one
		if index == len(queryList)-1 {
			element, orderBy = parseLastElement(element)
		}
		if len(element) == 0 {
			continue
		}

		if strings.Contains(strings.ToLower(element), " or ") {
			query = v.dealWithOrClause(element, query, validMap)
		} else {
			query = v.dealWithoutOrClause(element, query, validMap)
		}
	}

	if orderBy != "" {
		query.concatSorter(orderBy)
	}

	return query
}

func convertMissingFields(key string, op string, val string, element string) string {
	if val == "missing" {
		if strings.ToLower(key) == "historylength" {
			return fmt.Sprintf("%s %s %s", key, op, "0")
		}
		return fmt.Sprintf("%s %s %s", key, op, "-1")
	}
	return element
}

func (v *pinotVisibilityStore) convertRawToPinotQuery(element string, validMap map[string]interface{}) string {
	key, val, op := splitElement(element)

	// case 1: when key is a system key
	if ok, _ := isSystemKey(key); ok {
		return convertMissingFields(key, op, val, element)
	}

	// case 2: when key is valid within validMap
	if valType, ok := validMap[key]; ok {
		indexValType := common.ConvertIndexedValueTypeToInternalType(valType, log.NewNoop())

		if indexValType == types.IndexedValueTypeString {
			val = removeQuote(val)
			return getPartialFormatString(key, val)
		}
		return fmt.Sprintf("%s %s %s", key, op, val)
	}
	return ""
}

func trimElement(element string) string {
	if len(element) < 2 {
		return ""
	}

	if element[0] == '(' && element[len(element)-1] == ')' {
		element = element[1 : len(element)-1]
	}

	return element
}

func (v *pinotVisibilityStore) dealWithOrClause(element string, query PinotQuery, validMap map[string]interface{}) PinotQuery {
	element = trimElement(element)

	elementArray := strings.Split(element, " or ")

	orQuery := "("
	for index, value := range elementArray {
		orQuery += v.convertRawToPinotQuery(value, validMap)
		if index != len(elementArray)-1 {
			orQuery += " or "
		}
	}
	orQuery += ")"

	query.filters.addQuery(orQuery)
	return query
}

func (v *pinotVisibilityStore) dealWithoutOrClause(element string, query PinotQuery, validMap map[string]interface{}) PinotQuery {
	key, val, op := splitElement(element)

	// case 1: when key is a system key
	if ok, _ := isSystemKey(key); ok {
		element = convertMissingFields(key, op, val, element)
		query.filters.addQuery(element)
		return query
	}

	// case 2: when key is valid within validMap
	if valType, ok := validMap[key]; ok {
		indexValType := common.ConvertIndexedValueTypeToInternalType(valType, v.logger)

		if indexValType == types.IndexedValueTypeString {
			val = removeQuote(val)
			query.filters.addPartialMatch(key, val)
		} else {
			query.filters.addQuery(fmt.Sprintf("%s %s %s", key, op, val))
		}
	} else {
		v.logger.Error("Unregistered field!!")
	}

	return query
}

/*
Can have cases:
1. A=B
2. A<=B
3. A>=B
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
func parseLastElement(element string) (string, string) {
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
	return strings.Join(elementArray[:orderByIndex], " "), strings.Join(elementArray[orderByIndex:], " ")
}

func findLastOrderBy(list []string) int {
	for i := len(list) - 2; i >= 0; i-- {
		if strings.ToLower(list[i]) == "order" && strings.ToLower(list[i+1]) == "by" {
			return i
		}
	}
	return 0
}

func removeQuote(val string) string {
	if val[0] == '"' && val[len(val)-1] == '"' {
		val = fmt.Sprintf("%s", val[1:len(val)-1])
	} else if val[0] == '\'' && val[len(val)-1] == '\'' {
		val = fmt.Sprintf("%s", val[1:len(val)-1])
	}
	return fmt.Sprintf("%s", val)
}

// checks if a string is system key
func isSystemKey(key string) (bool, string) {
	msg := visibilityMessage{}
	values := reflect.ValueOf(msg)
	typesOf := values.Type()
	for i := 0; i < values.NumField(); i++ {
		fieldName := typesOf.Field(i).Name
		if fieldName == key {
			return true, typesOf.Field(i).Type.String()
		}
	}
	return false, "nil"
}

func getListWorkflowExecutionsQuery(tableName string, request *p.InternalListWorkflowExecutionsRequest, isClosed bool) string {
	if request == nil {
		return ""
	}

	token, err := pnt.GetNextPageToken(request.NextPageToken)
	if err != nil {
		panic(fmt.Sprintf("deserialize next page token error: %s", err))
	}

	from := token.From
	pageSize := request.PageSize

	query := NewPinotQuery(tableName)
	query.filters.addEqual(DomainID, request.DomainUUID)

	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, "0")
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, "0")
		query.filters.addEqual(CloseTime, "-1")
	}

	query.addPinotSorter(CloseTime, DescendingOrder)
	query.addPinotSorter(RunID, DescendingOrder)

	query.addOffsetAndLimits(from, pageSize)

	return query.String()
}

func getListWorkflowExecutionsByTypeQuery(tableName string, request *p.InternalListWorkflowExecutionsByTypeRequest, isClosed bool) string {
	if request == nil {
		return ""
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addEqual(WorkflowType, request.WorkflowTypeName)
	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, "0")
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, "0")
		query.filters.addEqual(CloseTime, "-1")
	}

	query.addPinotSorter(CloseTime, DescendingOrder)
	query.addPinotSorter(RunID, DescendingOrder)
	return query.String()
}

func getListWorkflowExecutionsByWorkflowIDQuery(tableName string, request *p.InternalListWorkflowExecutionsByWorkflowIDRequest, isClosed bool) string {
	if request == nil {
		return ""
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addEqual(WorkflowID, request.WorkflowID)
	earliest := request.EarliestTime.UnixMilli() - oneMicroSecondInNano
	latest := request.LatestTime.UnixMilli() + oneMicroSecondInNano

	if isClosed {
		query.filters.addTimeRange(CloseTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addGte(CloseStatus, "0")
	} else {
		query.filters.addTimeRange(StartTime, earliest, latest) //convert Unix Time to miliseconds
		query.filters.addLt(CloseStatus, "0")
		query.filters.addEqual(CloseTime, "-1")
	}

	query.addPinotSorter(CloseTime, DescendingOrder)
	query.addPinotSorter(RunID, DescendingOrder)
	return query.String()
}

func getListWorkflowExecutionsByStatusQuery(tableName string, request *p.InternalListClosedWorkflowExecutionsByStatusRequest) string {
	if request == nil {
		return ""
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)

	status := "0"
	switch request.Status.String() {
	case "COMPLETED":
		status = "0"
	case "FAILED":
		status = "1"
	case "CANCELED":
		status = "2"
	case "TERMINATED":
		status = "3"
	case "CONTINUED_AS_NEW":
		status = "4"
	case "TIMED_OUT":
		status = "5"
	}

	query.filters.addEqual(CloseStatus, status)
	query.filters.addTimeRange(CloseTime, request.EarliestTime.UnixMilli(), request.LatestTime.UnixMilli()) //convert Unix Time to miliseconds

	query.addPinotSorter(CloseTime, DescendingOrder)
	query.addPinotSorter(RunID, DescendingOrder)
	return query.String()
}

func getGetClosedWorkflowExecutionQuery(tableName string, request *p.InternalGetClosedWorkflowExecutionRequest) string {
	if request == nil {
		return ""
	}

	query := NewPinotQuery(tableName)

	query.filters.addEqual(DomainID, request.DomainUUID)
	query.filters.addGte(CloseStatus, "0")
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
