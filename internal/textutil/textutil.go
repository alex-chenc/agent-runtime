package textutil

import (
	"encoding/json"
	"strings"
)

// Truncate truncates text to maxLen characters, appending "..." if truncated.
func Truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

// SummarizeJSON returns a short summary of a JSON value.
func SummarizeJSON(v any, maxLen int) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<marshal error>"
	}
	return Truncate(string(b), maxLen)
}

// NormalizeWhitespace collapses multiple whitespace characters into single spaces.
func NormalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ExtractJSON attempts to extract a JSON object or array from text.
// Returns the first balanced JSON block found, or empty string.
func ExtractJSON(text string) string {
	// Try to find a JSON object
	start := strings.Index(text, "{")
	if start >= 0 {
		if end := findBalanced(text, start, '{', '}'); end > start {
			return text[start : end+1]
		}
	}
	// Try to find a JSON array
	start = strings.Index(text, "[")
	if start >= 0 {
		if end := findBalanced(text, start, '[', ']'); end > start {
			return text[start : end+1]
		}
	}
	return ""
}

func findBalanced(text string, start int, open, close byte) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
