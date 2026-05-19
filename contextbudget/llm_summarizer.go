package contextbudget

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alex-chenc/agent-runtime/core"
)

// LLMSummarizer compresses older conversation turns using an LLM call.
// Triggers at 95% context ratio. Keeps recent N turns verbatim.
type LLMSummarizer struct {
	client          core.LLMClient
	recentTurnsKeep int
}

// CompressionSummary is the structured JSON output from the LLM compression call.
type CompressionSummary struct {
	Summary         string         `json:"summary"`
	Facts           []string       `json:"facts"`
	Timeline        []string       `json:"timeline"`
	Evidence        []EvidenceItem `json:"evidence"`
	OpenQuestions   []string       `json:"open_questions"`
	Risks           []string       `json:"risks"`
	DiscardedDetail string         `json:"discarded_detail"`
}

// EvidenceItem represents a single piece of evidence in the compression summary.
type EvidenceItem struct {
	Source     string `json:"source"`
	Fact       string `json:"fact"`
	Confidence string `json:"confidence"`
}

// NewLLMSummarizer creates a new LLMSummarizer.
// recentTurnsKeep is the number of recent turns to preserve verbatim (default 6).
func NewLLMSummarizer(client core.LLMClient, recentTurnsKeep int) *LLMSummarizer {
	if recentTurnsKeep <= 0 {
		recentTurnsKeep = 6
	}
	return &LLMSummarizer{
		client:          client,
		recentTurnsKeep: recentTurnsKeep,
	}
}

// Summarize compresses older turns, preserving system prompt, user input, and recent turns.
// Returns a new message list with older turns replaced by a summary.
func (s *LLMSummarizer) Summarize(messages []core.LLMMessage) ([]core.LLMMessage, error) {
	if len(messages) <= 3 {
		return messages, nil
	}

	// Split: setup (system+user) vs conversation turns
	setupEnd := 0
	for i, msg := range messages {
		if msg.Role == "system" || msg.Role == "user" {
			setupEnd = i + 1
		} else {
			break
		}
	}

	setup := messages[:setupEnd]
	turns := messages[setupEnd:]

	// If not enough turns to compress, return as-is
	if len(turns) <= s.recentTurnsKeep*2 {
		return messages, nil
	}

	// Split into old (to compress) and recent (to keep)
	recentCount := s.recentTurnsKeep * 2 // each turn = assistant + tool
	oldTurns := turns[:len(turns)-recentCount]
	recentTurns := turns[len(turns)-recentCount:]

	// Call LLM to summarize old turns
	summary, err := s.callLLMSummarize(oldTurns)
	if err != nil {
		// Emergency fallback: programmatic compression
		summary = s.emergencyCompress(oldTurns)
	}

	// Reconstruct: setup + summary + recent
	result := make([]core.LLMMessage, 0, len(setup)+1+len(recentTurns))
	result = append(result, setup...)
	result = append(result, core.LLMMessage{
		Role:    "user",
		Content: summary,
	})
	result = append(result, recentTurns...)

	return result, nil
}

// callLLMSummarize calls the LLM to compress old turns.
func (s *LLMSummarizer) callLLMSummarize(turns []core.LLMMessage) (string, error) {
	prompt := s.buildCompressionPrompt(turns)

	req := core.LLMRequest{
		Purpose: core.PurposeCompress,
		Messages: []core.LLMMessage{
			{Role: "system", Content: compressionSystemPrompt},
			{Role: "user", Content: prompt},
		},
	}

	resp, err := s.client.Complete(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("LLM compression call failed: %w", err)
	}

	// Validate the response is valid JSON with required fields
	var summary CompressionSummary
	if err := json.Unmarshal([]byte(resp.Content), &summary); err != nil {
		return "", fmt.Errorf("LLM returned invalid JSON: %w", err)
	}

	return s.formatSummary(summary), nil
}

// buildCompressionPrompt builds the user prompt for the compression call.
func (s *LLMSummarizer) buildCompressionPrompt(turns []core.LLMMessage) string {
	var sb strings.Builder
	sb.WriteString("Compress the following conversation turns into a structured summary.\n")
	sb.WriteString("Preserve all security-relevant facts, evidence, and timeline.\n\n")
	sb.WriteString("Conversation to compress:\n")

	for _, msg := range turns {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}

	return sb.String()
}

// formatSummary formats a CompressionSummary as readable text.
func (s *LLMSummarizer) formatSummary(summary CompressionSummary) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Compressed Context Summary ===\n%s\n", summary.Summary))

	if len(summary.Facts) > 0 {
		sb.WriteString("\nKey Facts:\n")
		for _, f := range summary.Facts {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(summary.Timeline) > 0 {
		sb.WriteString("\nTimeline:\n")
		for _, t := range summary.Timeline {
			sb.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}

	if len(summary.Evidence) > 0 {
		sb.WriteString("\nEvidence:\n")
		for _, e := range summary.Evidence {
			sb.WriteString(fmt.Sprintf("- [%s] %s (confidence: %s)\n", e.Source, e.Fact, e.Confidence))
		}
	}

	if len(summary.OpenQuestions) > 0 {
		sb.WriteString("\nOpen Questions:\n")
		for _, q := range summary.OpenQuestions {
			sb.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}

	if len(summary.Risks) > 0 {
		sb.WriteString("\nRisks:\n")
		for _, r := range summary.Risks {
			sb.WriteString(fmt.Sprintf("- %s\n", r))
		}
	}

	if summary.DiscardedDetail != "" {
		sb.WriteString(fmt.Sprintf("\nDiscarded: %s\n", summary.DiscardedDetail))
	}

	return sb.String()
}

// emergencyCompress performs deterministic compression without LLM.
func (s *LLMSummarizer) emergencyCompress(turns []core.LLMMessage) string {
	var sb strings.Builder
	sb.WriteString("=== Emergency Compressed Context ===\n")
	sb.WriteString("Original context exceeded limits. Older turns compressed programmatically.\n\n")

	for i, msg := range turns {
		if msg.Role == "assistant" {
			content := msg.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("Turn %d: %s\n", i/2+1, content))
		} else if msg.Role == "tool" {
			sb.WriteString(fmt.Sprintf("Tool result %d: [compressed]\n", i/2+1))
		}
	}

	sb.WriteString(fmt.Sprintf("\nTotal turns compressed: %d\n", len(turns)/2))
	return sb.String()
}

const compressionSystemPrompt = `You are a context compression assistant. Your job is to summarize older conversation turns while preserving all security-relevant information.

You MUST return valid JSON with these fields:
- summary: A concise summary of what was investigated
- facts: Array of key facts discovered
- timeline: Array of timeline events
- evidence: Array of objects with {source, fact, confidence}
- open_questions: Array of unanswered questions
- risks: Array of identified risks
- discarded_detail: Description of what was discarded

Focus on preserving:
- Alert core fields (alert_id, rule_id, host_id, pid, commandline)
- Security findings and evidence
- Tool results that contain anomalies
- Timeline of events

Discard:
- Routine process listings
- Normal network statistics
- Repetitive tool outputs
- Verbose system information`
