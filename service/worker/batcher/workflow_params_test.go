package batcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func TestBatchTypeFromOperation(t *testing.T) {
	tests := map[string]struct {
		op   *types.BatchOperation
		want string
	}{
		"nil operation": {
			op:   nil,
			want: "",
		},
		"unset operation type": {
			op:   &types.BatchOperation{},
			want: "",
		},
		"terminate": {
			op:   &types.BatchOperation{OperationType: types.BatchOperationTypeTerminate.Ptr()},
			want: BatchTypeTerminate,
		},
		"cancel": {
			op:   &types.BatchOperation{OperationType: types.BatchOperationTypeCancel.Ptr()},
			want: BatchTypeCancel,
		},
		"signal": {
			op:   &types.BatchOperation{OperationType: types.BatchOperationTypeSignal.Ptr()},
			want: BatchTypeSignal,
		},
		"replicate": {
			op:   &types.BatchOperation{OperationType: types.BatchOperationTypeReplicate.Ptr()},
			want: BatchTypeReplicate,
		},
		"refresh": {
			op:   &types.BatchOperation{OperationType: types.BatchOperationTypeRefresh.Ptr()},
			want: BatchTypeRefresh,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, BatchTypeFromOperation(tt.op))
		})
	}
}

func TestNormalizeOperation(t *testing.T) {
	tests := map[string]struct {
		params BatchParams
		want   BatchParams
	}{
		"nil operation leaves CLI-style params untouched": {
			params: BatchParams{
				DomainName: "d",
				BatchType:  BatchTypeTerminate,
				TerminateParams: TerminateParams{
					TerminateChildren: common.BoolPtr(true),
				},
			},
			want: BatchParams{
				DomainName: "d",
				BatchType:  BatchTypeTerminate,
				TerminateParams: TerminateParams{
					TerminateChildren: common.BoolPtr(true),
				},
			},
		},
		"terminate operation": {
			params: BatchParams{
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeTerminate.Ptr(),
					TerminateBatchOperation: &types.TerminateBatchOperation{
						TerminateChildren: common.BoolPtr(false),
					},
				},
			},
			want: BatchParams{
				BatchType: BatchTypeTerminate,
				TerminateParams: TerminateParams{
					TerminateChildren: common.BoolPtr(false),
				},
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeTerminate.Ptr(),
					TerminateBatchOperation: &types.TerminateBatchOperation{
						TerminateChildren: common.BoolPtr(false),
					},
				},
			},
		},
		"cancel operation": {
			params: BatchParams{
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeCancel.Ptr(),
					CancelBatchOperation: &types.CancelBatchOperation{
						CancelChildren: common.BoolPtr(true),
					},
				},
			},
			want: BatchParams{
				BatchType: BatchTypeCancel,
				CancelParams: CancelParams{
					CancelChildren: common.BoolPtr(true),
				},
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeCancel.Ptr(),
					CancelBatchOperation: &types.CancelBatchOperation{
						CancelChildren: common.BoolPtr(true),
					},
				},
			},
		},
		"signal operation": {
			params: BatchParams{
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeSignal.Ptr(),
					SignalBatchOperation: &types.SignalBatchOperation{
						SignalName: "my-signal",
						Input:      []byte("my-input"),
					},
				},
			},
			want: BatchParams{
				BatchType: BatchTypeSignal,
				SignalParams: SignalParams{
					SignalName: "my-signal",
					Input:      "my-input",
				},
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeSignal.Ptr(),
					SignalBatchOperation: &types.SignalBatchOperation{
						SignalName: "my-signal",
						Input:      []byte("my-input"),
					},
				},
			},
		},
		"replicate operation": {
			params: BatchParams{
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeReplicate.Ptr(),
					ReplicateBatchOperation: &types.ReplicateBatchOperation{
						SourceCluster: "cluster-a",
						TargetCluster: "cluster-b",
					},
				},
			},
			want: BatchParams{
				BatchType: BatchTypeReplicate,
				ReplicateParams: ReplicateParams{
					SourceCluster: "cluster-a",
					TargetCluster: "cluster-b",
				},
				Operation: &types.BatchOperation{
					OperationType: types.BatchOperationTypeReplicate.Ptr(),
					ReplicateBatchOperation: &types.ReplicateBatchOperation{
						SourceCluster: "cluster-a",
						TargetCluster: "cluster-b",
					},
				},
			},
		},
		"refresh operation": {
			params: BatchParams{
				Operation: &types.BatchOperation{
					OperationType:         types.BatchOperationTypeRefresh.Ptr(),
					RefreshBatchOperation: &types.RefreshBatchOperation{},
				},
			},
			want: BatchParams{
				BatchType: BatchTypeRefresh,
				Operation: &types.BatchOperation{
					OperationType:         types.BatchOperationTypeRefresh.Ptr(),
					RefreshBatchOperation: &types.RefreshBatchOperation{},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeOperation(tt.params))
		})
	}
}
