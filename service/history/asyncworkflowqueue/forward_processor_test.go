package asyncworkflowqueue

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestForwardProcessor_Process(t *testing.T) {
	errRemote := errors.New("remote failed")
	errBean := errors.New("no client for cluster")

	// Deliberately written in non-sorted order; the processor must replay them sorted by key.
	unsortedHeaders := map[string][]byte{"b-key": []byte("2"), "a-key": []byte("1"), "c-key": []byte("3")}
	sortedOpts := []yarpc.CallOption{
		yarpc.WithHeader("a-key", "1"),
		yarpc.WithHeader("b-key", "2"),
		yarpc.WithHeader("c-key", "3"),
	}

	startReq := func(cluster string, hdr map[string][]byte) *Request {
		return &Request{
			DomainID:      "domain-id-1",
			ActiveCluster: cluster,
			Headers:       hdr,
			Start:         &types.StartWorkflowExecutionRequest{Domain: "d", WorkflowID: "wf-1", RequestID: "req-1"},
		}
	}
	signalReq := func(cluster string, hdr map[string][]byte) *Request {
		return &Request{
			DomainID:        "domain-id-1",
			ActiveCluster:   cluster,
			Headers:         hdr,
			SignalWithStart: &types.SignalWithStartWorkflowExecutionRequest{Domain: "d", WorkflowID: "wf-2", RequestID: "req-2", SignalName: "sig"},
		}
	}

	tests := []struct {
		name          string
		req           *Request
		beanErr       error
		remoteErr     error
		noBeanCall    bool
		noRemoteCall  bool
		wantOpts      []yarpc.CallOption // exact options the remote call must receive, in order
		wantErr       error
		wantInternal  bool // expect a *types.InternalServiceError
		wantForwarded int64
	}{
		{
			name:          "start forwarded with sorted headers replayed",
			req:           startReq("cluster-b", unsortedHeaders),
			wantOpts:      sortedOpts,
			wantForwarded: 1,
		},
		{
			name:          "signal with start forwarded with sorted headers replayed",
			req:           signalReq("cluster-b", unsortedHeaders),
			wantOpts:      sortedOpts,
			wantForwarded: 1,
		},
		{
			name:          "start forwarded with nil headers passes no options",
			req:           startReq("cluster-b", nil),
			wantForwarded: 1,
		},
		{
			name:          "signal with start forwarded with empty headers passes no options",
			req:           signalReq("cluster-b", map[string][]byte{}),
			wantForwarded: 1,
		},
		{
			name:      "start remote error returned unchanged and nothing counted",
			req:       startReq("cluster-b", unsortedHeaders),
			remoteErr: errRemote,
			wantOpts:  sortedOpts,
			wantErr:   errRemote,
		},
		{
			name:      "signal with start remote error returned unchanged and nothing counted",
			req:       signalReq("cluster-b", unsortedHeaders),
			remoteErr: errRemote,
			wantOpts:  sortedOpts,
			wantErr:   errRemote,
		},
		{
			name:         "no remote client for cluster returns the bean error",
			req:          startReq("cluster-x", unsortedHeaders),
			beanErr:      errBean,
			noRemoteCall: true,
			wantErr:      errBean,
		},
		{
			name:         "empty ActiveCluster is an internal service error and makes no calls",
			req:          startReq("", unsortedHeaders),
			noBeanCall:   true,
			noRemoteCall: true,
			wantInternal: true,
		},
		{
			name:         "neither payload set is an internal service error and makes no frontend call",
			req:          &Request{DomainID: "domain-id-1", ActiveCluster: "cluster-b"},
			noRemoteCall: true,
			wantInternal: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bean := client.NewMockBean(ctrl)
			fe := frontend.NewMockClient(ctrl)

			if !tc.noBeanCall {
				// For the "neither payload" case the bean lookup may happen before or after the payload check.
				call := bean.EXPECT().GetRemoteFrontendClient(tc.req.ActiveCluster)
				if tc.beanErr != nil {
					call.Return(nil, tc.beanErr)
				} else {
					call.Return(fe, nil)
				}
				if tc.req.Start == nil && tc.req.SignalWithStart == nil {
					call.AnyTimes()
				}
			}

			if !tc.noRemoteCall {
				// gomock matches variadic options positionally, so the expected options are passed as matchers.
				var optMatchers []any
				for _, o := range tc.wantOpts {
					optMatchers = append(optMatchers, o)
				}
				assertOpts := func(opts []yarpc.CallOption) {
					if len(tc.wantOpts) == 0 {
						require.Empty(t, opts)
						return
					}
					require.Equal(t, tc.wantOpts, opts)
				}
				if tc.req.Start != nil {
					fe.EXPECT().StartWorkflowExecutionAsync(gomock.Any(), gomock.Any(), optMatchers...).DoAndReturn(
						func(_ context.Context, r *types.StartWorkflowExecutionAsyncRequest, opts ...yarpc.CallOption) (*types.StartWorkflowExecutionAsyncResponse, error) {
							require.Equal(t, "wf-1", r.StartWorkflowExecutionRequest.WorkflowID)
							assertOpts(opts)
							if tc.remoteErr != nil {
								return nil, tc.remoteErr
							}
							return &types.StartWorkflowExecutionAsyncResponse{}, nil
						})
				} else {
					fe.EXPECT().SignalWithStartWorkflowExecutionAsync(gomock.Any(), gomock.Any(), optMatchers...).DoAndReturn(
						func(_ context.Context, r *types.SignalWithStartWorkflowExecutionAsyncRequest, opts ...yarpc.CallOption) (*types.SignalWithStartWorkflowExecutionAsyncResponse, error) {
							require.Equal(t, "sig", r.SignalWithStartWorkflowExecutionRequest.SignalName)
							assertOpts(opts)
							if tc.remoteErr != nil {
								return nil, tc.remoteErr
							}
							return &types.SignalWithStartWorkflowExecutionAsyncResponse{}, nil
						})
				}
			}

			ts, scope := newTestMetrics()
			p := NewForwardProcessor(bean, scope, testlogger.New(t))
			err := p.Process(context.Background(), tc.req)

			switch {
			case tc.wantInternal:
				var ise *types.InternalServiceError
				require.ErrorAs(t, err, &ise)
			case tc.wantErr != nil:
				require.Equal(t, tc.wantErr, err)
			default:
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantForwarded, counterTotal(ts.Snapshot(), "async_workflow_queue_forwarded"))
		})
	}
}
