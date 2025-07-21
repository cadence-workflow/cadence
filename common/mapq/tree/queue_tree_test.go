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

package tree

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/mapq/types"
	"github.com/uber/cadence/common/metrics"
)

func TestStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)
	consumerFactory := types.NewMockConsumerFactory(ctrl)
	consumer := types.NewMockConsumer(ctrl)
	// 7 consumers are created in the test tree for each leaf node defined by getTestPolicies()
	// - */timer/deletehistory/*
	// - */timer/*/domain1
	// - */timer/*/*
	// - */transfer/*/domain1
	// - */transfer/*/*
	// - */*/*/domain1
	// - */*/*/*
	consumerFactory.EXPECT().New(gomock.Any()).Return(consumer, nil).Times(7)

	tree, err := New(
		testlogger.New(t),
		metrics.NoopScope,
		[]string{"type", "sub-type", "domain"},
		getTestPolicies(),
		types.NewMockPersister(ctrl),
		consumerFactory,
	)
	if err != nil {
		t.Fatalf("failed to create queue tree: %v", err)
	}

	if err := tree.Start(context.Background()); err != nil {
		t.Fatalf("failed to start queue tree: %v", err)
	}

	if err := tree.Stop(context.Background()); err != nil {
		t.Fatalf("failed to stop queue tree: %v", err)
	}
}

func TestEnqueue(t *testing.T) {
	tests := []struct {
		name               string
		policies           []types.NodePolicy
		leafNodeCount      int
		items              []types.Item
		persistErr         error
		wantItemsToPersist []types.ItemToPersist
		wantErr            bool
	}{
		{
			name:          "success",
			policies:      getTestPolicies(),
			leafNodeCount: 7,
			items: []types.Item{
				mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain1"}),
				mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain2"}),
				mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain3"}),
				mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain1"}),
				mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain2"}),
				mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain3"}),
				mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain1"}),
				mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain2"}),
				mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain3"}),
			},
			wantItemsToPersist: func() []types.ItemToPersist {
				result := make([]types.ItemToPersist, 9)
				partitions := []string{"type", "sub-type", "domain"}

				// deletehistory timers should go to the leaf node "*/timer/deletehistory/*"
				result[0] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain1"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "*"}),
				)
				result[1] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain2"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "*"}),
				)
				result[2] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "domain3"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "deletehistory", "domain": "*"}),
				)

				// activitytimeout timers either goes to domain1 specific leaf node ("*/timer/*/domain1") or the catch all leaf node "*/timer/*/*"
				result[3] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain1"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "*", "domain": "domain1"}),
				)
				result[4] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain2"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "*", "domain": "*"}),
				)
				result[5] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "timer", "sub-type": "activitytimeout", "domain": "domain3"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "timer", "sub-type": "*", "domain": "*"}),
				)

				// transfer tasks either goes to domain1 specific leaf node ("*/transfer/*/domain1") or the catch all leaf node "*/transfer/*/*"
				result[6] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain1"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "transfer", "sub-type": "*", "domain": "domain1"}),
				)
				result[7] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain2"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "transfer", "sub-type": "*", "domain": "*"}),
				)
				result[8] = types.NewItemToPersist(
					mockItem(t, map[string]any{"type": "transfer", "sub-type": "activity", "domain": "domain3"}),
					types.NewItemPartitions(partitions, map[string]any{"type": "transfer", "sub-type": "*", "domain": "*"}),
				)

				return result
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			defer goleak.VerifyNone(t)
			ctrl := gomock.NewController(t)
			consumerFactory := types.NewMockConsumerFactory(ctrl)
			consumer := types.NewMockConsumer(ctrl)
			// consumers will be creeted for each leaf node
			consumerFactory.EXPECT().New(gomock.Any()).Return(consumer, nil).Times(tc.leafNodeCount)

			var gotItemsToPersistByPersister []types.ItemToPersist
			persister := types.NewMockPersister(ctrl)
			persister.EXPECT().Persist(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, itemsToPersist []types.ItemToPersist) error {
				gotItemsToPersistByPersister = itemsToPersist
				return tc.persistErr
			})

			tree, err := New(
				testlogger.New(t),
				metrics.NoopScope,
				[]string{"type", "sub-type", "domain"},
				getTestPolicies(),
				persister,
				consumerFactory,
			)
			if err != nil {
				t.Fatalf("failed to create queue tree: %v", err)
			}

			if err := tree.Start(context.Background()); err != nil {
				t.Fatalf("failed to start queue tree: %v", err)
			}
			defer tree.Stop(context.Background())

			returnedItemsToPersist, err := tree.Enqueue(context.Background(), tc.items)
			if (err != nil) != tc.wantErr {
				t.Errorf("Enqueue() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if got, want := len(gotItemsToPersistByPersister), len(tc.wantItemsToPersist); got != want {
				t.Fatalf("Items to persist count mismatch: gotItemsToPersistByPersister=%v, want=%v", got, want)
			}

			if got, want := len(returnedItemsToPersist), len(tc.wantItemsToPersist); got != want {
				t.Fatalf("Items to persist count mismatch: returnedItemsToPersist=%v, want=%v", got, want)
			}

			for i := range gotItemsToPersistByPersister {
				if diff := cmp.Diff(tc.wantItemsToPersist[i].String(), gotItemsToPersistByPersister[i].String()); diff != "" {
					t.Errorf("%d - gotItemsToPersistByPersister mismatch (-want +got):\n%s", i, diff)
				}

				if diff := cmp.Diff(tc.wantItemsToPersist[i].String(), returnedItemsToPersist[i].String()); diff != "" {
					t.Errorf("%d - returnedItemsToPersist mismatch (-want +got):\n%s", i, diff)
				}
			}
		})
	}
}

