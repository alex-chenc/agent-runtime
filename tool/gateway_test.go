package tool

import (
	"context"
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

type recordingGateway struct {
	called bool
}

func (g *recordingGateway) Call(_ context.Context, req core.ToolRequest) (core.ToolResponse, error) {
	g.called = true
	return core.ToolResponse{
		CallID:   req.CallID,
		ToolName: req.ToolName,
		Status:   core.ToolCallSuccess,
		Summary:  "ok",
	}, nil
}

func (g *recordingGateway) Cancel(context.Context, string, string) error {
	return nil
}

func TestGatewayWrapper_ValidatesRequiredArgs(t *testing.T) {
	registry, err := NewRegistry([]core.ToolDescriptor{{
		Name:      "search",
		RiskLevel: core.RiskReadOnly,
		ArgsSchema: map[string]any{
			"required": []any{"query"},
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	gw := &recordingGateway{}
	wrapper := NewGatewayWrapper(gw, registry, &DefaultPolicy{}, nil, 0)

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "search",
		Args:     map[string]any{},
	})
	if err == nil {
		t.Fatal("expected validation error for missing required arg")
	}
	if gw.called {
		t.Fatal("gateway should not be called when validation fails")
	}
}

func TestGatewayWrapper_ValidatesArgTypes(t *testing.T) {
	registry, err := NewRegistry([]core.ToolDescriptor{{
		Name:      "search",
		RiskLevel: core.RiskReadOnly,
		ArgsSchema: map[string]any{
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wrapper := NewGatewayWrapper(&recordingGateway{}, registry, &DefaultPolicy{}, nil, 0)

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "search",
		Args:     map[string]any{"query": 123},
	})
	if err == nil {
		t.Fatal("expected validation error for wrong arg type")
	}
}

func TestGatewayWrapper_ValidatesEnumAdditionalPropertiesAndNestedItems(t *testing.T) {
	registry, err := NewRegistry([]core.ToolDescriptor{{
		Name:      "scan",
		RiskLevel: core.RiskReadOnly,
		ArgsSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"mode", "targets"},
			"properties": map[string]any{
				"mode": map[string]any{
					"type": "string",
					"enum": []any{"quick", "full"},
				},
				"targets": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"host": map[string]any{"type": "string"},
							"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
						},
						"required":             []any{"host", "port"},
						"additionalProperties": false,
					},
				},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wrapper := NewGatewayWrapper(&recordingGateway{}, registry, &DefaultPolicy{}, nil, 0)

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "scan",
		Args: map[string]any{
			"mode": "quick",
			"targets": []any{
				map[string]any{"host": "127.0.0.1", "port": 22},
			},
		},
	})
	if err != nil {
		t.Fatalf("valid nested args rejected: %v", err)
	}

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "scan",
		Args: map[string]any{
			"mode":    "slow",
			"targets": []any{map[string]any{"host": "127.0.0.1", "port": 22}},
		},
	})
	if err == nil {
		t.Fatal("expected enum validation error")
	}

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "scan",
		Args: map[string]any{
			"mode":    "quick",
			"targets": []any{map[string]any{"host": "127.0.0.1", "port": 70000}},
		},
	})
	if err == nil {
		t.Fatal("expected nested maximum validation error")
	}

	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "scan",
		Args: map[string]any{
			"mode":       "quick",
			"targets":    []any{map[string]any{"host": "127.0.0.1", "port": 22}},
			"unexpected": true,
		},
	})
	if err == nil {
		t.Fatal("expected additionalProperties validation error")
	}
}
