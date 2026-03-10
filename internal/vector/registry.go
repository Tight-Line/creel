package vector

import (
	"fmt"
	"sync"
)

// Factory creates a Backend from a configuration map.
type Factory func(config map[string]any) (Backend, error)

// Registry manages lazy-initialized vector backends keyed by config ID.
// It holds a fallback backend used when no per-topic config is specified.
type Registry struct {
	mu        sync.RWMutex
	backends  map[string]Backend
	factories map[string]Factory
	fallback  Backend
}

// NewRegistry creates a new backend registry with the given fallback backend.
func NewRegistry(fallback Backend) *Registry {
	return &Registry{
		backends:  make(map[string]Backend),
		factories: make(map[string]Factory),
		fallback:  fallback,
	}
}

// RegisterFactory registers a factory for a backend type (e.g. "pgvector", "qdrant").
func (r *Registry) RegisterFactory(backendType string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[backendType] = factory
}

// Get returns a backend for the given config ID. If configID is nil or empty,
// the fallback backend is returned.
func (r *Registry) Get(configID *string) (Backend, error) {
	if configID == nil || *configID == "" {
		return r.fallback, nil
	}

	r.mu.RLock()
	b, ok := r.backends[*configID]
	r.mu.RUnlock()
	if ok {
		return b, nil
	}
	return nil, fmt.Errorf("vector backend config %q not initialized; call Put to register it", *configID)
}

// Put registers a live backend for a config ID.
func (r *Registry) Put(configID string, backend Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[configID] = backend
}

// GetOrCreate returns a cached backend for the config ID, or creates one using
// the registered factory for the given backend type and config map.
func (r *Registry) GetOrCreate(configID, backendType string, config map[string]any) (Backend, error) {
	if configID == "" {
		return r.fallback, nil
	}

	r.mu.RLock()
	b, ok := r.backends[configID]
	r.mu.RUnlock()
	if ok {
		return b, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// coverage:ignore - race condition double-check; requires concurrent access to trigger
	if b, ok := r.backends[configID]; ok {
		return b, nil
	}

	factory, ok := r.factories[backendType]
	if !ok {
		return nil, fmt.Errorf("no factory registered for backend type %q", backendType)
	}

	b, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("creating backend %q: %w", backendType, err)
	}
	r.backends[configID] = b
	return b, nil
}

// Remove evicts a cached backend for a config ID.
func (r *Registry) Remove(configID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.backends, configID)
}

// Fallback returns the registry's fallback backend.
func (r *Registry) Fallback() Backend {
	return r.fallback
}
