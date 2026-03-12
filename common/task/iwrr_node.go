package task

import (
	"sync"
	"sync/atomic"
	"time"
)

type (
	// WeightedKey represents a key-weight pair used in hierarchical IWRR scheduling.
	// The key identifies a node at a specific level in the hierarchy, and the weight
	// determines the relative priority of that node in the IWRR schedule.
	WeightedKey[K comparable] struct {
		Key    K   // the identifier for this level in the hierarchy
		Weight int // the relative weight/priority for IWRR scheduling (higher weight = more frequent selection)
	}

	// iwrrNode represents a node in the hierarchical IWRR tree structure.
	// Each node can contain its own TTL channel for items and/or children nodes that form a subtree.
	// The tree structure allows for hierarchical weighted round-robin scheduling where items are
	// distributed across multiple levels based on their key paths and weights.
	iwrrNode[K comparable, V any] struct {
		sync.RWMutex
		// The following fields are protected by the node's RWMutex
		children map[K]weightedContainer[*iwrrNode[K, V]] // child nodes with their associated weights

		// The following fields are immutable after construction
		c *TTLChannel[V] // TTL channel for storing items at this level

		// The following fields are concurrency-safe atomic fields
		childrenItemCount atomic.Int64                                  // total number of items in all children's channels (across entire subtree)
		drainSelfFirst    atomic.Bool                                   // when true, this node's channel is drained before children; when false, children are checked first
		iwrrSchedule      atomic.Pointer[iwrrSchedule[*iwrrNode[K, V]]] // atomic snapshot of all direct children for IWRR scheduling

		// The following fields are NOT concurrency-safe and must only be accessed by the single consumer goroutine
		iter Iterator[*iwrrNode[K, V]] // stateful iterator for traversing the IWRR schedule
	}
)

// newiwrrNode creates a new iwrrNode with the specified buffer size for its TTL channel.
// The node is initialized with an empty children map and an empty IWRR schedule.
func newiwrrNode[K comparable, V any](bufferSize int) *iwrrNode[K, V] {
	node := &iwrrNode[K, V]{
		c:        NewTTLChannel[V](bufferSize),
		children: make(map[K]weightedContainer[*iwrrNode[K, V]]),
	}
	schedule := newIWRRSchedule[K, *iwrrNode[K, V]](nil)
	node.iwrrSchedule.Store(schedule)
	node.iter = schedule.NewIterator()
	return node
}

// executeAtPath recursively navigates or creates nodes along the hierarchical key path
// and executes the callback function at the target (leaf) node.
//
// The method traverses the tree following the path, creating intermediate nodes as needed.
// When a child node is created or its weight changes, the parent's IWRR schedule is updated.
// After executing the callback, the drainSelfFirst flag is set to indicate children should
// be drained first, and the childrenItemCount is updated with the delta from the callback.
//
// Parameters:
//   - path: slice of WeightedKey representing the hierarchical path to the target node
//   - bufferSize: buffer size for any newly created nodes
//   - callback: function to execute on the target node's channel, returns the change in item count
//
// Returns: the delta in item count from the callback execution
//
// Concurrency: This method is safe for concurrent access from multiple producer goroutines.
func (n *iwrrNode[K, V]) executeAtPath(path []WeightedKey[K], bufferSize int, callback func(c *TTLChannel[V]) int64) int64 {
	if len(path) == 0 {
		n.drainSelfFirst.Store(false)
		delta := callback(n.c)
		return delta
	}

	key := path[0].Key
	weight := path[0].Weight
	needsUpdate := false

	n.RLock()
	if container, exists := n.children[key]; exists && container.weight == weight {
		// Increment reference count to prevent cleanup while we're using this child
		container.item.c.IncRef()
		defer container.item.c.DecRef()
		n.RUnlock()
		delta := container.item.executeAtPath(path[1:], bufferSize, callback)
		n.drainSelfFirst.Store(true)
		n.childrenItemCount.Add(delta)
		return delta
	}
	n.RUnlock()

	n.Lock()
	container, exists := n.children[key]
	if !exists {
		child := newiwrrNode[K, V](bufferSize)
		n.children[key] = weightedContainer[*iwrrNode[K, V]]{
			item:   child,
			weight: weight,
		}
		container = n.children[key]
		needsUpdate = true
	} else if container.weight != weight {
		n.children[key] = weightedContainer[*iwrrNode[K, V]]{
			item:   container.item,
			weight: weight,
		}
		container = n.children[key]
		needsUpdate = true
	}
	// Update this node's schedule on the way back if needed
	if needsUpdate {
		n.updateScheduleLocked()
	}
	// Increment reference count to prevent cleanup while we're using this child
	container.item.c.IncRef()
	defer container.item.c.DecRef()
	n.Unlock()
	// Recurse to execute at the leaf node
	delta := container.item.executeAtPath(path[1:], bufferSize, callback)
	n.drainSelfFirst.Store(true)
	n.childrenItemCount.Add(delta)
	return delta
}

