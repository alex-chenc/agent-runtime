package experience

import (
	"context"
	"errors"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestNullProvider_Fetch(t *testing.T) {
	p := NullProvider{}
	resp, err := p.Fetch(context.Background(), core.ExperienceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Errorf("NullProvider returned %d items, want 0", len(resp.Items))
	}
}

func TestStaticProvider_Fetch_All(t *testing.T) {
	items := []core.ExperienceItem{
		{Summary: "a"},
		{Summary: "b"},
		{Summary: "c"},
	}
	p := &StaticProvider{Items: items}
	resp, err := p.Fetch(context.Background(), core.ExperienceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 3 {
		t.Errorf("returned %d items, want 3", len(resp.Items))
	}
}

func TestStaticProvider_Fetch_MaxItems(t *testing.T) {
	items := []core.ExperienceItem{
		{Summary: "a"},
		{Summary: "b"},
		{Summary: "c"},
	}
	p := &StaticProvider{Items: items}
	resp, err := p.Fetch(context.Background(), core.ExperienceRequest{MaxItems: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("returned %d items, want 2", len(resp.Items))
	}
	if resp.Items[0].Summary != "a" || resp.Items[1].Summary != "b" {
		t.Errorf("items = %v, want [a, b]", resp.Items)
	}
}

func TestStaticProvider_Fetch_MaxZeroMeansNoLimit(t *testing.T) {
	items := []core.ExperienceItem{
		{Summary: "a"},
		{Summary: "b"},
	}
	p := &StaticProvider{Items: items}
	resp, err := p.Fetch(context.Background(), core.ExperienceRequest{MaxItems: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("returned %d items, want 2", len(resp.Items))
	}
}

func TestStaticProvider_Fetch_EmptyItems(t *testing.T) {
	p := &StaticProvider{}
	resp, err := p.Fetch(context.Background(), core.ExperienceRequest{MaxItems: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Errorf("returned %d items, want 0", len(resp.Items))
	}
}

func TestErrProvider_Fetch_WithErr(t *testing.T) {
	expected := errors.New("test error")
	p := &ErrProvider{Err: expected}
	_, err := p.Fetch(context.Background(), core.ExperienceRequest{})
	if err != expected {
		t.Errorf("error = %v, want %v", err, expected)
	}
}

func TestErrProvider_Fetch_NilErr(t *testing.T) {
	p := &ErrProvider{}
	_, err := p.Fetch(context.Background(), core.ExperienceRequest{})
	if err == nil {
		t.Error("expected default error, got nil")
	}
}
