package agentruntime

import (
	"regexp"
	"testing"
)

func TestDefaultDirectReplySystemPromptIsEnglish(t *testing.T) {
	if regexp.MustCompile(`[\p{Han}]`).MatchString(defaultDirectReplySystemPrompt) {
		t.Fatalf("direct reply prompt contains Han characters: %s", defaultDirectReplySystemPrompt)
	}
}
