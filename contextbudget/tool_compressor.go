package contextbudget

import (
	"encoding/json"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
)

const (
	maxToolResultTokens = 2000
	maxTextResultChars  = 8000
)

// ToolCompressor compresses tool results to reduce context usage.
// It supports 6 tool types with per-tool preservation strategies.
// JSON is parsed first; text fallback is used if parsing fails.
type ToolCompressor struct {
	estimator TokenEstimator
}

// NewToolCompressor creates a new ToolCompressor.
func NewToolCompressor(estimator TokenEstimator) *ToolCompressor {
	return &ToolCompressor{estimator: estimator}
}

// CompressToolResult compresses a tool result message.
// toolName identifies the tool type for specialized extraction.
// Returns the compressed content string (original is not modified).
func (tc *ToolCompressor) CompressToolResult(msg core.LLMMessage, toolName string) string {
	content := msg.Content

	// If content is short enough, don't compress
	if tc.estimator.EstimateText(content) <= maxToolResultTokens {
		return content
	}

	// Try JSON parse first
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		return tc.compressJSON(data, toolName)
	}

	// Fallback to text compression
	return tc.compressText(content, maxToolResultTokens)
}

// compressJSON compresses a parsed JSON tool result based on tool type.
func (tc *ToolCompressor) compressJSON(data map[string]interface{}, toolName string) string {
	toolCallID := getStringField(data, "tool_call_id")
	originalTokens := tc.estimator.EstimateText(mustMarshal(data))

	var retained map[string]interface{}

	switch toolName {
	case "QueryHistoricalLogs":
		retained = tc.compressQueryHistoricalLogs(data)
	case "GetProcessTree":
		retained = tc.compressGetProcessTree(data)
	case "GetNetworkConnections":
		retained = tc.compressGetNetworkConnections(data)
	case "GetOpenFiles":
		retained = tc.compressGetOpenFiles(data)
	case "GetRunningProcesses":
		retained = tc.compressGetRunningProcesses(data)
	case "GetUserSessions":
		retained = tc.compressGetUserSessions(data)
	default:
		// Unknown tool: keep top-level keys but truncate arrays
		retained = tc.compressGenericJSON(data)
	}

	retainedJSON := mustMarshalIndent(retained)
	retainedTokens := tc.estimator.EstimateText(retainedJSON)
	omitted := originalTokens - retainedTokens
	if omitted < 0 {
		omitted = 0
	}

	return fmt.Sprintf("Observation from %s compressed:\n- tool_call_id: %s\n- original_result_tokens: %d\n- retained_evidence:\n%s\n- omitted: %d tokens of routine data",
		toolName, toolCallID, originalTokens, retainedJSON, omitted)
}

// compressQueryHistoricalLogs preserves: time_range, hit_count, key_timeline, anomalous_logs
func (tc *ToolCompressor) compressQueryHistoricalLogs(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, key := range []string{"time_range", "hit_count", "key_timeline", "anomalous_logs"} {
		if v, ok := data[key]; ok {
			result[key] = v
		}
	}
	return result
}

// compressGetProcessTree preserves: suspicious processes, pid/ppid/name/user/cmdline
func (tc *ToolCompressor) compressGetProcessTree(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if v, ok := data["suspicious"]; ok {
		result["suspicious"] = v
	}
	if procs, ok := data["processes"].([]interface{}); ok {
		// Keep only suspicious-related processes (up to 20)
		if len(procs) > 20 {
			result["processes_count"] = len(procs)
			result["processes_sample"] = procs[:20]
		} else {
			result["processes"] = procs
		}
	}
	return result
}

