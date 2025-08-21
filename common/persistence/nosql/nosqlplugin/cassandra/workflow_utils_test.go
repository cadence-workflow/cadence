// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package cassandra

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

var FixedTime = time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)

// fakeSession is fake implementation of gocql.Session
type fakeSession struct {
	// inputs
	iter                      *fakeIter
	mapExecuteBatchCASApplied bool
	mapExecuteBatchCASPrev    map[string]any
	mapExecuteBatchCASErr     error
	query                     gocql.Query

	// outputs
	batches []*fakeBatch
	queries []string
}

func (s *fakeSession) Query(queryTmpl string, args ...interface{}) gocql.Query {
	s.queries = append(s.queries, sanitizedRenderedQuery(queryTmpl, args...))
	return s.query
}

func (s *fakeSession) NewBatch(gocql.BatchType) gocql.Batch {
	b := &fakeBatch{}
	s.batches = append(s.batches, b)
	return b
}

func (s *fakeSession) ExecuteBatch(gocql.Batch) error {
	return nil
}

func (s *fakeSession) MapExecuteBatchCAS(batch gocql.Batch, prev map[string]interface{}) (bool, gocql.Iter, error) {
	for k, v := range s.mapExecuteBatchCASPrev {
		prev[k] = v
	}
	return s.mapExecuteBatchCASApplied, s.iter, s.mapExecuteBatchCASErr
}

// fakeBatch is fake implementation of gocql.Batch
type fakeBatch struct {
	// outputs
	queries []string
}

// Query is fake implementation of gocql.Batch.Query
func (b *fakeBatch) Query(queryTmpl string, args ...interface{}) {
	b.queries = append(b.queries, sanitizedRenderedQuery(queryTmpl, args...))
}

// WithContext is fake implementation of gocql.Batch.WithContext
func (b *fakeBatch) WithContext(context.Context) gocql.Batch {
	return b
}

// WithTimestamp is fake implementation of gocql.Batch.WithTimestamp
func (b *fakeBatch) WithTimestamp(int64) gocql.Batch {
	return b
}

// Consistency is fake implementation of gocql.Batch.Consistency
func (b *fakeBatch) Consistency(gocql.Consistency) gocql.Batch {
	return b
}

// fakeQuery is fake implementation of gocql.Query
func (s *fakeSession) Close() {
}

func sanitizedRenderedQuery(queryTmpl string, args ...interface{}) string {
	argsSanitized := make([]interface{}, len(args))
	for i, arg := range args {
		// use values instead of pointer so that we can compare them in tests
		if reflect.ValueOf(arg).Kind() == reflect.Ptr && !reflect.ValueOf(arg).IsZero() {
			argsSanitized[i] = reflect.ValueOf(arg).Elem().Interface()
		} else {
			argsSanitized[i] = arg
		}

		if t, ok := argsSanitized[i].(time.Time); ok {
			// use fixed time format to avoid flakiness
			argsSanitized[i] = t.UTC().Format(time.RFC3339)
		}

	}
	queryTmpl = strings.ReplaceAll(queryTmpl, "?", "%v")
	return fmt.Sprintf(queryTmpl, argsSanitized...)
}

// fakeIter is fake implementation of gocql.Iter
type fakeIter struct {
	// input parametrs
	mapScanInputs []map[string]interface{}
	scanInputs    [][]interface{}
	pageState     []byte
	closeErr      error

	// output parameters
	mapScanCalls int
	scanCalls    int
	closed       bool
}

// Scan is fake implementation of gocql.Iter.Scan
func (i *fakeIter) Scan(outArgs ...interface{}) bool {
	if i.scanCalls >= len(i.scanInputs) {
		return false
	}

	for j, v := range i.scanInputs[i.scanCalls] {
		if len(outArgs) <= j {
			panic(fmt.Sprintf("outArgs length: %d is less than expected: %d", len(outArgs), len(i.scanInputs[i.scanCalls])))
		}

		if v == nil {
			continue
		}

		dst := outArgs[j]
		dstPtrValue := reflect.ValueOf(dst)
		dstValue := reflect.Indirect(dstPtrValue)

		func() {
			defer func() {
				if r := recover(); r != nil {
					panic(fmt.Sprintf("failed to set %dth value: %v to %v, inner panic: %s", j, v, dst, r))
				}
			}()
			dstValue.Set(reflect.ValueOf(v))
		}()
	}

	i.scanCalls++
	return true
}

// MapScan is fake implementation of gocql.Iter.MapScan
func (i *fakeIter) MapScan(res map[string]interface{}) bool {
	if i.mapScanCalls >= len(i.mapScanInputs) {
		return false
	}

	for k, v := range i.mapScanInputs[i.mapScanCalls] {
		res[k] = v
	}
	i.mapScanCalls++
	return true
}

// PageState is fake implementation of gocql.Iter.PageState
func (i *fakeIter) PageState() []byte {
	return i.pageState
}

// Close is fake implementation of gocql.Iter.Close
func (i *fakeIter) Close() error {
	i.closed = true
	return i.closeErr
}

type fakeUUID struct {
	uuid string
}

func (u *fakeUUID) String() string {
	return u.uuid
}

var _ (gocql.Query) = &fakeQuery{}

type fakeQuery struct {
	iter              *fakeIter
	mapScan           map[string]interface{}
	err               error
	scanCASApplied    bool
	mapScanCASApplied bool
}

func (q *fakeQuery) Exec() error {
	return q.err
}

func (q *fakeQuery) Scan(...interface{}) error {
	return q.err
}

func (q *fakeQuery) ScanCAS(...interface{}) (bool, error) {
	return q.scanCASApplied, q.err
}

func (q *fakeQuery) MapScan(res map[string]interface{}) error {
	for k, v := range q.mapScan {
		res[k] = v
	}
	return q.err
}

func (q *fakeQuery) MapScanCAS(map[string]interface{}) (bool, error) {
	return q.mapScanCASApplied, q.err
}

func (q *fakeQuery) Iter() gocql.Iter {
	return q.iter
}

func (q *fakeQuery) PageSize(int) gocql.Query {
	return q
}

func (q *fakeQuery) PageState([]byte) gocql.Query {
	return q
}

func (q *fakeQuery) WithContext(context.Context) gocql.Query {
	return q
}

func (q *fakeQuery) WithTimestamp(int64) gocql.Query {
	return q
}

func (q *fakeQuery) Consistency(gocql.Consistency) gocql.Query {
	return q
}

func (q *fakeQuery) Bind(...interface{}) gocql.Query {
	return q
}

