package asyncworkflowqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine"
)

func counterTotal(s tally.Snapshot, name string) int64 {
	var n int64
	for _, c := range s.Counters() {
		if c.Name() == name {
			n += c.Value()
		}
	}
	return n
}

// timerCount counts recorded timer values for name. A domain-tagged metrics scope dual-emits every timer
// (once under the domain tag and once under domain=all), so the aggregate series is skipped to count each
// recording exactly once.
func timerCount(s tally.Snapshot, name string) int {
	n := 0
	for _, tm := range s.Timers() {
		if tm.Name() == name && tm.Tags()["domain"] != "all" {
			n += len(tm.Values())
		}
	}
	return n
}

func newTestMetrics() (tally.TestScope, metrics.Scope) {
	ts := tally.NewTestScope("", make(map[string]string))
	c := metrics.NewClient(ts, metrics.History, metrics.MigrationConfig{})
	return ts, c.Scope(metrics.AsyncWorkflowQueueConsumerScope)
}

func TestActiveProcessor_Process(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	errBoom := errors.New("boom")

	startReq := func() *Request {
		return &Request{
			DomainID: "domain-id-1",
			Start: &types.StartWorkflowExecutionRequest{
				Domain: "test-domain", WorkflowID: "wf-1", RequestID: "req-1",
			},
		}
	}
	signalReq := func() *Request {
		return &Request{
			DomainID: "domain-id-1",
			SignalWithStart: &types.SignalWithStartWorkflowExecutionRequest{
				Domain: "test-domain", WorkflowID: "wf-2", RequestID: "req-2", SignalName: "sig",
			},
		}
	}

	tests := []struct {
		name               string
		req                func() *Request
		expect             func(t *testing.T, e *engine.MockEngine)
		wantErr            error
		wantSuccess        int64
		wantAlreadyStarted int64
		wantTimers         int
	}{
		{
			name: "start success",
			req:  startReq,
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Equal(t, "domain-id-1", r.DomainUUID)
						require.Equal(t, "wf-1", r.StartRequest.WorkflowID)
						require.Nil(t, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-1"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "start already started is success",
			req:  startReq,
			expect: func(_ *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{Message: "dup"})
			},
			wantAlreadyStarted: 1,
			wantTimers:         1,
		},
		{
			name: "start error passes through unchanged",
			req:  startReq,
			expect: func(_ *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errBoom)
			},
			wantErr:    errBoom,
			wantTimers: 1,
		},
		{
			name: "start DomainNotActive passes through unchanged",
			req:  startReq,
			expect: func(_ *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.DomainNotActiveError{ActiveCluster: "cluster-b"})
			},
			wantErr:    &types.DomainNotActiveError{ActiveCluster: "cluster-b"},
			wantTimers: 1,
		},
		{
			name: "signal with start success",
			req:  signalReq,
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistorySignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Equal(t, "domain-id-1", r.DomainUUID)
						require.Equal(t, "sig", r.SignalWithStartRequest.SignalName)
						require.Nil(t, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-2"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "start with client isolation group header sets partition config",
			req: func() *Request {
				r := startReq()
				r.Headers = map[string][]byte{common.ClientIsolationGroupHeaderName: []byte("zone-1")}
				return r
			},
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Equal(t, map[string]string{isolationgroup.GroupKey: "zone-1"}, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-1"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "start with unrelated headers only has nil partition config",
			req: func() *Request {
				r := startReq()
				r.Headers = map[string][]byte{"other": []byte("x")}
				return r
			},
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Nil(t, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-1"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "start with empty isolation group header has nil partition config",
			req: func() *Request {
				r := startReq()
				r.Headers = map[string][]byte{common.ClientIsolationGroupHeaderName: {}}
				return r
			},
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistoryStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Nil(t, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-1"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "signal with start with client isolation group header sets partition config",
			req: func() *Request {
				r := signalReq()
				r.Headers = map[string][]byte{common.ClientIsolationGroupHeaderName: []byte("zone-2")}
				return r
			},
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistorySignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Equal(t, map[string]string{isolationgroup.GroupKey: "zone-2"}, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-2"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "signal with start without header has nil partition config",
			req:  signalReq,
			expect: func(t *testing.T, e *engine.MockEngine) {
				e.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *types.HistorySignalWithStartWorkflowExecutionRequest) (*types.StartWorkflowExecutionResponse, error) {
						require.Nil(t, r.PartitionConfig)
						return &types.StartWorkflowExecutionResponse{RunID: "run-2"}, nil
					})
			},
			wantSuccess: 1,
			wantTimers:  1,
		},
		{
			name: "signal with start already started is success",
			req:  signalReq,
			expect: func(_ *testing.T, e *engine.MockEngine) {
				e.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{})
			},
			wantAlreadyStarted: 1,
			wantTimers:         1,
		},
		{
			name: "signal with start error passes through unchanged",
			req:  signalReq,
			expect: func(_ *testing.T, e *engine.MockEngine) {
				e.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errBoom)
			},
			wantErr:    errBoom,
			wantTimers: 1,
		},
		{
			name:   "neither payload set is an internal service error and never reaches the engine",
			req:    func() *Request { return &Request{DomainID: "domain-id-1"} },
			expect: func(*testing.T, *engine.MockEngine) {},
			wantErr: &types.InternalServiceError{
				Message: "async workflow queue request has neither start nor signal-with-start payload",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			eng := engine.NewMockEngine(ctrl)
			tc.expect(t, eng)
			ts, scope := newTestMetrics()
			timeSource := clock.NewMockedTimeSourceAt(now)

			p := NewActiveProcessor(func() engine.Engine { return eng }, timeSource, scope, testlogger.New(t))
			err := p.Process(context.Background(), tc.req())

			if tc.wantErr != nil {
				require.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
			}
			snap := ts.Snapshot()
			require.Equal(t, tc.wantSuccess, counterTotal(snap, "async_workflow_queue_success"))
			require.Equal(t, tc.wantAlreadyStarted, counterTotal(snap, "async_workflow_queue_already_started"))
			require.Equal(t, tc.wantTimers, timerCount(snap, "async_workflow_queue_process_latency"))
		})
	}
}
