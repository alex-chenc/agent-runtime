package agentruntime

import "github.com/alex-chenc/agent-runtime/core"

// ToolDescriptor describes a tool that can be called by the agent.
type ToolDescriptor = core.ToolDescriptor

// ToolPrerequisite declares evidence required before a descriptor can run.
type ToolPrerequisite = core.ToolPrerequisite

const PrerequisiteCapabilityEmptyResult = core.PrerequisiteCapabilityEmptyResult
