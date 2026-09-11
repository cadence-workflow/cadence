package semaphore

import "sync"

var _ SemaphoreRegistry = (*semaphoreRegistryImpl)(nil)

type semaphoreRegistryImpl struct {
	mu       sync.RWMutex
	managers map[Identifier]Manager
}

func NewSemaphoreRegistry() SemaphoreRegistry {
	return &semaphoreRegistryImpl{managers: make(map[Identifier]Manager)}
}

func (r *semaphoreRegistryImpl) Register(mgr Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.managers[mgr.Identifier()] = mgr
}

func (r *semaphoreRegistryImpl) Unregister(mgr Manager) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	// we need to make sure we still hold the given `mgr` or we already replaced with a new one.
	if current, ok := r.managers[mgr.Identifier()]; !ok || current != mgr {
		return false
	}
	delete(r.managers, mgr.Identifier())
	return true
}

// ManagerByIdentifier returns the manager held for id, if there is one. It may still be
// starting: Acquire is what waits for that.
func (r *semaphoreRegistryImpl) ManagerByIdentifier(id Identifier) (Manager, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mgr, ok := r.managers[id]
	return mgr, ok
}

func (r *semaphoreRegistryImpl) AllManagers() []Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	managers := make([]Manager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	return managers
}
