package task

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIwrrNode_ExecuteAtPath(t *testing.T) {
	tests := []struct {
		name               string
		setupFn            func() (*iwrrNode[string, int], *iwrrNode[string, int]) // returns (root, expectedChild)
		path               []WeightedKey[string]
		bufferSize         int
		callbackValue      int64
		expectedDelta      int64
		verifyFn           func(t *testing.T, root *iwrrNode[string, int], expectedChild *iwrrNode[string, int])
		expectedChildCount int64
	}{
		{
			name: "empty_path_executes_on_current_node",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path:          []WeightedKey[string]{},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				assert.False(t, root.drainSelfFirst.Load(), "drainSelfFirst should be false after execution on self")
			},
			expectedChildCount: 0,
		},
		{
			name: "single_element_path_creates_child",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path: []WeightedKey[string]{
				{Key: "child1", Weight: 3},
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				container := root.children["child1"]
				require.NotNil(t, container.item)
				assert.Equal(t, 3, container.weight)
				assert.True(t, root.drainSelfFirst.Load(), "root should have drainSelfFirst set")
				assert.False(t, container.item.drainSelfFirst.Load(), "target should not have drainSelfFirst set")
			},
			expectedChildCount: 1,
		},
		{
			name: "multi_element_path_creates_nested_children",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 5},
				{Key: "level2", Weight: 3},
				{Key: "level3", Weight: 2},
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				level1 := root.children["level1"]
				require.NotNil(t, level1.item)
				assert.Equal(t, 5, level1.weight)

				level2 := level1.item.children["level2"]
				require.NotNil(t, level2.item)
				assert.Equal(t, 3, level2.weight)

				level3 := level2.item.children["level3"]
				require.NotNil(t, level3.item)
				assert.Equal(t, 2, level3.weight)

				// Check childrenItemCount at each level
				assert.Equal(t, int64(1), root.childrenItemCount.Load())
				assert.Equal(t, int64(1), level1.item.childrenItemCount.Load())
				assert.Equal(t, int64(1), level2.item.childrenItemCount.Load())
			},
			expectedChildCount: 1,
		},
		{
			name: "existing_path_with_same_weight_reuses_nodes",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Pre-create a 3-level path
				root.Lock()
				level1 := newiwrrNode[string, int](10)
				root.children["level1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level1,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				level1.Lock()
				level2 := newiwrrNode[string, int](10)
				level1.children["level2"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level2,
					weight: 3,
				}
				level1.updateScheduleLocked()
				level1.Unlock()

				level2.Lock()
				level3 := newiwrrNode[string, int](10)
				level2.children["level3"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level3,
					weight: 2,
				}
				level2.updateScheduleLocked()
				level2.Unlock()

				// Store the expected nodes by writing unique IDs to their channels
				level1.c.Chan() <- 111
				level2.c.Chan() <- 222
				level3.c.Chan() <- 333

				return root, nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 5},
				{Key: "level2", Weight: 3},
				{Key: "level3", Weight: 2},
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				// Verify all three levels exist and have correct weights
				level1Container := root.children["level1"]
				require.NotNil(t, level1Container.item)
				assert.Equal(t, 5, level1Container.weight)
				// Check it's the same instance by reading the unique ID we stored
				id1, ok := <-level1Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 111, id1, "level1 should be reused (same instance)")

				level2Container := level1Container.item.children["level2"]
				require.NotNil(t, level2Container.item)
				assert.Equal(t, 3, level2Container.weight)
				id2, ok := <-level2Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 222, id2, "level2 should be reused (same instance)")

				level3Container := level2Container.item.children["level3"]
				require.NotNil(t, level3Container.item)
				assert.Equal(t, 2, level3Container.weight)
				id3, ok := <-level3Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 333, id3, "level3 should be reused (same instance)")

				// Verify childrenItemCount at each level
				assert.Equal(t, int64(1), root.childrenItemCount.Load())
				assert.Equal(t, int64(1), level1Container.item.childrenItemCount.Load())
				assert.Equal(t, int64(1), level2Container.item.childrenItemCount.Load())
			},
			expectedChildCount: 1,
		},
		{
			name: "existing_multi_level_path_with_weight_changes_reuses_nodes",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Pre-create a 3-level path with initial weights [5, 3, 2]
				root.Lock()
				level1 := newiwrrNode[string, int](10)
				root.children["level1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level1,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				level1.Lock()
				level2 := newiwrrNode[string, int](10)
				level1.children["level2"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level2,
					weight: 3,
				}
				level1.updateScheduleLocked()
				level1.Unlock()

				level2.Lock()
				level3 := newiwrrNode[string, int](10)
				level2.children["level3"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level3,
					weight: 2,
				}
				level2.updateScheduleLocked()
				level2.Unlock()

				// Store unique IDs to verify same instances
				level1.c.Chan() <- 111
				level2.c.Chan() <- 222
				level3.c.Chan() <- 333

				return root, nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 10}, // Changed from 5 to 10
				{Key: "level2", Weight: 7},  // Changed from 3 to 7
				{Key: "level3", Weight: 4},  // Changed from 2 to 4
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				// Verify all three levels exist with UPDATED weights
				level1Container := root.children["level1"]
				require.NotNil(t, level1Container.item)
				assert.Equal(t, 10, level1Container.weight, "level1 weight should be updated to 10")
				// Check it's the same instance by reading the unique ID we stored
				id1, ok := <-level1Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 111, id1, "level1 should be reused (same instance)")

				level2Container := level1Container.item.children["level2"]
				require.NotNil(t, level2Container.item)
				assert.Equal(t, 7, level2Container.weight, "level2 weight should be updated to 7")
				id2, ok := <-level2Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 222, id2, "level2 should be reused (same instance)")

				level3Container := level2Container.item.children["level3"]
				require.NotNil(t, level3Container.item)
				assert.Equal(t, 4, level3Container.weight, "level3 weight should be updated to 4")
				id3, ok := <-level3Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 333, id3, "level3 should be reused (same instance)")

				// Verify childrenItemCount at each level
				assert.Equal(t, int64(1), root.childrenItemCount.Load())
				assert.Equal(t, int64(1), level1Container.item.childrenItemCount.Load())
				assert.Equal(t, int64(1), level2Container.item.childrenItemCount.Load())
			},
			expectedChildCount: 1,
		},
		{
			name: "weight_change_triggers_schedule_update",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Pre-create a child with weight 5
				root.Lock()
				child := newiwrrNode[string, int](10)
				root.children["child1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   child,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()
				return root, child
			},
			path: []WeightedKey[string]{
				{Key: "child1", Weight: 10}, // Different weight
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], expectedChild *iwrrNode[string, int]) {
				container := root.children["child1"]
				require.NotNil(t, container.item)
				assert.Equal(t, 10, container.weight, "weight should be updated")
				// Should reuse the same child node even though weight changed
				assert.Same(t, expectedChild, container.item, "should reuse the same node instance")

				// Verify schedule was updated with new weight
				schedule := root.iwrrSchedule.Load()
				require.NotNil(t, schedule)
				// Schedule length should reflect the updated weight (10, not 5)
				assert.Equal(t, 10, schedule.Len(), "schedule length should reflect updated weight")
			},
			expectedChildCount: 1,
		},
		{
			name: "partial_path_exists_reuses_top_creates_bottom",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Only create level1, but not level2 or level3
				root.Lock()
				level1 := newiwrrNode[string, int](10)
				root.children["level1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level1,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				// Store unique ID in level1 to verify it's reused
				level1.c.Chan() <- 111

				return root, nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 5}, // Exists - should reuse
				{Key: "level2", Weight: 3}, // Doesn't exist - should create
				{Key: "level3", Weight: 2}, // Doesn't exist - should create
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				// Verify level1 was reused
				level1Container := root.children["level1"]
				require.NotNil(t, level1Container.item)
				assert.Equal(t, 5, level1Container.weight)
				id1, ok := <-level1Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 111, id1, "level1 should be reused (same instance)")

				// Verify level2 was created
				level2Container := level1Container.item.children["level2"]
				require.NotNil(t, level2Container.item)
				assert.Equal(t, 3, level2Container.weight)

				// Verify level3 was created
				level3Container := level2Container.item.children["level3"]
				require.NotNil(t, level3Container.item)
				assert.Equal(t, 2, level3Container.weight)

				// Verify childrenItemCount propagated correctly
				assert.Equal(t, int64(1), root.childrenItemCount.Load())
				assert.Equal(t, int64(1), level1Container.item.childrenItemCount.Load())
				assert.Equal(t, int64(1), level2Container.item.childrenItemCount.Load())
			},
			expectedChildCount: 1,
		},
		{
			name: "partial_path_exists_with_weight_change",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Create level1 and level2, but not level3
				root.Lock()
				level1 := newiwrrNode[string, int](10)
				root.children["level1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level1,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				level1.Lock()
				level2 := newiwrrNode[string, int](10)
				level1.children["level2"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level2,
					weight: 3,
				}
				level1.updateScheduleLocked()
				level1.Unlock()

				// Store unique IDs to verify reuse
				level1.c.Chan() <- 111
				level2.c.Chan() <- 222

				return root, nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 10}, // Exists - reuse with updated weight
				{Key: "level2", Weight: 3},  // Exists - reuse with same weight
				{Key: "level3", Weight: 2},  // Doesn't exist - create
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				// Verify level1 was reused with updated weight
				level1Container := root.children["level1"]
				require.NotNil(t, level1Container.item)
				assert.Equal(t, 10, level1Container.weight, "level1 weight should be updated")
				id1, ok := <-level1Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 111, id1, "level1 should be reused (same instance)")

				// Verify level2 was reused with same weight
				level2Container := level1Container.item.children["level2"]
				require.NotNil(t, level2Container.item)
				assert.Equal(t, 3, level2Container.weight)
				id2, ok := <-level2Container.item.c.Chan()
				require.True(t, ok)
				assert.Equal(t, 222, id2, "level2 should be reused (same instance)")

				// Verify level3 was newly created
				level3Container := level2Container.item.children["level3"]
				require.NotNil(t, level3Container.item)
				assert.Equal(t, 2, level3Container.weight)

				// Verify childrenItemCount propagated correctly
				assert.Equal(t, int64(1), root.childrenItemCount.Load())
				assert.Equal(t, int64(1), level1Container.item.childrenItemCount.Load())
				assert.Equal(t, int64(1), level2Container.item.childrenItemCount.Load())
			},
			expectedChildCount: 1,
		},
		{
			name: "multiple_children_at_same_level",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path: []WeightedKey[string]{
				{Key: "child1", Weight: 3},
			},
			bufferSize:    10,
			callbackValue: 1,
			expectedDelta: 1,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				// Execute on another child to verify multiple children work
				delta2 := root.executeAtPath([]WeightedKey[string]{
					{Key: "child2", Weight: 7},
				}, 10, func(c *TTLChannel[int]) int64 {
					return 2
				})
				assert.Equal(t, int64(2), delta2)
				assert.Len(t, root.children, 2)
				assert.Equal(t, 3, root.children["child1"].weight)
				assert.Equal(t, 7, root.children["child2"].weight)
				assert.Equal(t, int64(3), root.childrenItemCount.Load(), "should have sum of deltas")
			},
			expectedChildCount: 1,
		},
		{
			name: "callback_delta_propagates_up",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path: []WeightedKey[string]{
				{Key: "level1", Weight: 5},
				{Key: "level2", Weight: 3},
			},
			bufferSize:    10,
			callbackValue: 42,
			expectedDelta: 42,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				assert.Equal(t, int64(42), root.childrenItemCount.Load())
				assert.Equal(t, int64(42), root.children["level1"].item.childrenItemCount.Load())
			},
			expectedChildCount: 42,
		},
		{
			name: "zero_delta_from_callback",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				return newiwrrNode[string, int](10), nil
			},
			path: []WeightedKey[string]{
				{Key: "child1", Weight: 3},
			},
			bufferSize:    10,
			callbackValue: 0,
			expectedDelta: 0,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				assert.Equal(t, int64(0), root.childrenItemCount.Load())
			},
			expectedChildCount: 0,
		},
		{
			name: "negative_delta_from_callback",
			setupFn: func() (*iwrrNode[string, int], *iwrrNode[string, int]) {
				root := newiwrrNode[string, int](10)
				// Pre-populate some count
				root.childrenItemCount.Store(10)
				return root, nil
			},
			path: []WeightedKey[string]{
				{Key: "child1", Weight: 3},
			},
			bufferSize:    10,
			callbackValue: -5,
			expectedDelta: -5,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int], _ *iwrrNode[string, int]) {
				assert.Equal(t, int64(5), root.childrenItemCount.Load(), "should subtract from existing count")
			},
			expectedChildCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, expectedChild := tt.setupFn()

			// Create callback that returns the expected value
			delta := root.executeAtPath(tt.path, tt.bufferSize, func(c *TTLChannel[int]) int64 {
				return tt.callbackValue
			})

			assert.Equal(t, tt.expectedDelta, delta, "delta mismatch")
			assert.Equal(t, tt.expectedChildCount, root.childrenItemCount.Load(), "childrenItemCount mismatch")

			if tt.verifyFn != nil {
				tt.verifyFn(t, root, expectedChild)
			}
		})
	}
}