// compressGetNetworkConnections preserves: anomalous_outbound, listening_ports, suspicious connections
func (tc *ToolCompressor) compressGetNetworkConnections(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, key := range []string{"anomalous_outbound", "listening_ports"} {
		if v, ok := data[key]; ok {
			result[key] = v
		}
	}
	if conns, ok := data["connections"].([]interface{}); ok {
		// Keep first 20 connections
		if len(conns) > 20 {
			result["connections_count"] = len(conns)
			result["connections_sample"] = conns[:20]
		} else {
			result["connections"] = conns
		}
	}
	return result
}

// compressGetOpenFiles preserves: sensitive_paths, temp_files, deleted_files, executable files
func (tc *ToolCompressor) compressGetOpenFiles(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, key := range []string{"sensitive_paths", "temp_files", "deleted_files", "total_count"} {
		if v, ok := data[key]; ok {
			result[key] = v
		}
	}
	if files, ok := data["files"].([]interface{}); ok {
		// Keep only sensitive/executable/temp files
		var important []interface{}
		for _, f := range files {
			if file, ok := f.(map[string]interface{}); ok {
				ftype := getStringField(file, "type")
				if ftype == "config" || ftype == "executable" || ftype == "deleted" {
					important = append(important, f)
				}
			}
		}
		if len(important) > 0 {
			result["important_files"] = important
		}
		result["files_total"] = len(files)
	}
	return result
}

// compressGetRunningProcesses preserves: suspicious processes, alert-related
func (tc *ToolCompressor) compressGetRunningProcesses(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if v, ok := data["suspicious"]; ok {
		result["suspicious"] = v
	}
	if v, ok := data["total_count"]; ok {
		result["total_count"] = v
	}
	if procs, ok := data["processes"].([]interface{}); ok {
		if len(procs) > 20 {
			result["processes_count"] = len(procs)
			result["processes_sample"] = procs[:20]
		} else {
			result["processes"] = procs
		}
	}
	return result
}

// compressGetUserSessions preserves: user, source_ip, login_time, tty, active
func (tc *ToolCompressor) compressGetUserSessions(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if v, ok := data["sessions"]; ok {
		result["sessions"] = v
	}
	if v, ok := data["total_count"]; ok {
		result["total_count"] = v
	}
	return result
}

// compressGenericJSON keeps top-level keys, truncates large arrays.
func (tc *ToolCompressor) compressGenericJSON(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		if arr, ok := v.([]interface{}); ok && len(arr) > 20 {
			result[k+"_count"] = len(arr)
			result[k+"_sample"] = arr[:20]
		} else {
			result[k] = v
		}
	}
	return result
}

// compressText truncates long text to fit within token budget.
func (tc *ToolCompressor) compressText(content string, maxTokens int) string {
	if tc.estimator.EstimateText(content) <= maxTokens {
		return content
	}

	// Calculate max chars based on token budget
	maxChars := maxTokens * 4 // ~4 chars per token
	if maxChars > maxTextResultChars {
		maxChars = maxTextResultChars
	}

	if len(content) <= maxChars {
		return content
	}

	half := maxChars / 2
	return content[:half] + "\n... [truncated] ...\n" + content[len(content)-half:]
}

// getStringField extracts a string field from a map.
func getStringField(data map[string]interface{}, key string) string {
	if v, ok := data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// mustMarshal marshals to JSON, returns empty string on error.
func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// mustMarshalIndent marshals to indented JSON, returns empty string on error.
func mustMarshalIndent(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

// CompressToolResults compresses tool result messages in a message list.
// Returns a new message list with compressed tool results.
func (tc *ToolCompressor) CompressToolResults(messages []core.LLMMessage) []core.LLMMessage {
	result := make([]core.LLMMessage, len(messages))
	copy(result, messages)

	for i := range result {
		if result[i].Role == "tool" {
			// Try to extract tool name from the content
			toolName := extractToolName(result[i].Content)
			result[i].Content = tc.CompressToolResult(result[i], toolName)
		}
	}

	return result
}

// extractToolName tries to extract tool_name from JSON content.
func extractToolName(content string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return ""
	}
	return getStringField(data, "tool_name")
}
