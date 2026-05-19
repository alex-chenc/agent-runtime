package tool

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestRegistry_NewRegistry(t *testing.T) {
	tools := []core.ToolDescriptor{
		{Name: "grep", Description: "search files"},
		{Name: "find", Description: "find files"},
	}
	r, err := NewRegistry(tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.List()) != 2 {
		t.Errorf("list count = %d, want 2", len(r.List()))
	}
}

func TestRegistry_DuplicateName(t *testing.T) {
	tools := []core.ToolDescriptor{
		{Name: "grep", Description: "a"},
		{Name: "grep", Description: "b"},
	}
	_, err := NewRegistry(tools)
	if err == nil {
		t.Error("expected error for duplicate tool name")
	}
}

func TestRegistry_EmptyName(t *testing.T) {
	tools := []core.ToolDescriptor{
		{Name: "", Description: "no name"},
	}
	_, err := NewRegistry(tools)
	if err == nil {
		t.Error("expected error for empty tool name")
	}
}

func TestRegistry_Get(t *testing.T) {
	tools := []core.ToolDescriptor{{Name: "grep", Description: "search"}}
	r, _ := NewRegistry(tools)
	d, err := r.Get("grep")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "grep" {
		t.Errorf("name = %q", d.Name)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r, _ := NewRegistry(nil)
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}

func TestRegistry_Has(t *testing.T) {
	tools := []core.ToolDescriptor{{Name: "grep"}}
	r, _ := NewRegistry(tools)
	if !r.Has("grep") {
		t.Error("expected Has(grep) = true")
	}
	if r.Has("find") {
		t.Error("expected Has(find) = false")
	}
}

func TestRegistry_Names(t *testing.T) {
	tools := []core.ToolDescriptor{
		{Name: "grep"},
		{Name: "find"},
	}
	r, _ := NewRegistry(tools)
	names := r.Names()
	if len(names) != 2 {
		t.Errorf("names count = %d, want 2", len(names))
	}
}