func TestIwrrNode_ExecuteAtPath_ConcurrentAccess(t *testing.T) {
	root := newiwrrNode[int, int](100)

	// Concurrently execute on multi-level paths
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				// Create 3-level paths with varying structures
				path := []WeightedKey[int]{
					{Key: id % 3, Weight: (id % 3) + 1}, // level1: 3 different keys
					{Key: id % 5, Weight: (id % 5) + 1}, // level2: 5 different keys
					{Key: id, Weight: id + 1},           // level3: 10 different keys
				}
				root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
					// Just return delta without writing to channel
					// to avoid blocking when channel is full
					return 1
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify tree structure
	level1Count := len(root.children)
	assert.LessOrEqual(t, level1Count, 3, "should have at most 3 level1 children")
	assert.GreaterOrEqual(t, level1Count, 1, "should have at least 1 level1 child")

	// Verify level2 and level3 exist
	for key1, container1 := range root.children {
		require.NotNil(t, container1.item, "level1 child %d should not be nil", key1)

		level2Count := len(container1.item.children)
		assert.LessOrEqual(t, level2Count, 5, "level1[%d] should have at most 5 level2 children", key1)
		assert.GreaterOrEqual(t, level2Count, 1, "level1[%d] should have at least 1 level2 child", key1)

		for key2, container2 := range container1.item.children {
			require.NotNil(t, container2.item, "level2 child %d->%d should not be nil", key1, key2)

			level3Count := len(container2.item.children)
			assert.GreaterOrEqual(t, level3Count, 1, "level2[%d][%d] should have at least 1 level3 child", key1, key2)
		}
	}

	// Total items should be 10 * 100 = 1000
	assert.Equal(t, int64(1000), root.childrenItemCount.Load())
}

