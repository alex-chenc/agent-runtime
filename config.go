package agentruntime

import "github.com/alex-chenc/agent-runtime/core"

// RuntimeConfig holds all configuration for a Runtime instance.
type RuntimeConfig = core.RuntimeConfig

// ConfigPatch holds partial config overrides for a single task.
type ConfigPatch = core.ConfigPatch

// DefaultConfig returns a RuntimeConfig with sensible defaults.
func DefaultConfig() RuntimeConfig {
	return core.DefaultConfig()
}