// updateScheduleLocked updates the IWRR schedule for this node based on its current direct children.
// The new schedule is atomically stored, allowing the consumer goroutine to pick it up when the
// current iterator naturally exhausts and resets.
//
// Concurrency: Must be called with write lock held on this node.
func (n *iwrrNode[K, V]) updateScheduleLocked() {
	schedule := newIWRRSchedule[K, *iwrrNode[K, V]](n.children)
	n.iwrrSchedule.Store(schedule)
	// The iterator will pick up the new schedule when it naturally exhausts and resets
}

// tryGetNextItem attempts to retrieve the next item from this subtree using IWRR scheduling.
// The method respects the drainSelfFirst flag to determine priority:
//   - If drainSelfFirst is true: tries own channel first, then children
//   - If drainSelfFirst is false: tries children first, then own channel
//
// When traversing children, uses the IWRR iterator to select children according to their weights.
//
// Returns: the item and true if an item was successfully retrieved, zero value and false otherwise
//
// Concurrency: NOT safe for concurrent access. Must only be called by a single consumer goroutine.
func (n *iwrrNode[K, V]) tryGetNextItem() (V, bool) {
	var zero V

	// Read drainSelfFirst flag
	drainSelf := n.drainSelfFirst.Load()

	// Check drainSelfFirst flag to determine order
	if drainSelf {
		if item, ok := n.tryOwnChannel(); ok {
			return item, true
		}
	}

	// Try children using IWRR iterator
	if item, ok := n.tryChildren(); ok {
		return item, true
	}

	// Try own channel if we haven't already
	if !drainSelf {
		if item, ok := n.tryOwnChannel(); ok {
			return item, true
		}
	}

	return zero, false
}

// tryOwnChannel attempts to retrieve an item from this node's own TTL channel.
// Uses a non-blocking select to avoid waiting if the channel is empty.
//
// Returns: the item and true if an item was available, zero value and false if the channel is empty
func (n *iwrrNode[K, V]) tryOwnChannel() (V, bool) {
	var zero V
	select {
	case item := <-n.c.Chan():
		return item, true
	default:
		return zero, false
	}
}

// tryChildren attempts to retrieve an item from one of the child nodes using the IWRR iterator.
// The method loops while childrenItemCount indicates items are available in the subtree.
// If the iterator is exhausted but items remain, it resets the iterator with a fresh schedule snapshot.
// When a child returns an item, the childrenItemCount is decremented to reflect the removal.
//
// This approach ensures:
//   - Children are selected according to their IWRR weights
//   - Schedule updates from producers are eventually picked up
//   - The method doesn't block indefinitely on empty children
//
// Returns: the item and true if an item was retrieved from a child, zero value and false otherwise
func (n *iwrrNode[K, V]) tryChildren() (V, bool) {
	var zero V
	for n.childrenItemCount.Load() > 0 {
		child, ok := n.iter.TryNext()
		if !ok {
			// Iterator exhausted - check if any children have items
			if n.childrenItemCount.Load() == 0 {
				// No children have items, give up
				return zero, false
			}

			// Some child has items, reset iterator and try another round
			schedule := n.iwrrSchedule.Load()
			n.iter = schedule.NewIterator()
			continue
		}

		// Recursively try to get item from child
		item, ok := child.tryGetNextItem()
		if ok {
			// Got an item from child, decrement children count
			n.childrenItemCount.Add(-1)
			return item, true
		}
		// Child had no item, try next child
	}
	return zero, false
}

// cleanup recursively removes idle nodes from the tree based on the TTL.
// The method performs a depth-first traversal, cleaning up children before checking itself.
//
// Process:
//  1. Recursively cleanup all children
//  2. Remove children that return true (indicating they should be removed)
//  3. Update the IWRR schedule if any children were removed
//  4. If this node is now a leaf, check if its own channel should be cleaned up
//
// A leaf node is eligible for removal if its TTL channel has been idle beyond the TTL duration.
//
// Returns: true if this node should be removed by its parent, false otherwise
//
// Concurrency: Safe for concurrent access. Acquires write lock during execution.
func (n *iwrrNode[K, V]) cleanup(now time.Time, ttl time.Duration) bool {
	n.Lock()
	defer n.Unlock()

	// Clean up children first and collect keys to remove
	keysToRemove := make([]K, 0)
	for key, container := range n.children {
		if container.item.cleanup(now, ttl) {
			keysToRemove = append(keysToRemove, key)
		}
	}

	// Remove children that should be cleaned up
	needsUpdate := false
	for _, key := range keysToRemove {
		delete(n.children, key)
		needsUpdate = true
	}

	// Update schedule if children were removed
	if needsUpdate {
		n.updateScheduleLocked()
	}

	// If the node is a leaf node or becomes a leaf node, check if it should be removed
	if len(n.children) == 0 {
		return n.c.ShouldCleanup(now, ttl)
	}
	return false
}