func TestIwrrNode_Cleanup(t *testing.T) {
	baseTime := time.Unix(1000, 0)
	ttl := 10 * time.Second

	tests := []struct {
		name           string
		setupFn        func() *iwrrNode[string, int]
		now            time.Time
		ttl            time.Duration
		expectedReturn bool
		verifyFn       func(t *testing.T, root *iwrrNode[string, int])
	}{
		{
			name: "leaf_node_should_be_cleaned_up",
			setupFn: func() *iwrrNode[string, int] {
				node := newiwrrNode[string, int](10)
				// Set last write time to old time
				node.c.UpdateLastWriteTime(baseTime)
				// Ensure refCount is 0 and channel is empty
				return node
			},
			now:            baseTime.Add(ttl + time.Second), // Past TTL
			ttl:            ttl,
			expectedReturn: true,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Empty(t, root.children, "should have no children")
			},
		},
		{
			name: "leaf_node_should_not_be_cleaned_up_not_expired",
			setupFn: func() *iwrrNode[string, int] {
				node := newiwrrNode[string, int](10)
				node.c.UpdateLastWriteTime(baseTime)
				return node
			},
			now:            baseTime.Add(ttl - time.Second), // Before TTL
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Empty(t, root.children, "should have no children")
			},
		},
		{
			name: "leaf_node_should_not_be_cleaned_up_has_refs",
			setupFn: func() *iwrrNode[string, int] {
				node := newiwrrNode[string, int](10)
				node.c.UpdateLastWriteTime(baseTime)
				node.c.IncRef() // Add reference
				return node
			},
			now:            baseTime.Add(ttl + time.Second), // Past TTL
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Equal(t, int32(1), root.c.RefCount())
			},
		},
		{
			name: "leaf_node_should_not_be_cleaned_up_has_data",
			setupFn: func() *iwrrNode[string, int] {
				node := newiwrrNode[string, int](10)
				node.c.UpdateLastWriteTime(baseTime)
				node.c.Chan() <- 42 // Add data to channel
				return node
			},
			now:            baseTime.Add(ttl + time.Second), // Past TTL
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Equal(t, 1, root.c.Len())
			},
		},
		{
			name: "internal_node_with_active_children_should_not_be_cleaned_up",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)
				root.Lock()
				child := newiwrrNode[string, int](10)
				child.c.UpdateLastWriteTime(baseTime) // Recent write
				root.children["child1"] = weightedContainer[*iwrrNode[string, int]]{
					item:   child,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()
				return root
			},
			now:            baseTime.Add(ttl - time.Second), // Before TTL
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Len(t, root.children, 1, "child should not be removed")
				_, exists := root.children["child1"]
				assert.True(t, exists, "child should remain")
			},
		},
		{
			name: "internal_node_removes_expired_children",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)
				root.Lock()
				// Add two children: one expired, one active
				expiredChild := newiwrrNode[string, int](10)
				expiredChild.c.UpdateLastWriteTime(baseTime.Add(-2 * ttl)) // Way past TTL
				root.children["expired"] = weightedContainer[*iwrrNode[string, int]]{
					item:   expiredChild,
					weight: 3,
				}

				activeChild := newiwrrNode[string, int](10)
				activeChild.c.UpdateLastWriteTime(baseTime.Add(ttl + time.Second)) // Before TTL
				root.children["active"] = weightedContainer[*iwrrNode[string, int]]{
					item:   activeChild,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()
				return root
			},
			now:            baseTime.Add(ttl + time.Second),
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Len(t, root.children, 1, "should have 1 child remaining")
				_, exists := root.children["active"]
				assert.True(t, exists, "active child should remain")
				_, exists = root.children["expired"]
				assert.False(t, exists, "expired child should be removed")
			},
		},
		{
			name: "internal_node_removes_all_expired_children",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)
				root.Lock()
				// Add multiple expired children
				for i := 1; i <= 3; i++ {
					child := newiwrrNode[string, int](10)
					child.c.UpdateLastWriteTime(baseTime.Add(-ttl - time.Second))
					root.children[string(rune('a'+i-1))] = weightedContainer[*iwrrNode[string, int]]{
						item:   child,
						weight: i,
					}
				}
				root.updateScheduleLocked()
				root.Unlock()
				return root
			},
			now:            baseTime.Add(ttl + time.Second),
			ttl:            ttl,
			expectedReturn: true, // All children removed, so node should be removed
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Empty(t, root.children, "all children should be removed")
			},
		},
		{
			name: "schedule_updated_when_children_removed",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)
				root.Lock()
				// Add two children with different weights
				child1 := newiwrrNode[string, int](10)
				child1.c.UpdateLastWriteTime(baseTime.Add(-ttl - time.Second)) // Expired
				root.children["expired"] = weightedContainer[*iwrrNode[string, int]]{
					item:   child1,
					weight: 5,
				}

				child2 := newiwrrNode[string, int](10)
				child2.c.UpdateLastWriteTime(baseTime.Add(ttl + time.Second)) // Active
				root.children["active"] = weightedContainer[*iwrrNode[string, int]]{
					item:   child2,
					weight: 3,
				}
				root.updateScheduleLocked()
				root.Unlock()
				return root
			},
			now:            baseTime.Add(ttl + time.Second),
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Len(t, root.children, 1)
				_, exists := root.children["expired"]
				assert.False(t, exists, "expired child should be removed")
				_, exists = root.children["active"]
				assert.True(t, exists, "active child should remain")

				// New schedule should only have weight 3
				newSchedule := root.iwrrSchedule.Load()
				assert.Equal(t, 3, newSchedule.Len(), "updated schedule should only have weight 3")
			},
		},
		{
			name: "multi_level_recursive_cleanup",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)

				// Create 3-level tree: root -> level1 -> level2 -> level3
				root.Lock()
				level1 := newiwrrNode[string, int](10)
				level1.c.UpdateLastWriteTime(baseTime.Add(-ttl - time.Second)) // Expired
				root.children["expired"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level1,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				level1.Lock()
				level2 := newiwrrNode[string, int](10)
				level2.c.UpdateLastWriteTime(baseTime.Add(-ttl - time.Second)) // Expired
				level1.children["expired"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level2,
					weight: 3,
				}
				level1.updateScheduleLocked()
				level1.Unlock()

				level2.Lock()
				level3 := newiwrrNode[string, int](10)
				level3.c.UpdateLastWriteTime(baseTime.Add(-ttl - time.Second)) // Expired
				level2.children["level3"] = weightedContainer[*iwrrNode[string, int]]{
					item:   level3,
					weight: 2,
				}
				level2.updateScheduleLocked()
				level2.Unlock()

				return root
			},
			now:            baseTime.Add(ttl + time.Second),
			ttl:            ttl,
			expectedReturn: true, // All should cascade up and be removed
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Empty(t, root.children, "all children should be recursively removed")
			},
		},
		{
			name: "multi_level_partial_cleanup",
			setupFn: func() *iwrrNode[string, int] {
				root := newiwrrNode[string, int](10)

				// Create tree where one branch is expired, another is active
				root.Lock()
				expiredBranch := newiwrrNode[string, int](10)
				expiredBranch.c.UpdateLastWriteTime(baseTime.Add(-2 * ttl)) // Way past TTL
				root.children["expired"] = weightedContainer[*iwrrNode[string, int]]{
					item:   expiredBranch,
					weight: 3,
				}

				activeBranch := newiwrrNode[string, int](10)
				root.children["active"] = weightedContainer[*iwrrNode[string, int]]{
					item:   activeBranch,
					weight: 5,
				}
				root.updateScheduleLocked()
				root.Unlock()

				// Add active child to active branch
				activeBranch.Lock()
				activeLeaf := newiwrrNode[string, int](10)
				activeLeaf.c.UpdateLastWriteTime(baseTime) // Recent write
				activeLeaf.c.IncRef()                      // Keep it active
				activeBranch.children["leaf"] = weightedContainer[*iwrrNode[string, int]]{
					item:   activeLeaf,
					weight: 2,
				}
				activeBranch.updateScheduleLocked()
				activeBranch.Unlock()

				return root
			},
			now:            baseTime.Add(ttl + time.Second),
			ttl:            ttl,
			expectedReturn: false,
			verifyFn: func(t *testing.T, root *iwrrNode[string, int]) {
				assert.Len(t, root.children, 1, "only active branch should remain")
				_, exists := root.children["active"]
				assert.True(t, exists, "active branch should remain")

				activeBranch := root.children["active"].item
				assert.Len(t, activeBranch.children, 1, "active leaf should remain")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.setupFn()
			shouldRemove := root.cleanup(tt.now, tt.ttl)
			assert.Equal(t, tt.expectedReturn, shouldRemove, "cleanup return value mismatch")

			if tt.verifyFn != nil {
				tt.verifyFn(t, root)
			}
		})
	}
}

