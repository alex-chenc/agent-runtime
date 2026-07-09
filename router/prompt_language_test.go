package router

import (
	"regexp"
	"testing"
)

func TestClassificationStaticPromptIsEnglish(t *testing.T) {
	prompt := buildClassificationSystemPrompt("- base_assistant: Base assistant")
	if regexp.MustCompile(`[\p{Han}]`).MatchString(prompt) {
		t.Fatalf("classification prompt contains Han characters:\n%s", prompt)
	}
}
