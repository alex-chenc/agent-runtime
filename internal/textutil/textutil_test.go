package textutil

import (
	"testing"
)

func TestTruncate_ShortText(t *testing.T) {
	got := Truncate("hello", 10)
	if got != "hello" {
		t.Errorf("Truncate(\"hello\", 10) = %q, want \"hello\"", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := Truncate("hello", 5)
	if got != "hello" {
		t.Errorf("Truncate(\"hello\", 5) = %q, want \"hello\"", got)
	}
}

func TestTruncate_NeedsTruncation(t *testing.T) {
	got := Truncate("hello world", 8)
	if got != "hello..." {
		t.Errorf("Truncate(\"hello world\", 8) = %q, want \"hello...\"", got)
	}
}

func TestTruncate_MaxLen3(t *testing.T) {
	got := Truncate("hello", 3)
	if got != "hel" {
		t.Errorf("Truncate(\"hello\", 3) = %q, want \"hel\"", got)
	}
}

func TestTruncate_MaxLen4(t *testing.T) {
	got := Truncate("hello world", 4)
	if got != "h..." {
		t.Errorf("Truncate(\"hello world\", 4) = %q, want \"h...\"", got)
	}
}

func TestTruncate_EmptyText(t *testing.T) {
	got := Truncate("", 5)
	if got != "" {
		t.Errorf("Truncate(\"\", 5) = %q, want \"\"", got)
	}
}

func TestSummarizeJSON_Valid(t *testing.T) {
	v := map[string]string{"key": "value"}
	got := SummarizeJSON(v, 100)
	if got != `{"key":"value"}` {
		t.Errorf("SummarizeJSON = %q", got)
	}
}

func TestSummarizeJSON_Truncated(t *testing.T) {
	v := map[string]string{"key": "a long value here"}
	got := SummarizeJSON(v, 15)
	if len(got) > 15 {
		t.Errorf("SummarizeJSON length = %d, want <= 15", len(got))
	}
}

func TestSummarizeJSON_Unmarshalable(t *testing.T) {
	got := SummarizeJSON(func() {}, 100)
	if got != "<marshal error>" {
		t.Errorf("SummarizeJSON = %q, want \"<marshal error>\"", got)
	}
}

func TestNormalizeWhitespace_MultipleSpaces(t *testing.T) {
	got := NormalizeWhitespace("hello   world")
	if got != "hello world" {
		t.Errorf("NormalizeWhitespace = %q, want \"hello world\"", got)
	}
}

func TestNormalizeWhitespace_TabsAndNewlines(t *testing.T) {
	got := NormalizeWhitespace("hello\t\t\nworld")
	if got != "hello world" {
		t.Errorf("NormalizeWhitespace = %q, want \"hello world\"", got)
	}
}

func TestNormalizeWhitespace_Empty(t *testing.T) {
	got := NormalizeWhitespace("")
	if got != "" {
		t.Errorf("NormalizeWhitespace(\"\") = %q, want \"\"", got)
	}
}

func TestNormalizeWhitespace_LeadingTrailing(t *testing.T) {
	got := NormalizeWhitespace("  hello world  ")
	if got != "hello world" {
		t.Errorf("NormalizeWhitespace = %q, want \"hello world\"", got)
	}
}

func TestExtractJSON_PlainObject(t *testing.T) {
	input := `{"key": "value"}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

func TestExtractJSON_EmbeddedInProse(t *testing.T) {
	input := `Here is the result: {"drifted": false, "decision": "continue"} and more text`
	got := ExtractJSON(input)
	want := `{"drifted": false, "decision": "continue"}`
	if got != want {
		t.Errorf("ExtractJSON = %q, want %q", got, want)
	}
}

func TestExtractJSON_NestedObjects(t *testing.T) {
	input := `{"outer": {"inner": "value"}}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

func TestExtractJSON_StringsContainingBraces(t *testing.T) {
	input := `{"key": "va{lue}"}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

func TestExtractJSON_EscapedQuotes(t *testing.T) {
	input := `{"key": "va\"lue"}`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	got := ExtractJSON("no json here")
	if got != "" {
		t.Errorf("ExtractJSON = %q, want \"\"", got)
	}
}

func TestExtractJSON_Array(t *testing.T) {
	input := `[1, 2, 3]`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("ExtractJSON = %q, want %q", got, input)
	}
}

func TestExtractJSON_UnbalancedBraces(t *testing.T) {
	got := ExtractJSON(`{"key": "value"`)
	if got != "" {
		t.Errorf("ExtractJSON(unbalanced) = %q, want \"\"", got)
	}
}

func TestExtractJSON_ObjectBeforeArray(t *testing.T) {
	input := `text {"a": 1} [2, 3]`
	got := ExtractJSON(input)
	want := `{"a": 1}`
	if got != want {
		t.Errorf("ExtractJSON = %q, want %q", got, want)
	}
}

func TestExtractJSON_MarkdownWrapped(t *testing.T) {
	input := "```json\n{\"result\": \"ok\"}\n```"
	got := ExtractJSON(input)
	want := `{"result": "ok"}`
	if got != want {
		t.Errorf("ExtractJSON = %q, want %q", got, want)
	}
}