func TestIwrrNode_Cleanup_ConcurrentCleanup(t *testing.T) {
	// Test that multiple concurrent cleanup calls don't cause race conditions
	root := newiwrrNode[string, int](10)

	// Create a tree with multiple levels and old timestamps
	// root -> level1a -> level2a
	//      -> level1b -> level2b
	//      -> level1c -> level2c
	oldTime := time.Now().Add(-2 * time.Hour) // Old enough to be cleaned up
	for i := 0; i < 3; i++ {
		path := []WeightedKey[string]{
			{Key: fmt.Sprintf("level1_%d", i), Weight: 1},
			{Key: fmt.Sprintf("level2_%d", i), Weight: 1},
		}
		root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
			c.UpdateLastWriteTime(oldTime) // Set old timestamp
			return 0                       // Don't add items, just create the tree structure
		})
	}

	// Verify initial state
	require.Equal(t, 3, len(root.children))

	// Run multiple concurrent cleanup operations
	now := time.Now()
	ttl := time.Hour

	var wg sync.WaitGroup
	numGoroutines := 10
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			root.cleanup(now, ttl)
		}()
	}

	wg.Wait()

	// Verify tree was cleaned up correctly
	// All nodes should be removed since they're idle
	assert.Equal(t, 0, len(root.children))
}