func TestExecuteCreateWorkflowBatchTransaction(t *testing.T) {
	tests := []struct {
		// fake setup parameters
		desc    string
		session *fakeSession

		// executeCreateWorkflowBatchTransaction args
		batch        *fakeBatch
		currentWFReq *nosqlplugin.CurrentWorkflowWriteRequest
		execution    *nosqlplugin.WorkflowExecutionRequest
		shardCond    *nosqlplugin.ShardCondition

		// expectations
		wantErr error
	}{
		{
			desc:  "applied",
			batch: &fakeBatch{},
			session: &fakeSession{
				mapExecuteBatchCASApplied: true,
				iter:                      &fakeIter{},
			},
		},
		{
			desc:  "CAS error",
			batch: &fakeBatch{},
			session: &fakeSession{
				mapExecuteBatchCASErr: fmt.Errorf("db operation failed for some reason"),
				iter:                  &fakeIter{},
			},
			wantErr: fmt.Errorf("db operation failed for some reason"),
		},
		{
			desc: "shard range id mismatch",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":     rowTypeShard,
					"run_id":   uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
					"range_id": int64(200),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				ShardRangeIDNotMatch: common.Int64Ptr(200),
			},
		},
		{
			desc: "workflow request already exists",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":   rowTypeWorkflowRequestCancel,
					"run_id": uuid.Parse("4b8045c8-7b45-41e0-bf03-1f0d166b818d"),
				},
				query: &fakeQuery{
					mapScan: map[string]interface{}{
						"current_run_id": uuid.Parse("6b844fb4-c18a-4979-a2d3-731ebdd1db08"),
					},
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			execution: &nosqlplugin.WorkflowExecutionRequest{
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					DomainID:   "fd88863f-bb32-4daa-8878-49e08b91545e",
					WorkflowID: "wfid",
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				ShardID: 1,
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				DuplicateRequest: &nosqlplugin.DuplicateRequest{
					RequestType: persistence.WorkflowRequestTypeCancel,
					RunID:       "6b844fb4-c18a-4979-a2d3-731ebdd1db08",
				},
			},
		},
		{
			desc: "workflow request already exists - but failed to get latest request",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":   rowTypeWorkflowRequestSignal,
					"run_id": uuid.Parse("4b8045c8-7b45-41e0-bf03-1f0d166b818d"),
				},
				query: &fakeQuery{
					mapScan: map[string]interface{}{
						"current_run_id": uuid.Parse("6b844fb4-c18a-4979-a2d3-731ebdd1db08"),
					},
					err: fmt.Errorf("failed to get latest request"),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			execution: &nosqlplugin.WorkflowExecutionRequest{
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					DomainID:   "fd88863f-bb32-4daa-8878-49e08b91545e",
					WorkflowID: "wfid",
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				ShardID: 1,
				RangeID: 100,
			},
			wantErr: fmt.Errorf("failed to get latest request"),
		},
		{
			desc: "current execution already exists and write mode is CurrentWorkflowWriteModeInsert",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse(permanentRunID),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
					"execution": map[string]any{
						"run_id": uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
						"state":  1,
					},
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeInsert,
				Row: nosqlplugin.CurrentWorkflowRow{
					WorkflowID: "test-workflow-id",
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
					RunID:            "bda9cd9c-32fb-4267-b120-346e5351fc46",
					State:            1,
					LastWriteVersion: 3,
					OtherInfo:        "Workflow execution already running. WorkflowId: test-workflow-id, RunId: bda9cd9c-32fb-4267-b120-346e5351fc46",
				},
			},
		},
		{
			desc: "creation condition failed by mismatch runID",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse(permanentRunID),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
					"current_run_id":              uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					WorkflowID: "wfid",
				},
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: common.StringPtr("fd88863f-bb32-4daa-8878-49e08b91545e"), // not matching current_run_id above on purpose
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				CurrentWorkflowConditionFailInfo: common.StringPtr(
					"Workflow execution creation condition failed by mismatch runID. " +
						"WorkflowId: wfid, Expected Current RunID: fd88863f-bb32-4daa-8878-49e08b91545e, " +
						"Actual Current RunID: bda9cd9c-32fb-4267-b120-346e5351fc46"),
			},
		},
		{
			desc: "creation condition failed by mismatch last write version",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse(permanentRunID),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
					"current_run_id":              uuid.Parse("fd88863f-bb32-4daa-8878-49e08b91545e"),
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					WorkflowID: "wfid",
				},
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID:     common.StringPtr("fd88863f-bb32-4daa-8878-49e08b91545e"), // not matching current_run_id above on purpose
					LastWriteVersion: common.Int64Ptr(1),
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				CurrentWorkflowConditionFailInfo: common.StringPtr(
					"Workflow execution creation condition failed. " +
						"WorkflowId: wfid, Expected Version: 1, Actual Version: 3"),
			},
		},
		{
			desc: "creation condition failed by mismatch state",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse(permanentRunID),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
					"current_run_id":              uuid.Parse("fd88863f-bb32-4daa-8878-49e08b91545e"),
					"workflow_state":              2,
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					WorkflowID: "wfid",
				},
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID:     common.StringPtr("fd88863f-bb32-4daa-8878-49e08b91545e"), // not matching current_run_id above on purpose
					LastWriteVersion: common.Ptr(int64(3)),
					State:            common.Ptr(int(10)),
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				CurrentWorkflowConditionFailInfo: common.StringPtr(
					"Workflow execution creation condition failed. " +
						"WorkflowId: wfid, Expected State: 10, Actual State: 2"),
			},
		},
		{
			desc: "concrete execution already exists",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			execution: &nosqlplugin.WorkflowExecutionRequest{
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					WorkflowID:      "wfid",
					RunID:           "bda9cd9c-32fb-4267-b120-346e5351fc46",
					CreateRequestID: "reqid_123",
					State:           2,
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
					OtherInfo: "Workflow execution already running. WorkflowId: wfid, " +
						"RunId: bda9cd9c-32fb-4267-b120-346e5351fc46",
					CreateRequestID:  "reqid_123",
					RunID:            "bda9cd9c-32fb-4267-b120-346e5351fc46",
					State:            2,
					LastWriteVersion: 3,
				},
			},
		},
		{
			desc: "unknown condition failure",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":                        rowTypeExecution,
					"run_id":                      uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
					"range_id":                    int64(100),
					"workflow_last_write_version": int64(3),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			execution: &nosqlplugin.WorkflowExecutionRequest{
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					RunID: "something else",
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				UnknownConditionFailureDetails: common.StringPtr("Failed to operate on workflow execution.  Request RangeID: 100"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := executeCreateWorkflowBatchTransaction(context.Background(), tc.session, tc.batch, tc.currentWFReq, tc.execution, tc.shardCond)
			if diff := errDiff(tc.wantErr, err); diff != "" {
				t.Fatalf("Error mismatch (-want +got):\n%s", diff)
			}
			if !tc.session.iter.closed {
				t.Error("iterator not closed")
			}
		})
	}
}

func TestExecuteUpdateWorkflowBatchTransaction(t *testing.T) {
	tests := []struct {
		// fake setup parameters
		desc    string
		session *fakeSession

		// executeUpdateWorkflowBatchTransaction args
		batch               *fakeBatch
		currentWFReq        *nosqlplugin.CurrentWorkflowWriteRequest
		prevNextEventIDCond int64
		shardCond           *nosqlplugin.ShardCondition

		// expectations
		wantErr error
	}{
		{
			desc:  "applied",
			batch: &fakeBatch{},
			session: &fakeSession{
				mapExecuteBatchCASApplied: true,
				iter:                      &fakeIter{},
			},
		},
		{
			desc:  "CAS error",
			batch: &fakeBatch{},
			session: &fakeSession{
				mapExecuteBatchCASErr: fmt.Errorf("db operation failed for some reason"),
				iter:                  &fakeIter{},
			},
			wantErr: fmt.Errorf("db operation failed for some reason"),
		},
		{
			desc: "range id mismatch for shard row",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":     rowTypeShard,
					"run_id":   uuid.Parse("bda9cd9c-32fb-4267-b120-346e5351fc46"),
					"range_id": int64(200),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				ShardRangeIDNotMatch: common.Int64Ptr(200),
			},
		},
		{
			desc: "workflow request already exists",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":   rowTypeWorkflowRequestSignal,
					"run_id": uuid.Parse("4b8045c8-7b45-41e0-bf03-1f0d166b818d"),
				},
				query: &fakeQuery{
					mapScan: map[string]interface{}{
						"current_run_id": uuid.Parse("6b844fb4-c18a-4979-a2d3-731ebdd1db08"),
					},
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			shardCond: &nosqlplugin.ShardCondition{
				ShardID: 1,
				RangeID: 100,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				DuplicateRequest: &nosqlplugin.DuplicateRequest{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RunID:       "6b844fb4-c18a-4979-a2d3-731ebdd1db08",
				},
			},
		},
		{
			desc: "workflow request already exists - but failed to get latest request",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":   rowTypeWorkflowRequestCancel,
					"run_id": uuid.Parse("4b8045c8-7b45-41e0-bf03-1f0d166b818d"),
				},
				query: &fakeQuery{
					mapScan: map[string]interface{}{
						"current_run_id": uuid.Parse("6b844fb4-c18a-4979-a2d3-731ebdd1db08"),
					},
					err: fmt.Errorf("failed to get latest request"),
				},
			},
			batch:        &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{},
			shardCond: &nosqlplugin.ShardCondition{
				ShardID: 1,
				RangeID: 100,
			},
			wantErr: fmt.Errorf("failed to get latest request"),
		},
		{
			desc: "nextEventID mismatch for execution row",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":          rowTypeExecution,
					"run_id":        uuid.Parse("0875863e-dcef-496a-b8a2-3210b2958e25"),
					"next_event_id": int64(10),
				},
			},
			batch:               &fakeBatch{},
			prevNextEventIDCond: 11, // not matching next_event_id above on purpose
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID: "0875863e-dcef-496a-b8a2-3210b2958e25",
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				RangeID: 200,
			},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				UnknownConditionFailureDetails: common.StringPtr(
					"Failed to update mutable state. " +
						"previousNextEventIDCondition: 11, actualNextEventID: 10, Request Current RunID: 0875863e-dcef-496a-b8a2-3210b2958e25"),
			},
		},
		{
			desc: "runID mismatch for current execution row",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":           rowTypeExecution,
					"run_id":         uuid.Parse(permanentRunID),
					"current_run_id": uuid.Parse("0875863e-dcef-496a-b8a2-3210b2958e25"),
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: common.StringPtr("fd88863f-bb32-4daa-8878-49e08b91545e"), // not matching current_run_id above on purpose
				},
			},
			shardCond: &nosqlplugin.ShardCondition{},
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				CurrentWorkflowConditionFailInfo: common.StringPtr(
					"Failed to update mutable state. requestConditionalRunID: fd88863f-bb32-4daa-8878-49e08b91545e, " +
						"Actual Value: 0875863e-dcef-496a-b8a2-3210b2958e25"),
			},
		},
		{
			desc: "unknown condition failure",
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				iter:                      &fakeIter{},
				mapExecuteBatchCASPrev: map[string]any{
					"type":           rowTypeExecution,
					"run_id":         uuid.Parse(permanentRunID),
					"current_run_id": uuid.Parse("0875863e-dcef-496a-b8a2-3210b2958e25"),
				},
			},
			batch: &fakeBatch{},
			currentWFReq: &nosqlplugin.CurrentWorkflowWriteRequest{
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: common.StringPtr("0875863e-dcef-496a-b8a2-3210b2958e25"), // not matching current_run_id above on purpose
				},
			},
			shardCond: &nosqlplugin.ShardCondition{
				ShardID: 345,
				RangeID: 200,
			},
			prevNextEventIDCond: 11,
			wantErr: &nosqlplugin.WorkflowOperationConditionFailure{
				UnknownConditionFailureDetails: common.StringPtr(
					"Failed to update mutable state. ShardID: 345, RangeID: 200, previousNextEventIDCondition: 11, requestConditionalRunID: 0875863e-dcef-496a-b8a2-3210b2958e25"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := executeUpdateWorkflowBatchTransaction(context.Background(), tc.session, tc.batch, tc.currentWFReq, tc.prevNextEventIDCond, tc.shardCond)
			if diff := errDiff(tc.wantErr, err); diff != "" {
				t.Fatalf("Error mismatch (-want +got):\n%s", diff)
			}
			if !tc.session.iter.closed {
				t.Error("iterator not closed")
			}
		})
	}
}

