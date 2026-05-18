package agentruntime

import "github.com/chenchen511/agent-runtime/core"

// Re-export all interfaces and their request/response types from core.

type LLMClient = core.LLMClient
type LLMRequest = core.LLMRequest
type LLMMessage = core.LLMMessage
type LLMResponse = core.LLMResponse
type LLMUsage = core.LLMUsage
type ResponseFormat = core.ResponseFormat
type ResponseFormatSchema = core.ResponseFormatSchema

type ToolGateway = core.ToolGateway
type ToolRequest = core.ToolRequest
type ToolResponse = core.ToolResponse

type ExperienceProvider = core.ExperienceProvider
type ExperienceRequest = core.ExperienceRequest
type ExperienceResponse = core.ExperienceResponse
type ExperienceItem = core.ExperienceItem

type HookSink = core.HookSink
type HookEvent = core.HookEvent

type PromptProvider = core.PromptProvider
type PromptRequest = core.PromptRequest
type PromptBundle = core.PromptBundle

type ToolPolicy = core.ToolPolicy
type ToolPolicyRequest = core.ToolPolicyRequest

type Clock = core.Clock
type IDGenerator = core.IDGenerator
