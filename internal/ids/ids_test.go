package ids

import (
	"regexp"
	"testing"
)

func TestGenerator_Format(t *testing.T) {
	var g Generator
	id := g.Generate()
	pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	matched, err := regexp.MatchString(pattern, id)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Errorf("Generate() = %q, does not match UUID pattern", id)
	}
}

func TestGenerator_Uniqueness(t *testing.T) {
	var g Generator
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := g.Generate()
		if seen[id] {
			t.Errorf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestGenerator_NoPanic(t *testing.T) {
	var g Generator
	for i := 0; i < 1000; i++ {
		g.Generate()
	}
}
