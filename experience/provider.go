package experience

import (
	"context"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
)

// NullProvider is a no-op experience provider that returns empty results.
type NullProvider struct{}

func (NullProvider) Fetch(_ context.Context, _ core.ExperienceRequest) (core.ExperienceResponse, error) {
	return core.ExperienceResponse{}, nil
}

// StaticProvider returns a fixed set of experience items.
type StaticProvider struct {
	Items []core.ExperienceItem
}

func (p *StaticProvider) Fetch(_ context.Context, req core.ExperienceRequest) (core.ExperienceResponse, error) {
	if req.MaxItems > 0 && len(p.Items) > req.MaxItems {
		return core.ExperienceResponse{Items: p.Items[:req.MaxItems]}, nil
	}
	return core.ExperienceResponse{Items: p.Items}, nil
}

// ErrProvider always returns an error.
type ErrProvider struct {
	Err error
}

func (p *ErrProvider) Fetch(_ context.Context, _ core.ExperienceRequest) (core.ExperienceResponse, error) {
	if p.Err != nil {
		return core.ExperienceResponse{}, p.Err
	}
	return core.ExperienceResponse{}, fmt.Errorf("experience provider: not available")
}