func TestToRequestRowType(t *testing.T) {
	testCases := []struct {
		name        string
		requestType persistence.WorkflowRequestType
		wantErr     bool
		want        int
	}{
		{
			name:        "StartWorkflow request",
			requestType: persistence.WorkflowRequestTypeStart,
			wantErr:     false,
			want:        rowTypeWorkflowRequestStart,
		},
		{
			name:        "SignalWithWorkflow request",
			requestType: persistence.WorkflowRequestTypeSignal,
			wantErr:     false,
			want:        rowTypeWorkflowRequestSignal,
		},
		{
			name:        "SignalWorkflow request",
			requestType: persistence.WorkflowRequestTypeSignal,
			wantErr:     false,
			want:        rowTypeWorkflowRequestSignal,
		},
		{
			name:        "CancelWorkflow request",
			requestType: persistence.WorkflowRequestTypeCancel,
			wantErr:     false,
			want:        rowTypeWorkflowRequestCancel,
		},
		{
			name:        "ResetWorkflow request",
			requestType: persistence.WorkflowRequestTypeReset,
			wantErr:     false,
			want:        rowTypeWorkflowRequestReset,
		},
		{
			name:        "unknown request",
			requestType: persistence.WorkflowRequestType(-999),
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := toRequestRowType(tc.requestType)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Fatalf("request type mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFromRequestRowType(t *testing.T) {
	testCases := []struct {
		name        string
		requestType int
		wantErr     bool
		want        persistence.WorkflowRequestType
	}{
		{
			name:        "StartWorkflow request",
			requestType: rowTypeWorkflowRequestStart,
			wantErr:     false,
			want:        persistence.WorkflowRequestTypeStart,
		},
		{
			name:        "SignalWithWorkflow request",
			requestType: rowTypeWorkflowRequestSignal,
			wantErr:     false,
			want:        persistence.WorkflowRequestTypeSignal,
		},
		{
			name:        "SignalWorkflow request",
			requestType: rowTypeWorkflowRequestSignal,
			wantErr:     false,
			want:        persistence.WorkflowRequestTypeSignal,
		},
		{
			name:        "CancelWorkflow request",
			requestType: rowTypeWorkflowRequestCancel,
			wantErr:     false,
			want:        persistence.WorkflowRequestTypeCancel,
		},
		{
			name:        "ResetWorkflow request",
			requestType: rowTypeWorkflowRequestReset,
			wantErr:     false,
			want:        persistence.WorkflowRequestTypeReset,
		},
		{
			name:        "unknown request",
			requestType: rowTypeShard,
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := fromRequestRowType(tc.requestType)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Fatalf("request type mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsertOrUpsertWorkflowRequestRow(t *testing.T) {
	testCases := []struct {
		name        string
		request     *nosqlplugin.WorkflowRequestsWriteRequest
		wantErr     bool
		wantQueries []string
	}{
		{
			name: "WorkflowRequestWriteModeInsert",
			request: &nosqlplugin.WorkflowRequestsWriteRequest{
				Rows: []*nosqlplugin.WorkflowRequestRow{
					{
						ShardID:    1,
						DomainID:   "c09537fd-67ce-4b08-a817-eb8f12ad3a91",
						WorkflowID: "test",
						RunID:      "25bd1013-0e79-4c45-8e55-08bb45886896",
						RequestID:  "9ab1d25d-8620-440a-b3f1-d3167e08c769",
						Version:    100,
					},
				},
				WriteMode: nosqlplugin.WorkflowRequestWriteModeInsert,
			},
			wantErr: false,
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, visibility_ts, task_id, current_run_id, created_time) ` +
					`VALUES(1, 7, c09537fd-67ce-4b08-a817-eb8f12ad3a91, test, 9ab1d25d-8620-440a-b3f1-d3167e08c769, 946684800000, 1000, 25bd1013-0e79-4c45-8e55-08bb45886896, 2025-01-06T15:00:00Z) ` +
					`IF NOT EXISTS USING TTL 10800`,
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, visibility_ts, task_id, current_run_id, created_time) ` +
					`VALUES(1, 7, c09537fd-67ce-4b08-a817-eb8f12ad3a91, test, 9ab1d25d-8620-440a-b3f1-d3167e08c769, 946684800000, -100, 25bd1013-0e79-4c45-8e55-08bb45886896, 2025-01-06T15:00:00Z) ` +
					`IF NOT EXISTS USING TTL 10800`,
			},
		},
		{
			name: "WorkflowRequestWriteModeUpsert",
			request: &nosqlplugin.WorkflowRequestsWriteRequest{
				Rows: []*nosqlplugin.WorkflowRequestRow{
					{
						ShardID:    1,
						DomainID:   "c09537fd-67ce-4b08-a817-eb8f12ad3a91",
						WorkflowID: "test",
						RunID:      "25bd1013-0e79-4c45-8e55-08bb45886896",
						RequestID:  "9ab1d25d-8620-440a-b3f1-d3167e08c769",
						Version:    100,
					},
				},
				WriteMode: nosqlplugin.WorkflowRequestWriteModeUpsert,
			},
			wantErr: false,
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, visibility_ts, task_id, current_run_id, last_updated_time) ` +
					`VALUES(1, 7, c09537fd-67ce-4b08-a817-eb8f12ad3a91, test, 9ab1d25d-8620-440a-b3f1-d3167e08c769, 946684800000, 1000, 25bd1013-0e79-4c45-8e55-08bb45886896, 2025-01-06T15:00:00Z) ` +
					`USING TTL 10800`,
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, visibility_ts, task_id, current_run_id, last_updated_time) ` +
					`VALUES(1, 7, c09537fd-67ce-4b08-a817-eb8f12ad3a91, test, 9ab1d25d-8620-440a-b3f1-d3167e08c769, 946684800000, -100, 25bd1013-0e79-4c45-8e55-08bb45886896, 2025-01-06T15:00:00Z) ` +
					`USING TTL 10800`,
			},
		},
		{
			name: "unknown mode",
			request: &nosqlplugin.WorkflowRequestsWriteRequest{
				Rows: []*nosqlplugin.WorkflowRequestRow{
					{
						RequestType: persistence.WorkflowRequestTypeStart,
					},
				},
				WriteMode: nosqlplugin.WorkflowRequestWriteMode(-100),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			batch := &fakeBatch{}
			err := insertOrUpsertWorkflowRequestRow(batch, tc.request, FixedTime)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateTimerTasks(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-12T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		timerTasks []*nosqlplugin.HistoryMigrationTask
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain_xyz",
			workflowID: "workflow_xyz",
			timerTasks: []*nosqlplugin.HistoryMigrationTask{
				{
					Timer: &nosqlplugin.TimerTask{
						RunID:               "rundid_1",
						TaskID:              1,
						TaskType:            1,
						TimeoutType:         1,
						EventID:             10,
						ScheduleAttempt:     0,
						Version:             0,
						VisibilityTimestamp: ts,
					},
					Task: &persistence.DataBlob{
						Data:     []byte("timer1"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
				{
					Timer: &nosqlplugin.TimerTask{
						RunID:               "rundid_1",
						TaskID:              2,
						TaskType:            1,
						TimeoutType:         1,
						EventID:             11,
						ScheduleAttempt:     0,
						Version:             0,
						VisibilityTimestamp: ts.Add(time.Minute),
					},
					Task: &persistence.DataBlob{
						Data:     []byte("timer2"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
			},
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, timer, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 3, 10000000-4000-f000-f000-000000000000, 20000000-4000-f000-f000-000000000000, 30000000-4000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_1, visibility_ts: 1702418921000, task_id: 1, type: 1, timeout_type: 1, event_id: 10, schedule_attempt: 0, version: 0}, ` +
					`[116 105 109 101 114 49], thriftrw, 1702418921000, 1, 2025-01-06T15:00:00Z)`,
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, timer, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 3, 10000000-4000-f000-f000-000000000000, 20000000-4000-f000-f000-000000000000, 30000000-4000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_1, visibility_ts: 1702418981000, task_id: 2, type: 1, timeout_type: 1, event_id: 11, schedule_attempt: 0, version: 0}, ` +
					`[116 105 109 101 114 50], thriftrw, 1702418981000, 2, 2025-01-06T15:00:00Z)`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}
			err := createTimerTasks(batch, tc.shardID, tc.domainID, tc.workflowID, tc.timerTasks, FixedTime)
			if err != nil {
				t.Fatalf("createTimerTasks failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReplicationTasks(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-12T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		replTasks  []*nosqlplugin.HistoryMigrationTask
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain_xyz",
			workflowID: "workflow_xyz",
			replTasks: []*nosqlplugin.HistoryMigrationTask{
				{
					Replication: &nosqlplugin.ReplicationTask{
						RunID:             "rundid_1",
						TaskID:            644,
						TaskType:          0,
						FirstEventID:      5,
						NextEventID:       8,
						Version:           0,
						ScheduledID:       constants.EmptyEventID,
						NewRunBranchToken: []byte{'a', 'b', 'c'},
						CreationTime:      ts,
					},
					Task: &persistence.DataBlob{
						Data:     []byte("rep1"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
				{
					Replication: &nosqlplugin.ReplicationTask{
						RunID:             "rundid_1",
						TaskID:            645,
						TaskType:          0,
						FirstEventID:      25,
						NextEventID:       28,
						Version:           0,
						ScheduledID:       constants.EmptyEventID,
						NewRunBranchToken: []byte{'a', 'b', 'c'},
						CreationTime:      ts.Add(time.Hour),
					},
					Task: &persistence.DataBlob{
						Data:     []byte("rep2"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
			},
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, replication, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 4, 10000000-5000-f000-f000-000000000000, 20000000-5000-f000-f000-000000000000, 30000000-5000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_1, task_id: 644, type: 0, ` +
					`first_event_id: 5,next_event_id: 8,version: 0,scheduled_id: -23, event_store_version: 2, branch_token: [], ` +
					`new_run_event_store_version: 2, new_run_branch_token: [97 98 99], created_time: 1702418921000000000 }, [114 101 112 49], thriftrw, 946684800000, 644, 2025-01-06T15:00:00Z)`,
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, replication, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 4, 10000000-5000-f000-f000-000000000000, 20000000-5000-f000-f000-000000000000, 30000000-5000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_1, task_id: 645, type: 0, ` +
					`first_event_id: 25,next_event_id: 28,version: 0,scheduled_id: -23, event_store_version: 2, branch_token: [], ` +
					`new_run_event_store_version: 2, new_run_branch_token: [97 98 99], created_time: 1702422521000000000 }, [114 101 112 50], thriftrw, 946684800000, 645, 2025-01-06T15:00:00Z)`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}
			createReplicationTasks(batch, tc.shardID, tc.domainID, tc.workflowID, tc.replTasks, FixedTime)
			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTransferTasks(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-12T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc          string
		shardID       int
		domainID      string
		workflowID    string
		transferTasks []*nosqlplugin.HistoryMigrationTask
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain_xyz",
			workflowID: "workflow_xyz",
			transferTasks: []*nosqlplugin.HistoryMigrationTask{
				{
					Transfer: &nosqlplugin.TransferTask{
						RunID:                   "rundid_1",
						TaskID:                  355,
						TaskType:                0,
						Version:                 1,
						VisibilityTimestamp:     ts,
						TargetDomainID:          "e2bf2c8f-0ddf-4451-8840-27cfe8addd62",
						TargetWorkflowID:        persistence.TransferTaskTransferTargetWorkflowID,
						TargetRunID:             persistence.TransferTaskTransferTargetRunID,
						TargetChildWorkflowOnly: true,
						TaskList:                "tasklist_1",
						ScheduleID:              14,
					},
					Task: &persistence.DataBlob{
						Data:     []byte("tr1"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
				{
					Transfer: &nosqlplugin.TransferTask{
						RunID:                   "rundid_2",
						TaskID:                  220,
						TaskType:                0,
						Version:                 1,
						VisibilityTimestamp:     ts.Add(time.Minute),
						TargetDomainID:          "e2bf2c8f-0ddf-4451-8840-27cfe8addd62",
						TargetWorkflowID:        persistence.TransferTaskTransferTargetWorkflowID,
						TargetRunID:             persistence.TransferTaskTransferTargetRunID,
						TargetChildWorkflowOnly: true,
						TaskList:                "tasklist_2",
						ScheduleID:              3,
					},
					Task: &persistence.DataBlob{
						Data:     []byte("tr2"),
						Encoding: constants.EncodingTypeThriftRW,
					},
				},
			},
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, transfer, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 2, 10000000-3000-f000-f000-000000000000, 20000000-3000-f000-f000-000000000000, 30000000-3000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_1, visibility_ts: 2023-12-12T22:08:41Z, ` +
					`task_id: 355, target_domain_id: e2bf2c8f-0ddf-4451-8840-27cfe8addd62, target_domain_ids: map[],` +
					`target_workflow_id: 20000000-0000-f000-f000-000000000001, target_run_id: 30000000-0000-f000-f000-000000000002, ` +
					`target_child_workflow_only: true, task_list: tasklist_1, type: 0, schedule_id: 14, record_visibility: false, version: 1}, ` +
					`[116 114 49], thriftrw, 946684800000, 355, 2025-01-06T15:00:00Z)`,
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, transfer, data, data_encoding, visibility_ts, task_id, created_time) ` +
					`VALUES(1000, 2, 10000000-3000-f000-f000-000000000000, 20000000-3000-f000-f000-000000000000, 30000000-3000-f000-f000-000000000000, ` +
					`{domain_id: domain_xyz, workflow_id: workflow_xyz, run_id: rundid_2, visibility_ts: 2023-12-12T22:09:41Z, ` +
					`task_id: 220, target_domain_id: e2bf2c8f-0ddf-4451-8840-27cfe8addd62, target_domain_ids: map[],` +
					`target_workflow_id: 20000000-0000-f000-f000-000000000001, target_run_id: 30000000-0000-f000-f000-000000000002, ` +
					`target_child_workflow_only: true, task_list: tasklist_2, type: 0, schedule_id: 3, record_visibility: false, version: 1}, ` +
					`[116 114 50], thriftrw, 946684800000, 220, 2025-01-06T15:00:00Z)`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}
			createTransferTasks(batch, tc.shardID, tc.domainID, tc.workflowID, tc.transferTasks, FixedTime)
			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetSignalsRequested(t *testing.T) {
	tests := []struct {
		desc         string
		shardID      int
		domainID     string
		workflowID   string
		runID        string
		signalReqIDs []string
		// expectations
		wantQueries []string
	}{
		{
			desc:         "ok",
			shardID:      1000,
			domainID:     "domain_123",
			workflowID:   "workflow_123",
			runID:        "runid_123",
			signalReqIDs: []string{"signalReqID_1", "signalReqID_2"},
			wantQueries: []string{
				`UPDATE executions SET signal_requested = [signalReqID_1 signalReqID_2] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain_123 and workflow_id = workflow_123 and ` +
					`run_id = runid_123 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}
			err := resetSignalsRequested(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.signalReqIDs, FixedTime)
			if err != nil {
				t.Fatalf("resetSignalsRequested failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateSignalsRequested(t *testing.T) {
	tests := []struct {
		desc               string
		shardID            int
		domainID           string
		workflowID         string
		runID              string
		signalReqIDs       []string
		deleteSignalReqIDs []string
		// expectations
		wantQueries []string
	}{
		{
			desc:               "update only",
			shardID:            1000,
			domainID:           "domain_abc",
			workflowID:         "workflow_abc",
			runID:              "runid_abc",
			signalReqIDs:       []string{"signalReqID_3", "signalReqID_4"},
			deleteSignalReqIDs: []string{},
			wantQueries: []string{
				`UPDATE executions SET signal_requested = signal_requested + [signalReqID_3 signalReqID_4] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain_abc and workflow_id = workflow_abc and ` +
					`run_id = runid_abc and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
		{
			desc:               "delete only",
			shardID:            1001,
			domainID:           "domain_def",
			workflowID:         "workflow_def",
			runID:              "runid_def",
			signalReqIDs:       []string{},
			deleteSignalReqIDs: []string{"signalReqID_5", "signalReqID_6"},
			wantQueries: []string{
				`UPDATE executions SET signal_requested = signal_requested - [signalReqID_5 signalReqID_6] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1001 and type = 1 and domain_id = domain_def and workflow_id = workflow_def and ` +
					`run_id = runid_def and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
		{
			desc:               "update and delete",
			shardID:            1002,
			domainID:           "domain_ghi",
			workflowID:         "workflow_ghi",
			runID:              "runid_ghi",
			signalReqIDs:       []string{"signalReqID_7"},
			deleteSignalReqIDs: []string{"signalReqID_8"},
			wantQueries: []string{
				`UPDATE executions SET signal_requested = signal_requested + [signalReqID_7] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1002 and type = 1 and domain_id = domain_ghi and workflow_id = workflow_ghi and ` +
					`run_id = runid_ghi and visibility_ts = 946684800000 and task_id = -10 `,
				`UPDATE executions SET signal_requested = signal_requested - [signalReqID_8] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1002 and type = 1 and domain_id = domain_ghi and workflow_id = workflow_ghi and ` +
					`run_id = runid_ghi and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}
			err := updateSignalsRequested(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.signalReqIDs, tc.deleteSignalReqIDs, FixedTime)
			if err != nil {
				t.Fatalf("updateSignalsRequested failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetSignalInfos(t *testing.T) {
	tests := []struct {
		desc        string
		shardID     int
		domainID    string
		workflowID  string
		runID       string
		signalInfos map[int64]*persistence.SignalInfo
		// expectations
		wantQueries []string
	}{
		{
			desc:       "single signal info",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			signalInfos: map[int64]*persistence.SignalInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					SignalRequestID:       "request1",
					SignalName:            "signal1",
					Input:                 []byte("input1"),
					Control:               []byte("control1"),
				},
				2: {
					Version:               1,
					InitiatedID:           5,
					InitiatedEventBatchID: 6,
					SignalRequestID:       "request2",
					SignalName:            "signal2",
					Input:                 []byte("input2"),
					Control:               []byte("control2"),
				},
			},
			wantQueries: []string{
				`UPDATE executions SET signal_map = map[` +
					`1:map[control:[99 111 110 116 114 111 108 49] initiated_event_batch_id:2 initiated_id:1 input:[105 110 112 117 116 49] signal_name:signal1 signal_request_id:request1 version:1] ` +
					`5:map[control:[99 111 110 116 114 111 108 50] initiated_event_batch_id:6 initiated_id:5 input:[105 110 112 117 116 50] signal_name:signal2 signal_request_id:request2 version:1]` +
					`] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetSignalInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.signalInfos, FixedTime)
			if err != nil {
				t.Fatalf("resetSignalInfos failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateSignalInfos(t *testing.T) {
	tests := []struct {
		desc        string
		shardID     int
		domainID    string
		workflowID  string
		runID       string
		signalInfos map[int64]*persistence.SignalInfo
		deleteInfos []int64
		// expectations
		wantQueries []string
	}{
		{
			desc:       "update and delete signal infos",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			signalInfos: map[int64]*persistence.SignalInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					SignalRequestID:       "request1",
					SignalName:            "signal1",
					Input:                 []byte("input1"),
					Control:               []byte("control1"),
				},
			},
			deleteInfos: []int64{2},
			wantQueries: []string{
				`UPDATE executions SET signal_map[ 1 ] = {` +
					`version: 1, initiated_id: 1, initiated_event_batch_id: 2, signal_request_id: request1, ` +
					`signal_name: signal1, input: [105 110 112 117 116 49], ` +
					`control: [99 111 110 116 114 111 108 49]` +
					`} , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and ` +
					`workflow_id = workflow1 and run_id = runid1 and ` +
					`visibility_ts = 946684800000 and task_id = -10 `,
				`DELETE signal_map[ 2 ] FROM executions WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and ` +
					`workflow_id = workflow1 and run_id = runid1 ` +
					`and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateSignalInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.signalInfos, tc.deleteInfos, FixedTime)
			if err != nil {
				t.Fatalf("updateSignalInfos failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetRequestCancelInfos(t *testing.T) {
	tests := []struct {
		desc               string
		shardID            int
		domainID           string
		workflowID         string
		runID              string
		requestCancelInfos map[int64]*persistence.RequestCancelInfo
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			requestCancelInfos: map[int64]*persistence.RequestCancelInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					CancelRequestID:       "cancelRequest1",
				},
				3: {
					Version:               2,
					InitiatedID:           3,
					InitiatedEventBatchID: 4,
					CancelRequestID:       "cancelRequest3",
				},
			},
			wantQueries: []string{
				`UPDATE executions SET request_cancel_map = map[` +
					`1:map[cancel_request_id:cancelRequest1 initiated_event_batch_id:2 initiated_id:1 version:1] ` +
					`3:map[cancel_request_id:cancelRequest3 initiated_event_batch_id:4 initiated_id:3 version:2]` +
					`], last_updated_time = 2025-01-06T15:00:00Z WHERE shard_id = 1000 and type = 1 and domain_id = domain1 and ` +
					`workflow_id = workflow1 and run_id = runid1 and ` +
					`visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetRequestCancelInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.requestCancelInfos, FixedTime)
			if err != nil {
				t.Fatalf("resetRequestCancelInfos failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateRequestCancelInfos(t *testing.T) {
	tests := []struct {
		desc               string
		shardID            int
		domainID           string
		workflowID         string
		runID              string
		requestCancelInfos map[int64]*persistence.RequestCancelInfo
		deleteInfos        []int64
		// expectations
		wantQueries []string
	}{
		{
			desc:       "update and delete request cancel infos",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			requestCancelInfos: map[int64]*persistence.RequestCancelInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					CancelRequestID:       "cancelRequest1",
				},
			},
			deleteInfos: []int64{2},
			wantQueries: []string{
				`UPDATE executions SET request_cancel_map[ 1 ] = ` +
					`{version: 1,initiated_id: 1, initiated_event_batch_id: 2, cancel_request_id: cancelRequest1 } , last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE shard_id = 1000 and type = 1 and domain_id = domain1 and ` +
					`workflow_id = workflow1 and run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
				`DELETE request_cancel_map[ 2 ] FROM executions WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateRequestCancelInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.requestCancelInfos, tc.deleteInfos, FixedTime)
			if err != nil {
				t.Fatalf("updateRequestCancelInfos failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetChildExecutionInfos(t *testing.T) {
	tests := []struct {
		desc                string
		shardID             int
		domainID            string
		workflowID          string
		runID               string
		childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo
		// expectations
		wantQueries []string
		wantErr     bool
	}{
		{
			desc:       "execution info with runid",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			childExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					StartedID:             3,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					StartedWorkflowID: "startedWorkflowID1",
					StartedRunID:      "startedRunID1",
					CreateRequestID:   "createRequestID1",
					InitiatedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					DomainID:          "domain1",
					WorkflowTypeName:  "workflowType1",
					ParentClosePolicy: types.ParentClosePolicyAbandon,
				},
			},
			wantQueries: []string{
				`UPDATE executions SET child_executions_map = ` +
					`map[1:map[` +
					`create_request_id:createRequestID1 domain_id:domain1 domain_name: event_data_encoding:thriftrw ` +
					`initiated_event:[] initiated_event_batch_id:2 initiated_id:1 parent_close_policy:0 ` +
					`started_event:[] started_id:3 started_run_id:startedRunID1 started_workflow_id:startedWorkflowID1 ` +
					`version:1 workflow_type_name:workflowType1` +
					`]], last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
		{
			desc:       "execution info without runid",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      emptyRunID,
			childExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					StartedID:             3,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					StartedWorkflowID: "startedWorkflowID1",
					StartedRunID:      "", // leave empty and validate it's querying empty runid
					CreateRequestID:   "createRequestID1",
					InitiatedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					DomainID:          "domain1",
					WorkflowTypeName:  "workflowType1",
					ParentClosePolicy: types.ParentClosePolicyAbandon,
				},
			},
			wantQueries: []string{
				`UPDATE executions SET child_executions_map = ` +
					`map[1:map[` +
					`create_request_id:createRequestID1 domain_id:domain1 domain_name: event_data_encoding:thriftrw ` +
					`initiated_event:[] initiated_event_batch_id:2 initiated_id:1 parent_close_policy:0 ` +
					`started_event:[] started_id:3 started_run_id:30000000-0000-f000-f000-000000000000 started_workflow_id:startedWorkflowID1 ` +
					`version:1 workflow_type_name:workflowType1` +
					`]], last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = 30000000-0000-f000-f000-000000000000 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetChildExecutionInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.childExecutionInfos, FixedTime)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resetChildExecutionInfos() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateChildExecutionInfos(t *testing.T) {
	tests := []struct {
		desc                string
		shardID             int
		domainID            string
		workflowID          string
		runID               string
		childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo
		deleteInfos         []int64
		// expectations
		wantQueries []string
	}{
		{
			desc:       "update and delete child execution infos",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			childExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{
				1: {
					Version:               1,
					InitiatedID:           1,
					InitiatedEventBatchID: 2,
					StartedID:             3,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					StartedWorkflowID: "startedWorkflowID1",
					StartedRunID:      "startedRunID1",
					CreateRequestID:   "createRequestID1",
					InitiatedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
					},
					DomainID:          "domain1",
					WorkflowTypeName:  "workflowType1",
					ParentClosePolicy: types.ParentClosePolicyAbandon,
				},
			},
			deleteInfos: []int64{2},
			wantQueries: []string{
				`UPDATE executions SET child_executions_map[ 1 ] = {` +
					`version: 1, initiated_id: 1, initiated_event_batch_id: 2, initiated_event: [], ` +
					`started_id: 3, started_workflow_id: startedWorkflowID1, started_run_id: startedRunID1, ` +
					`started_event: [], create_request_id: createRequestID1, event_data_encoding: thriftrw, ` +
					`domain_id: domain1, domain_name: , workflow_type_name: workflowType1, parent_close_policy: 0` +
					`} , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
				`DELETE child_executions_map[ 2 ] FROM executions WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateChildExecutionInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.childExecutionInfos, tc.deleteInfos, FixedTime)
			if err != nil {
				t.Fatalf("updateChildExecutionInfos failed: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetTimerInfos(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-12T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		runID      string
		timerInfos map[string]*persistence.TimerInfo
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			timerInfos: map[string]*persistence.TimerInfo{
				"timer1": {
					Version:    1,
					TimerID:    "timer1",
					StartedID:  2,
					ExpiryTime: ts.UTC(),
					TaskStatus: 1,
				},
			},
			wantQueries: []string{
				`UPDATE executions SET timer_map = map[` +
					`timer1:map[expiry_time:2023-12-12 22:08:41 +0000 UTC started_id:2 task_id:1 timer_id:timer1 version:1]` +
					`] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetTimerInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.timerInfos, FixedTime)
			if err != nil {
				t.Fatalf("resetTimerInfos() error = %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateTimerInfos(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc        string
		shardID     int
		domainID    string
		workflowID  string
		runID       string
		timerInfos  map[string]*persistence.TimerInfo
		deleteInfos []string
		// expectations
		wantQueries []string
	}{
		{
			desc:       "update and delete timer infos",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			timerInfos: map[string]*persistence.TimerInfo{
				"timer1": {
					TimerID:    "timer1",
					Version:    1,
					StartedID:  2,
					ExpiryTime: ts.UTC(),
					TaskStatus: 1,
				},
			},
			deleteInfos: []string{"timer2"},
			wantQueries: []string{
				`UPDATE executions SET timer_map[ timer1 ] = {` +
					`version: 1, timer_id: timer1, started_id: 2, expiry_time: 2023-12-19T22:08:41Z, task_id: 1` +
					`} , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
				`DELETE timer_map[ timer2 ] FROM executions WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateTimerInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.timerInfos, tc.deleteInfos, FixedTime)
			if err != nil {
				t.Fatalf("updateTimerInfos() error = %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResetActivityInfos(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc          string
		shardID       int
		domainID      string
		workflowID    string
		runID         string
		activityInfos map[int64]*persistence.InternalActivityInfo
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			activityInfos: map[int64]*persistence.InternalActivityInfo{
				1: {
					Version: 1,
					ScheduledEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-scheduled-event-data"),
					},
					ScheduledTime: ts.UTC(),
					ScheduleID:    1,
					StartedID:     2,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-started-event-data"),
					},
					ActivityID:             "activity1",
					ScheduleToStartTimeout: 1 * time.Minute,
					ScheduleToCloseTimeout: 2 * time.Minute,
					StartToCloseTimeout:    3 * time.Minute,
					HeartbeatTimeout:       1 * time.Minute,
					Attempt:                3,
					MaximumAttempts:        5,
					TaskList:               "tasklist1",
					HasRetryPolicy:         true,
					LastFailureReason:      "retry reason",
				},
				2: {
					Version: 1,
					ScheduledEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-scheduled-event-data"),
					},
					ScheduledTime: ts.UTC(),
					ScheduleID:    2,
					StartedID:     3,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-started-event-data"),
					},
					ActivityID:             "activity2",
					ScheduleToStartTimeout: 1 * time.Minute,
					ScheduleToCloseTimeout: 2 * time.Minute,
					StartToCloseTimeout:    3 * time.Minute,
					HeartbeatTimeout:       1 * time.Minute,
					Attempt:                1,
					MaximumAttempts:        5,
					TaskList:               "tasklist1",
					HasRetryPolicy:         true,
					LastFailureReason:      "another retry reason",
				},
			},
			wantQueries: []string{
				`UPDATE executions SET activity_map = map[` +
					`1:map[` +
					`activity_id:activity1 attempt:3 backoff_coefficient:0 cancel_request_id:0 cancel_requested:false ` +
					`details:[] event_data_encoding:thriftrw expiration_time:0001-01-01 00:00:00 +0000 UTC has_retry_policy:true ` +
					`heart_beat_timeout:60 init_interval:0 last_failure_details:[] last_failure_reason:retry reason ` +
					`last_hb_updated_time:0001-01-01 00:00:00 +0000 UTC last_worker_identity: max_attempts:5 max_interval:0 ` +
					`non_retriable_errors:[] request_id: schedule_id:1 schedule_to_close_timeout:120 schedule_to_start_timeout:60 ` +
					`scheduled_event:[116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 99 104 101 100 117 108 101 100 45 101 118 101 110 116 45 100 97 116 97] ` +
					`scheduled_event_batch_id:0 scheduled_time:2023-12-19 22:08:41 +0000 UTC start_to_close_timeout:180 ` +
					`started_event:[116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 116 97 114 116 101 100 45 101 118 101 110 116 45 100 97 116 97] ` +
					`started_id:2 started_identity: started_time:0001-01-01 00:00:00 +0000 UTC task_list:tasklist1 timer_task_status:0 version:1` +
					`] ` +
					`2:map[` +
					`activity_id:activity2 attempt:1 backoff_coefficient:0 cancel_request_id:0 cancel_requested:false ` +
					`details:[] event_data_encoding:thriftrw expiration_time:0001-01-01 00:00:00 +0000 UTC has_retry_policy:true ` +
					`heart_beat_timeout:60 init_interval:0 last_failure_details:[] last_failure_reason:another retry reason ` +
					`last_hb_updated_time:0001-01-01 00:00:00 +0000 UTC last_worker_identity: max_attempts:5 max_interval:0 ` +
					`non_retriable_errors:[] request_id: schedule_id:2 schedule_to_close_timeout:120 schedule_to_start_timeout:60 ` +
					`scheduled_event:[116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 99 104 101 100 117 108 101 100 45 101 118 101 110 116 45 100 97 116 97] ` +
					`scheduled_event_batch_id:0 scheduled_time:2023-12-19 22:08:41 +0000 UTC start_to_close_timeout:180 ` +
					`started_event:[116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 116 97 114 116 101 100 45 101 118 101 110 116 45 100 97 116 97] ` +
					`started_id:3 started_identity: started_time:0001-01-01 00:00:00 +0000 UTC task_list:tasklist1 timer_task_status:0 version:1` +
					`]` +
					`] , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetActivityInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.activityInfos, FixedTime)
			if err != nil {
				t.Fatalf("resetActivityInfos() error = %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateActivityInfos(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc          string
		shardID       int
		domainID      string
		workflowID    string
		runID         string
		activityInfos map[int64]*persistence.InternalActivityInfo
		deleteInfos   []int64
		// expectations
		wantQueries []string
	}{
		{
			desc:       "update and delete activity infos",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			runID:      "runid1",
			activityInfos: map[int64]*persistence.InternalActivityInfo{
				1: {
					Version: 1,
					ScheduledEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-scheduled-event-data"),
					},
					ScheduledTime: ts.UTC(),
					ScheduleID:    1,
					StartedID:     2,
					StartedEvent: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-started-event-data"),
					},
					ActivityID:             "activity1",
					ScheduleToStartTimeout: 1 * time.Minute,
					ScheduleToCloseTimeout: 2 * time.Minute,
					StartToCloseTimeout:    3 * time.Minute,
					HeartbeatTimeout:       1 * time.Minute,
					Attempt:                3,
					MaximumAttempts:        5,
					TaskList:               "tasklist1",
					TaskListKind:           types.TaskListKindEphemeral,
					HasRetryPolicy:         true,
					LastFailureReason:      "retry reason",
				},
			},
			deleteInfos: []int64{2},
			wantQueries: []string{
				`UPDATE executions SET activity_map[ 1 ] = {` +
					`version: 1, schedule_id: 1, scheduled_event_batch_id: 0, ` +
					`scheduled_event: [116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 99 104 101 100 117 108 101 100 45 101 118 101 110 116 45 100 97 116 97], ` +
					`scheduled_time: 2023-12-19T22:08:41Z, started_id: 2, ` +
					`started_event: [116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 115 116 97 114 116 101 100 45 101 118 101 110 116 45 100 97 116 97], ` +
					`started_time: 0001-01-01T00:00:00Z, activity_id: activity1, request_id: , ` +
					`details: [], schedule_to_start_timeout: 60, schedule_to_close_timeout: 120, start_to_close_timeout: 180, ` +
					`heart_beat_timeout: 60, cancel_requested: false, cancel_request_id: 0, last_hb_updated_time: 0001-01-01T00:00:00Z, ` +
					`timer_task_status: 0, attempt: 3, task_list: tasklist1, task_list_kind: 2, started_identity: , has_retry_policy: true, ` +
					`init_interval: 0, backoff_coefficient: 0, max_interval: 0, expiration_time: 0001-01-01T00:00:00Z, ` +
					`max_attempts: 5, non_retriable_errors: [], last_failure_reason: retry reason, last_worker_identity: , ` +
					`last_failure_details: [], event_data_encoding: thriftrw` +
					`} , last_updated_time = 2025-01-06T15:00:00Z WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
				`DELETE activity_map[ 2 ] FROM executions ` +
					`WHERE shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateActivityInfos(batch, tc.shardID, tc.domainID, tc.workflowID, tc.runID, tc.activityInfos, tc.deleteInfos, FixedTime)
			if err != nil {
				t.Fatalf("updateActivityInfos() error = %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateWorkflowExecutionWithMergeMaps(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.WorkflowExecutionRequest
		// expectations
		wantQueries int
		wantErr     bool
	}{
		{
			desc:       "EventBufferWriteMode is not None",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeAppend,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			wantErr: true,
		},
		{
			desc:       "MapsWriteMode is not Create",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeNone,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeUpdate,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			wantErr: true,
		},
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeNone,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeCreate,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
				ActivityInfos: map[int64]*persistence.InternalActivityInfo{
					1: {
						Version: 1,
						ScheduledEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
							Data:     []byte("thrift-encoded-scheduled-event-data"),
						},
						ScheduledTime: ts.UTC(),
						ScheduleID:    1,
						StartedID:     2,
						StartedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
							Data:     []byte("thrift-encoded-started-event-data"),
						},
						ActivityID:             "activity1",
						ScheduleToStartTimeout: 1 * time.Minute,
						ScheduleToCloseTimeout: 2 * time.Minute,
						StartToCloseTimeout:    3 * time.Minute,
						HeartbeatTimeout:       1 * time.Minute,
						Attempt:                3,
						MaximumAttempts:        5,
						TaskList:               "tasklist1",
						HasRetryPolicy:         true,
						LastFailureReason:      "retry reason",
					},
				},
				TimerInfos: map[string]*persistence.TimerInfo{
					"timer1": {
						Version:    1,
						TimerID:    "timer1",
						StartedID:  2,
						ExpiryTime: ts,
						TaskStatus: 1,
					},
				},
				ChildWorkflowInfos: map[int64]*persistence.InternalChildExecutionInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						StartedID:             3,
						StartedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
						},
						StartedWorkflowID: "startedWorkflowID1",
						StartedRunID:      "startedRunID1",
						CreateRequestID:   "createRequestID1",
						InitiatedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
						},
						DomainID:          "domain1",
						WorkflowTypeName:  "workflowType1",
						ParentClosePolicy: types.ParentClosePolicyAbandon,
					},
				},
				RequestCancelInfos: map[int64]*persistence.RequestCancelInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						CancelRequestID:       "cancelRequest1",
					},
				},
				SignalInfos: map[int64]*persistence.SignalInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						SignalRequestID:       "request1",
						SignalName:            "signal1",
						Input:                 []byte("input1"),
						Control:               []byte("control1"),
					},
				},
				SignalRequestedIDs: []string{"signalRequestedID1"},
			},
			// expecting 7 queries:
			// - 1 for execution record
			// - 1 for activity info
			// - 1 for timer info
			// - 1 for child execution info
			// - 1 for request cancel info
			// - 1 for signal info
			// - 1 for signal requested IDs
			wantQueries: 7,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := createWorkflowExecutionWithMergeMaps(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			// actual queries generated by helper functions are covered in other unit tests. check the numer of total queries here.
			if got := len(batch.queries); got != tc.wantQueries {
				t.Fatalf("len(queries): %v, want: %v", got, tc.wantQueries)
			}
		})
	}
}

func TestResetWorkflowExecutionAndMapsAndEventBuffer(t *testing.T) {
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.WorkflowExecutionRequest
		// expectations
		wantQueries int
		wantErr     bool
	}{
		{
			desc:       "EventBufferWriteMode is not Clear",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeAppend,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			wantErr: true,
		},
		{
			desc:       "MapsWriteMode is not Reset",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeUpdate, // Incorrect mode
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			wantErr: true,
		},
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeReset,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			// expecting 8 queries:
			// - 1 for execution record
			// - 1 for deletion of buffered events
			// - 1 for activity info map reset
			// - 1 for timer info map reset
			// - 1 for child execution info map reset
			// - 1 for request cancel info map reset
			// - 1 for signal info map reset
			// - 1 for signal requested IDs reset
			wantQueries: 8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := resetWorkflowExecutionAndMapsAndEventBuffer(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			// Check the number of total queries, actual queries generated by helper functions are covered in other unit tests.
			if got := len(batch.queries); got != tc.wantQueries {
				t.Fatalf("len(queries): %v, want: %v", got, tc.wantQueries)
			}
		})
	}
}

func TestUpdateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.WorkflowExecutionRequest
		// expectations
		wantQueries int
		wantErr     bool
	}{
		{
			desc:       "MapsWriteMode is not Update",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeCreate, // Incorrect mode
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
			},
			wantErr: true,
		},
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeUpdate,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					CompletionEvent: &persistence.DataBlob{},
					AutoResetPoints: &persistence.DataBlob{},
				},
				VersionHistories: &persistence.DataBlob{},
				Checksums:        &checksum.Checksum{},
				ActivityInfos: map[int64]*persistence.InternalActivityInfo{
					1: {
						Version: 1,
						ScheduledEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
							Data:     []byte("thrift-encoded-scheduled-event-data"),
						},
						ScheduledTime: ts.UTC(),
						ScheduleID:    1,
						StartedID:     2,
						StartedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
							Data:     []byte("thrift-encoded-started-event-data"),
						},
						ActivityID:             "activity1",
						ScheduleToStartTimeout: 1 * time.Minute,
						ScheduleToCloseTimeout: 2 * time.Minute,
						StartToCloseTimeout:    3 * time.Minute,
						HeartbeatTimeout:       1 * time.Minute,
						Attempt:                3,
						MaximumAttempts:        5,
						TaskList:               "tasklist1",
						HasRetryPolicy:         true,
						LastFailureReason:      "retry reason",
					},
				},
				TimerInfos: map[string]*persistence.TimerInfo{
					"timer1": {
						Version:    1,
						TimerID:    "timer1",
						StartedID:  2,
						ExpiryTime: ts.UTC(),
						TaskStatus: 1,
					},
				},
				ChildWorkflowInfos: map[int64]*persistence.InternalChildExecutionInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						StartedID:             3,
						StartedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
						},
						StartedWorkflowID: "startedWorkflowID1",
						StartedRunID:      "startedRunID1",
						CreateRequestID:   "createRequestID1",
						InitiatedEvent: &persistence.DataBlob{
							Encoding: constants.EncodingTypeThriftRW,
						},
						DomainID:          "domain1",
						WorkflowTypeName:  "workflowType1",
						ParentClosePolicy: types.ParentClosePolicyAbandon,
					},
				},
				RequestCancelInfos: map[int64]*persistence.RequestCancelInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						CancelRequestID:       "cancelRequest1",
					},
				},
				SignalInfos: map[int64]*persistence.SignalInfo{
					1: {
						Version:               1,
						InitiatedID:           1,
						InitiatedEventBatchID: 2,
						SignalRequestID:       "request1",
						SignalName:            "signal1",
						Input:                 []byte("input1"),
						Control:               []byte("control1"),
					},
				},
				SignalRequestedIDs: []string{"signalRequestedID1"},
			},
			// expecting 8 queries:
			// - 1 for execution record
			// - 1 for deletion of buffered events
			// - 1 for activity info map update
			// - 1 for timer info map update
			// - 1 for child execution info map update
			// - 1 for request cancel info map update
			// - 1 for signal info map update
			// - 1 for signal requested IDs update
			wantQueries: 8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			// Check the number of total queries, actual queries generated by helper functions are covered in other unit tests.
			if got := len(batch.queries); got != tc.wantQueries {
				t.Fatalf("len(queries): %v, want: %v", got, tc.wantQueries)
			}
		})
	}
}

func TestUpdateWorkflowExecution(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.WorkflowExecutionRequest
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeUpdate,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					DomainID:             "domain1",
					WorkflowID:           "workflow1",
					RunID:                "runid1",
					ParentRunID:          "parentRunID1",
					WorkflowTypeName:     "workflowType1",
					TaskList:             "tasklist1",
					TaskListKind:         types.TaskListKindNormal,
					StartTimestamp:       ts,
					LastUpdatedTimestamp: ts.Add(1 * time.Minute),
					DecisionScheduleID:   2,
					DecisionStartedID:    3,
					CompletionEvent:      &persistence.DataBlob{},
					AutoResetPoints:      &persistence.DataBlob{},
					CronOverlapPolicy:    0,
				},
				PreviousNextEventIDCondition: common.Int64Ptr(10),
				VersionHistories:             &persistence.DataBlob{},
				Checksums:                    &checksum.Checksum{},
			},
			wantQueries: []string{
				`UPDATE executions SET execution = {` +
					`domain_id: domain1, workflow_id: workflow1, run_id: runid1, first_run_id: , parent_domain_id: , parent_workflow_id: , ` +
					`parent_run_id: parentRunID1, initiated_id: 0, completion_event_batch_id: 0, completion_event: [], ` +
					`completion_event_data_encoding: , task_list: tasklist1, task_list_kind: 0, workflow_type_name: workflowType1, workflow_timeout: 0, ` +
					`decision_task_timeout: 0, execution_context: [], state: 0, close_status: 0, last_first_event_id: 0, last_event_task_id: 0, ` +
					`next_event_id: 0, last_processed_event: 0, start_time: 2023-12-19T22:08:41Z, last_updated_time: 2023-12-19T22:09:41Z, ` +
					`create_request_id: , signal_count: 0, history_size: 0, decision_version: 0, decision_schedule_id: 2, decision_started_id: 3, ` +
					`decision_request_id: , decision_timeout: 0, decision_attempt: 0, decision_timestamp: -6795364578871345152, ` +
					`decision_scheduled_timestamp: -6795364578871345152, decision_original_scheduled_timestamp: -6795364578871345152, ` +
					`cancel_requested: false, cancel_request_id: , sticky_task_list: , sticky_schedule_to_start_timeout: 0,client_library_version: , ` +
					`client_feature_version: , client_impl: , auto_reset_points: [], auto_reset_points_encoding: , attempt: 0, has_retry_policy: false, ` +
					`init_interval: 0, backoff_coefficient: 0, max_interval: 0, expiration_time: 0001-01-01T00:00:00Z, max_attempts: 0, ` +
					`non_retriable_errors: [], event_store_version: 2, branch_token: [], cron_schedule: , cron_overlap_policy: 0, expiration_seconds: 0, search_attributes: map[], ` +
					`memo: map[], partition_config: map[], active_cluster_selection_policy: [], active_cluster_selection_policy_encoding: ` +
					`}, next_event_id = 0 , version_histories = [] , version_histories_encoding =  , checksum = {version: 0, flavor: 0, value: [] }, workflow_last_write_version = 0 , workflow_state = 0 , last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = runid1 and visibility_ts = 946684800000 and task_id = -10 ` +
					`IF next_event_id = 10 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := updateWorkflowExecution(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			if err != nil {
				t.Fatalf("updateWorkflowExecution failed, err: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateWorkflowExecution(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2023-12-19T22:08:41Z")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.WorkflowExecutionRequest
		// expectations
		wantQueries []string
	}{
		{
			desc:       "ok",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.WorkflowExecutionRequest{
				EventBufferWriteMode: nosqlplugin.EventBufferWriteModeClear,
				MapsWriteMode:        nosqlplugin.WorkflowExecutionMapsWriteModeUpdate,
				InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
					DomainID:             "domain1",
					WorkflowID:           "workflow1",
					RunID:                "runid1",
					ParentRunID:          "parentRunID1",
					WorkflowTypeName:     "workflowType1",
					TaskList:             "tasklist1",
					TaskListKind:         types.TaskListKindNormal,
					StartTimestamp:       ts,
					LastUpdatedTimestamp: ts.Add(1 * time.Minute),
					DecisionScheduleID:   2,
					DecisionStartedID:    3,
					CompletionEvent:      &persistence.DataBlob{},
					AutoResetPoints:      &persistence.DataBlob{},
					ActiveClusterSelectionPolicy: &persistence.DataBlob{
						Encoding: constants.EncodingTypeThriftRW,
						Data:     []byte("thrift-encoded-active-cluster-selection-policy-data"),
					},
					CronOverlapPolicy: types.CronOverlapPolicyBufferOne,
				},
				PreviousNextEventIDCondition: common.Int64Ptr(10),
				VersionHistories:             &persistence.DataBlob{},
				Checksums:                    &checksum.Checksum{},
			},
			wantQueries: []string{
				`INSERT INTO executions (shard_id, domain_id, workflow_id, run_id, type, execution, next_event_id, visibility_ts, task_id, version_histories, version_histories_encoding, checksum, workflow_last_write_version, workflow_state, created_time) ` +
					`VALUES(1000, domain1, workflow1, runid1, 1, ` +
					`{domain_id: domain1, workflow_id: workflow1, run_id: runid1, first_run_id: , parent_domain_id: , parent_workflow_id: , ` +
					`parent_run_id: parentRunID1, initiated_id: 0, completion_event_batch_id: 0, completion_event: [], completion_event_data_encoding: , ` +
					`task_list: tasklist1, task_list_kind: 0, workflow_type_name: workflowType1, workflow_timeout: 0, decision_task_timeout: 0, execution_context: [], state: 0, ` +
					`close_status: 0, last_first_event_id: 0, last_event_task_id: 0, next_event_id: 0, last_processed_event: 0, start_time: 2023-12-19T22:08:41Z, ` +
					`last_updated_time: 2023-12-19T22:09:41Z, create_request_id: , signal_count: 0, history_size: 0, decision_version: 0, ` +
					`decision_schedule_id: 2, decision_started_id: 3, decision_request_id: , decision_timeout: 0, decision_attempt: 0, ` +
					`decision_timestamp: -6795364578871345152, decision_scheduled_timestamp: -6795364578871345152, decision_original_scheduled_timestamp: -6795364578871345152, ` +
					`cancel_requested: false, cancel_request_id: , sticky_task_list: , sticky_schedule_to_start_timeout: 0,client_library_version: , client_feature_version: , ` +
					`client_impl: , auto_reset_points: [], auto_reset_points_encoding: , attempt: 0, has_retry_policy: false, init_interval: 0, ` +
					`backoff_coefficient: 0, max_interval: 0, expiration_time: 0001-01-01T00:00:00Z, max_attempts: 0, non_retriable_errors: [], ` +
					`event_store_version: 2, branch_token: [], cron_schedule: , cron_overlap_policy: 1, expiration_seconds: 0, search_attributes: map[], memo: map[], partition_config: map[], ` +
					`active_cluster_selection_policy: [116 104 114 105 102 116 45 101 110 99 111 100 101 100 45 97 99 116 105 118 101 45 99 108 117 115 116 101 114 45 115 101 108 101 99 116 105 111 110 45 112 111 108 105 99 121 45 100 97 116 97], active_cluster_selection_policy_encoding: thriftrw` +
					`}, 0, 946684800000, -10, [], , {version: 0, flavor: 0, value: [] }, 0, 0, 2025-01-06T15:00:00Z) IF NOT EXISTS `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := createWorkflowExecution(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			if err != nil {
				t.Fatalf("createWorkflowExecution failed, err: %v", err)
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateOrUpdateWorkflowExecution(t *testing.T) {
	tests := []struct {
		desc       string
		shardID    int
		domainID   string
		workflowID string
		execution  *nosqlplugin.CurrentWorkflowWriteRequest
		// expectations
		wantQueries []string
		wantErr     bool
	}{
		{
			desc:       "unknown write mode",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: 255, // unknown write mode
			},
			wantErr: true,
		},
		{
			desc:       "CurrentWorkflowWriteModeNoop",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			wantQueries: nil,
		},
		{
			desc:       "CurrentWorkflowWriteModeInsert",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeInsert,
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID:           "runid1",
					CreateRequestID: "createRequestID1",
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
				},
			},
			wantQueries: []string{
				`INSERT INTO executions (shard_id, type, domain_id, workflow_id, run_id, visibility_ts, task_id, current_run_id, execution, workflow_last_write_version, workflow_state, created_time) ` +
					`VALUES(1000, 1, domain1, workflow1, 30000000-0000-f000-f000-000000000001, 946684800000, -10, runid1, ` +
					`{run_id: runid1, create_request_id: createRequestID1, state: 0, close_status: 0}, 0, 0, 2025-01-06T15:00:00Z) ` +
					`IF NOT EXISTS USING TTL 0 `,
			},
		},
		{
			desc:       "CurrentWorkflowWriteModeUpdate and condition missing",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID:           "runid1",
					CreateRequestID: "createRequestID1",
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
				},
			},
			wantErr: true,
		},
		{
			desc:       "CurrentWorkflowWriteModeUpdate and condition runid missing",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: nil,
				},
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID:           "runid1",
					CreateRequestID: "createRequestID1",
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
				},
			},
			wantErr: true,
		},
		{
			desc:       "CurrentWorkflowWriteModeUpdate with LastWriteVersion",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID:     common.StringPtr("runid1"),
					LastWriteVersion: common.Int64Ptr(1),
					State:            common.IntPtr(persistence.WorkflowStateCreated),
				},
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID:           "runid1",
					CreateRequestID: "createRequestID1",
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
				},
			},
			wantQueries: []string{
				`UPDATE executions USING TTL 0 SET ` +
					`current_run_id = runid1, ` +
					`execution = {run_id: runid1, create_request_id: createRequestID1, state: 0, close_status: 0}, ` +
					`workflow_last_write_version = 0, workflow_state = 0, last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = 30000000-0000-f000-f000-000000000001 and visibility_ts = 946684800000 and task_id = -10 ` +
					`IF current_run_id = runid1 and workflow_last_write_version = 1 and workflow_state = 0 `,
			},
		},
		{
			desc:       "CurrentWorkflowWriteModeUpdate",
			shardID:    1000,
			domainID:   "domain1",
			workflowID: "workflow1",
			execution: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: common.StringPtr("runid1"),
				},
				Row: nosqlplugin.CurrentWorkflowRow{
					RunID:           "runid1",
					CreateRequestID: "createRequestID1",
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
				},
			},
			wantQueries: []string{
				`UPDATE executions USING TTL 0 SET ` +
					`current_run_id = runid1, ` +
					`execution = {run_id: runid1, create_request_id: createRequestID1, state: 0, close_status: 0}, ` +
					`workflow_last_write_version = 0, workflow_state = 0, last_updated_time = 2025-01-06T15:00:00Z ` +
					`WHERE ` +
					`shard_id = 1000 and type = 1 and domain_id = domain1 and workflow_id = workflow1 and ` +
					`run_id = 30000000-0000-f000-f000-000000000001 and visibility_ts = 946684800000 and task_id = -10 ` +
					`IF current_run_id = runid1 `,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			batch := &fakeBatch{}

			err := createOrUpdateCurrentWorkflow(batch, tc.shardID, tc.domainID, tc.workflowID, tc.execution, FixedTime)
			gotErr := (err != nil)
			if gotErr != tc.wantErr {
				t.Fatalf("Got error: %v, want?: %v", err, tc.wantErr)
			}
			if gotErr {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, batch.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMustConvertToSlice(t *testing.T) {
	tests := []struct {
		desc      string
		in        interface{}
		want      []interface{}
		wantPanic bool
	}{
		{
			desc:      "nil",
			in:        nil,
			wantPanic: true,
		},
		{
			desc: "empty",
			in:   []string{},
			want: []interface{}{},
		},
		{
			desc: "slice",
			in:   []string{"a", "b", "c"},
			want: []interface{}{"a", "b", "c"},
		},
		{
			desc: "array",
			in:   [3]string{"a", "b", "c"},
			want: []interface{}{"a", "b", "c"},
		},
		{
			desc:      "non-slice",
			in:        "a",
			wantPanic: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tc.wantPanic {
					t.Fatalf("Got panic: %v, want panic?: %v", r, tc.wantPanic)
				}
			}()

			got := mustConvertToSlice(tc.in)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatalf("Slice mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsRequestRowType(t *testing.T) {
	assert.True(t, isRequestRowType(rowTypeWorkflowRequestStart))
	assert.True(t, isRequestRowType(rowTypeWorkflowRequestSignal))
	assert.True(t, isRequestRowType(rowTypeWorkflowRequestCancel))
	assert.True(t, isRequestRowType(rowTypeWorkflowRequestReset))
}

func errDiff(want, got error) string {
	wantCondFailure, wantOk := want.(*nosqlplugin.WorkflowOperationConditionFailure)
	gotCondFailure, gotOk := got.(*nosqlplugin.WorkflowOperationConditionFailure)
	if wantOk && gotOk {
		arg1 := trimWorkflowConditionalFailureErr(wantCondFailure)
		arg2 := trimWorkflowConditionalFailureErr(gotCondFailure)
		return cmp.Diff(arg1, arg2)
	}

	wantMsg := ""
	if want != nil {
		wantMsg = want.Error()
	}
	gotMsg := ""
	if got != nil {
		gotMsg = got.Error()
	}
	return cmp.Diff(wantMsg, gotMsg)
}

func trimWorkflowConditionalFailureErr(condFailure *nosqlplugin.WorkflowOperationConditionFailure) any {
	trimColumnsPart(condFailure.CurrentWorkflowConditionFailInfo)
	trimColumnsPart(condFailure.UnknownConditionFailureDetails)
	if condFailure.WorkflowExecutionAlreadyExists != nil {
		trimColumnsPart(&condFailure.WorkflowExecutionAlreadyExists.OtherInfo)
	}
	return condFailure
}

func trimColumnsPart(s *string) {
	if s == nil {
		return
	}
	re := regexp.MustCompile(`, columns: \(.*\)`)
	trimmed := re.ReplaceAllString(*s, "")
	*s = trimmed
}