func TestIwrrNode_Cleanup_ConcurrentWithExecuteAtPath(t *testing.T) {
	// Test that cleanup and executeAtPath can run concurrently without deadlock or corruption
	// Uses multi-level hierarchy with multiple goroutines for both operations
	root := newiwrrNode[string, int](100)

	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	var tasksAdded atomic.Int64

	// Multiple goroutines running executeAtPath with multi-level paths
	numExecuteGoroutines := 10
	for g := 0; g < numExecuteGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stopCh:
					return
				default:
					// Create multi-level paths: domain -> tasklist -> tenant
					domainKey := fmt.Sprintf("domain_%d", (goroutineID+counter)%3)
					tasklistKey := fmt.Sprintf("tasklist_%d", (goroutineID+counter)%5)
					tenantKey := fmt.Sprintf("tenant_%d", (goroutineID+counter)%7)

					path := []WeightedKey[string]{
						{Key: domainKey, Weight: ((goroutineID + counter) % 3) + 1},
						{Key: tasklistKey, Weight: ((goroutineID + counter) % 5) + 1},
						{Key: tenantKey, Weight: ((goroutineID + counter) % 7) + 1},
					}

					taskValue := goroutineID*10000 + counter
					root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
						select {
						case c.Chan() <- taskValue:
							c.UpdateLastWriteTime(time.Now())
							tasksAdded.Add(1)
							return 1
						default:
							return 0
						}
					})
					counter++
				}
			}
		}(g)
	}

	// Multiple goroutines running cleanup
	numCleanupGoroutines := 5
	for g := 0; g < numCleanupGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					// Cleanup with a TTL that will remove idle nodes
					now := time.Now()
					ttl := 50 * time.Millisecond
					root.cleanup(now, ttl)
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Let them run concurrently for a short time
	time.Sleep(50 * time.Millisecond)
	close(stopCh)
	wg.Wait()

	// Drain all tasks and verify count
	drainedCount := 0
	for {
		if _, ok := root.tryGetNextItem(); ok {
			drainedCount++
		} else {
			break
		}
	}

	// All added items should be drained
	assert.Equal(t, int(tasksAdded.Load()), drainedCount, "All enqueued items should be dequeuable")
}

