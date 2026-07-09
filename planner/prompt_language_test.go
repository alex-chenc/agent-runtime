package planner

import (
	"regexp"
	"testing"
)

func TestPlannerStaticPromptsAreEnglish(t *testing.T) {
	han := regexp.MustCompile(`[\p{Han}]`)
	for name, prompt := range map[string]string{
		"assessment": assessmentSystemPrompt,
		"plan":       DefaultPlanSystemPrompt,
		"json_retry": planJSONRetryPrompt,
	} {
		t.Run(name, func(t *testing.T) {
			if han.MatchString(prompt) {
				t.Fatalf("%s prompt contains Han characters:\n%s", name, prompt)
			}
		})
	}
}
