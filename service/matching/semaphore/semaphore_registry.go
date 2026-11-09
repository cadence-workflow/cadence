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

func (r *semaphoreRegistryImpl) GetOrCreate(id Identifier, create func() (Manager, error)) (Manager, error) {
	// Almost every call finds a manager already there, so look under the read lock first.
	r.mu.RLock()
	mgr, ok := r.managers[id]
	r.mu.RUnlock()
	if ok {
		return mgr, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// Checked again: another caller may have built one between the two locks.
	if existing, ok := r.managers[id]; ok {
		return existing, nil
	}
	mgr, err := create()
	if err != nil {
		return nil, err
	}
	r.managers[id] = mgr
	return mgr, nil
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