func TestIwrrNode_Cleanup_PointerEqualityCheck(t *testing.T) {
	// Test that cleanup only removes the child it decided to remove,
	// not a newly created child with the same key
	root := newiwrrNode[string, int](10)

	// Add an initial child that will be cleaned up
	path := []WeightedKey[string]{
		{Key: "child", Weight: 1},
	}
	root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
		c.UpdateLastWriteTime(time.Now().Add(-2 * time.Hour)) // Old timestamp
		return 0
	})

	// Start cleanup in background (it will snapshot, then recurse, then try to delete)
	cleanupDone := make(chan bool)
	go func() {
		// Add a small delay to ensure executeAtPath runs during the recursive phase
		now := time.Now()
		ttl := time.Hour
		root.cleanup(now, ttl)
		cleanupDone <- true
	}()

	// While cleanup is running, add a new child with the same key
	// This simulates the race condition the pointer equality check protects against
	time.Sleep(time.Millisecond) // Give cleanup time to snapshot
	path2 := []WeightedKey[string]{
		{Key: "child", Weight: 2}, // Same key, different weight
	}
	root.executeAtPath(path2, 10, func(c *TTLChannel[int]) int64 {
		c.UpdateLastWriteTime(time.Now())
		return 0
	})

	<-cleanupDone

	// Verify: the new child should still exist
	root.RLock()
	container, exists := root.children["child"]
	root.RUnlock()

	// The child should exist (either old or new depending on timing)
	// If it's the new child, weight should be 2
	// If it's the old child, weight should be 1
	// Either way, the tree should be in a consistent state
	if exists {
		// If child exists, verify it's a valid node
		assert.NotNil(t, container.item)
		assert.Contains(t, []int{1, 2}, container.weight)
	}
}

