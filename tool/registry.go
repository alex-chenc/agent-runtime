package tool

import (
	"fmt"
	"sync"

	"github.com/alex-chenc/agent-runtime/core"
)

// Registry manages the set of available tool descriptors.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]core.ToolDescriptor
}

// NewRegistry creates a new tool registry from a slice of descriptors.
func NewRegistry(descriptors []core.ToolDescriptor) (*Registry, error) {
	r := &Registry{tools: make(map[string]core.ToolDescriptor, len(descriptors))}
	for _, d := range descriptors {
		if d.Name == "" {
			return nil, fmt.Errorf("tool registry: tool descriptor must have a name")
		}
		if _, exists := r.tools[d.Name]; exists {
			return nil, fmt.Errorf("tool registry: duplicate tool name %q", d.Name)
		}
		r.tools[d.Name] = d
	}
	return r, nil
}

// Get returns the descriptor for the named tool, or an error if not found.
func (r *Registry) Get(name string) (core.ToolDescriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.tools[name]
	if !ok {
		return core.ToolDescriptor{}, fmt.Errorf("tool %q not found in registry", name)
	}
	return d, nil
}

// Has returns true if the named tool is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// List returns all registered tool descriptors.
func (r *Registry) List() []core.ToolDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]core.ToolDescriptor, 0, len(r.tools))
	for _, d := range r.tools {
		result = append(result, d)
	}
	return result
}

// Names returns all registered tool names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}
