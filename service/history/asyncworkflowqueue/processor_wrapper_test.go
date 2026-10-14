package asyncworkflowqueue

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestProcessorWrapper_Process(t *testing.T) {
	const (
		current  = "cluster-a"
		other    = "cluster-b"
		domainID = "domain-id-1"
	)
	attr := &types.ClusterAttribute{Scope: "region", Name: "us"}
	policy := &types.ActiveClusterSelectionPolicy{ClusterAttribute: attr}

	newStartReq := func() *Request {
		return &Request{
			DomainID: domainID,
			Start:    &types.StartWorkflowExecutionRequest{Domain: "d", WorkflowID: "wf", ActiveClusterSelectionPolicy: policy},
		}
	}
	newSignalReq := func() *Request {
		return &Request{
			DomainID:        domainID,
			SignalWithStart: &types.SignalWithStartWorkflowExecutionRequest{Domain: "d", WorkflowID: "wf", ActiveClusterSelectionPolicy: policy},
		}
	}
	// No policy at all: the lookup must still be made, with a nil attribute (domain default).
	newNoPolicyReq := func() *Request {
		return &Request{DomainID: domainID, Start: &types.StartWorkflowExecutionRequest{Domain: "d", WorkflowID: "wf"}}
	}

	errActive := errors.New("active failed")
	errForward := errors.New("forward failed")

	lookupReturns := func(attribute *types.ClusterAttribute, cluster string) func(m *activecluster.MockManager) {
		return func(m *activecluster.MockManager) {
			m.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), domainID, attribute).
				Return(&types.ActiveClusterInfo{ActiveClusterName: cluster}, nil)
		}
	}
	activeReturns := func(err error) func(a *MockProcessor) {
		return func(a *MockProcessor) {
			a.EXPECT().Process(gomock.Any(), gomock.Any()).Return(err)
		}
	}
	nothing := func(*MockProcessor) {}
	// notActiveExpects asserts the request carries wantCluster when the not-active processor is called.
	notActiveExpects := func(wantCluster string, ret error) func(n *MockProcessor) {
		return func(n *MockProcessor) {
			n.EXPECT().Process(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, r *Request) error {
					require.Equal(t, wantCluster, r.ActiveCluster, "ActiveCluster must be set before delegating")
					return ret
				}).Times(1)
		}
	}

	tests := []struct {
		name        string
		req         func() *Request
		lookup      func(m *activecluster.MockManager)
		setupActive func(a *MockProcessor)
		setupNot    func(n *MockProcessor)
		wantErr     error
		wantCluster string // expected req.ActiveCluster after Process
	}{
		{
			name:        "start: active cluster is current, delegates to active only",
			req:         newStartReq,
			lookup:      lookupReturns(attr, current),
			setupActive: activeReturns(nil),
			setupNot:    nothing,
		},
		{
			name:        "signal with start: uses its own policy attribute",
			req:         newSignalReq,
			lookup:      lookupReturns(attr, current),
			setupActive: activeReturns(nil),
			setupNot:    nothing,
		},
		{
			name:        "no policy: nil attribute lookup, active here",
			req:         newNoPolicyReq,
			lookup:      lookupReturns(nil, current),
			setupActive: activeReturns(nil),
			setupNot:    nothing,
		},
		{
			name:        "active error is returned unchanged",
			req:         newStartReq,
			lookup:      lookupReturns(attr, current),
			setupActive: activeReturns(errActive),
			setupNot:    nothing,
			wantErr:     errActive,
		},
		{
			name:        "active cluster elsewhere: sets ActiveCluster and delegates to not-active only",
			req:         newStartReq,
			lookup:      lookupReturns(attr, other),
			setupActive: nothing,
			setupNot:    notActiveExpects(other, nil),
			wantCluster: other,
		},
		{
			name:        "active cluster elsewhere: stamps forwarded-from header with the current cluster",
			req:         newStartReq,
			lookup:      lookupReturns(attr, other),
			setupActive: nothing,
			setupNot: func(n *MockProcessor) {
				n.EXPECT().Process(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, r *Request) error {
						require.Equal(t, []byte(current), r.Headers[ForwardedFromHeaderName])
						require.Equal(t, other, r.ActiveCluster)
						return nil
					}).Times(1)
			},
			wantCluster: other,
		},
		{
			name: "already forwarded once: not delegated again, non-transient DomainNotActive returned",
			req: func() *Request {
				r := newStartReq()
				r.Message = &persistence.AsyncWorkflowMessage{DomainName: "d", WorkflowID: "wf"}
				r.Headers = map[string][]byte{ForwardedFromHeaderName: []byte(other)}
				return r
			},
			lookup:      lookupReturns(attr, other),
			setupActive: nothing,
			setupNot:    nothing,
			wantErr: &types.DomainNotActiveError{
				Message: "async workflow request for domain " + domainID + " already forwarded from " + other +
					"; active cluster is now " + other,
				DomainName:     "d",
				CurrentCluster: current,
				ActiveCluster:  other,
			},
		},
		{
			name:        "signal with start, active cluster elsewhere: delegates to not-active",
			req:         newSignalReq,
			lookup:      lookupReturns(attr, other),
			setupActive: nothing,
			setupNot:    notActiveExpects(other, nil),
			wantCluster: other,
		},
		{
			name:        "not-active error is returned unchanged",
			req:         newStartReq,
			lookup:      lookupReturns(attr, other),
			setupActive: nothing,
			setupNot:    notActiveExpects(other, errForward),
			wantErr:     errForward,
			wantCluster: other,
		},
		{
			name: "lookup error is treated as active",
			req:  newStartReq,
			lookup: func(m *activecluster.MockManager) {
				m.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), domainID, attr).
					Return(nil, errors.New("cluster attribute not found"))
			},
			setupActive: activeReturns(nil),
			setupNot:    nothing,
		},
		{
			name:   "DomainNotActive from active falls back to not-active exactly once",
			req:    newStartReq,
			lookup: lookupReturns(attr, current),
			setupActive: activeReturns(
				&types.DomainNotActiveError{DomainName: "d", CurrentCluster: current, ActiveCluster: other}),
			setupNot:    notActiveExpects(other, nil),
			wantCluster: other,
		},
		{
			name:   "DomainNotActive fallback propagates the not-active error",
			req:    newStartReq,
			lookup: lookupReturns(attr, current),
			setupActive: activeReturns(
				&types.DomainNotActiveError{DomainName: "d", CurrentCluster: current, ActiveCluster: other}),
			setupNot:    notActiveExpects(other, errForward),
			wantErr:     errForward,
			wantCluster: other,
		},
		{
			name:        "DomainNotActive without ActiveCluster is returned as is",
			req:         newStartReq,
			lookup:      lookupReturns(attr, current),
			setupActive: activeReturns(&types.DomainNotActiveError{DomainName: "d", CurrentCluster: current}),
			setupNot:    nothing,
			wantErr:     &types.DomainNotActiveError{DomainName: "d", CurrentCluster: current},
		},
		{
			name:   "DomainNotActive naming the current cluster is returned as is (no self-forward)",
			req:    newStartReq,
			lookup: lookupReturns(attr, current),
			setupActive: activeReturns(
				&types.DomainNotActiveError{DomainName: "d", CurrentCluster: current, ActiveCluster: current}),
			setupNot: nothing,
			wantErr:  &types.DomainNotActiveError{DomainName: "d", CurrentCluster: current, ActiveCluster: current},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := activecluster.NewMockManager(ctrl)
			active := NewMockProcessor(ctrl)
			notActive := NewMockProcessor(ctrl)

			tc.lookup(mgr)
			tc.setupActive(active)
			tc.setupNot(notActive)

			p := NewProcessorWrapper(current, mgr, active, notActive, testlogger.New(t))
			req := tc.req()
			err := p.Process(context.Background(), req)

			if tc.wantErr != nil {
				require.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantCluster, req.ActiveCluster)
		})
	}
}