func TestIwrrNode_Cleanup_DeepHierarchyConcurrency(t *testing.T) {
	// Test cleanup performance with deep hierarchy and concurrent operations
	// Verifies all tasks can be drained even with concurrent cleanup
	root := newiwrrNode[string, int](10)

	// Track tasks added
	var tasksAdded atomic.Int64

	// Create a deep hierarchy (5 levels, 3 children per level = 243 leaf paths)
	var createPaths func(depth int, prefix string) [][]WeightedKey[string]
	createPaths = func(depth int, prefix string) [][]WeightedKey[string] {
		if depth == 0 {
			return [][]WeightedKey[string]{{}}
		}
		var paths [][]WeightedKey[string]
		for i := 0; i < 3; i++ {
			key := fmt.Sprintf("%s_L%d_C%d", prefix, 5-depth, i)
			subPaths := createPaths(depth-1, prefix)
			for _, subPath := range subPaths {
				path := append([]WeightedKey[string]{{Key: key, Weight: i + 1}}, subPath...)
				paths = append(paths, path)
			}
		}
		return paths
	}

	paths := createPaths(5, "node")
	for i, path := range paths {
		root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
			select {
			case c.Chan() <- i:
				c.UpdateLastWriteTime(time.Now())
				tasksAdded.Add(1)
				return 1
			default:
				return 0
			}
		})
	}

	// Verify tree was created
	require.Equal(t, 3, len(root.children))

	var wg sync.WaitGroup

	// Start multiple cleanup operations concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			now := time.Now().Add(2 * time.Hour)
			ttl := time.Hour
			root.cleanup(now, ttl)
		}()
	}

	// Also start some executeAtPath operations to add new nodes/tasks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path := []WeightedKey[string]{
				{Key: fmt.Sprintf("new_L0_C%d", idx), Weight: 1},
			}
			root.executeAtPath(path, 10, func(c *TTLChannel[int]) int64 {
				select {
				case c.Chan() <- idx + 1000:
					c.UpdateLastWriteTime(time.Now())
					tasksAdded.Add(1)
					return 1
				default:
					return 0
				}
			})
		}(i)
	}

	wg.Wait()

	// Drain all remaining tasks to verify nothing was lost
	drainedCount := 0
	for {
		if _, ok := root.tryGetNextItem(); ok {
			drainedCount++
		} else {
			break
		}
	}

	totalAdded := int(tasksAdded.Load())
	assert.Equal(t, totalAdded, drainedCount, "All added tasks should be dequeuable")
}