func mockItem(t *testing.T, attributes map[string]any) types.Item {
	item := types.NewMockItem(gomock.NewController(t))
	item.EXPECT().GetAttribute(gomock.Any()).DoAndReturn(func(key string) any {
		return attributes[key]
	}).AnyTimes()
	item.EXPECT().String().Return("mockitem").AnyTimes()
	return item
}

func getTestPolicies() []types.NodePolicy {
	return []types.NodePolicy{
		{
			Path: "*", // level 0
			SplitPolicy: &types.SplitPolicy{
				PredefinedSplits: []any{"timer", "transfer"},
			},
			DispatchPolicy: &types.DispatchPolicy{DispatchRPS: 100},
		},
		{
			Path:           "*/.", // level 1 default policy
			SplitPolicy:    &types.SplitPolicy{},
			DispatchPolicy: &types.DispatchPolicy{DispatchRPS: 50},
		},
		{
			Path: "*/timer", // level 1 timer node
			SplitPolicy: &types.SplitPolicy{
				PredefinedSplits: []any{"deletehistory"},
			},
		},
		{
			Path: "*/./.", // level 2 default policy
			SplitPolicy: &types.SplitPolicy{
				PredefinedSplits: []any{"domain1"},
			},
		},
		{
			Path: "*/timer/deletehistory", // level 2 deletehistory timer node policy
			SplitPolicy: &types.SplitPolicy{
				Disabled: true,
			},
			DispatchPolicy: &types.DispatchPolicy{DispatchRPS: 5},
		},
		{
			Path: "*/././*", // level 3 default catch-all node policy
			SplitPolicy: &types.SplitPolicy{
				Disabled: true,
			},
			DispatchPolicy: &types.DispatchPolicy{DispatchRPS: 1000},
		},
		{
			Path: "*/././domain1", // level 3 domain node policy
			SplitPolicy: &types.SplitPolicy{
				Disabled: true,
			},
			DispatchPolicy: &types.DispatchPolicy{DispatchRPS: 42},
		},
	}
}
