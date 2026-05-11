package apperr

import (
	"github.com/chenchen511/agent-runtime/core"
	"github.com/chenchen511/agent-runtime/internal/ids"
)

// Manager tracks and categorizes runtime errors.
type Manager struct {
	errors []core.RuntimeError
	idGen  core.IDGenerator
}

// NewManager creates a new error manager.
func NewManager(idGen core.IDGenerator) *Manager {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Manager{idGen: idGen}
}

// Add records an error, assigning it an ID if missing.
func (m *Manager) Add(err core.RuntimeError) {
	if err.ErrorID == "" {
		err.ErrorID = m.idGen.Generate()
	}
	m.errors = append(m.errors, err)
}

// Errors returns all recorded errors.
func (m *Manager) Errors() []core.RuntimeError {
	return m.errors
}

// Count returns the number of errors of a given kind.
func (m *Manager) Count(kind core.ErrorKind) int {
	count := 0
	for _, e := range m.errors {
		if e.Kind == kind {
			count++
		}
	}
	return count
}

// Recent returns the last n errors.
func (m *Manager) Recent(n int) []core.RuntimeError {
	if len(m.errors) <= n {
		return m.errors
	}
	return m.errors[len(m.errors)-n:]
}
