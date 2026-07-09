package tool

import (
	"context"
	"errors"
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

type recordingGateway struct {
	called      bool
	lastRequest core.ToolRequest
}

func (g *recordingGateway) Call(_ context.Context, req core.ToolRequest) (core.ToolResponse, error) {
	g.called = true
	g.lastRequest = req
	return core.ToolResponse{
		CallID:   req.CallID,
		ToolName: req.ToolName,
		Status:   core.ToolCallSuccess,
		Summary:  "ok",
	}, nil
}

type preparingGateway struct {
	recordingGateway
}

func (g *preparingGateway) Prepare(_ context.Context, req core.ToolRequest) (core.ToolRequest, error) {
	if req.Args == nil {
		req.Args = map[string]any{}
	}
	req.Args["query"] = "CVE-2021-45340"
	return req, nil
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
	var validationErr *core.ToolCallValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want ToolCallValidationError", err)
	}
	if validationErr.Stage != core.ToolValidationArguments {
		t.Fatalf("validation stage = %q, want %q", validationErr.Stage, core.ToolValidationArguments)
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

func TestGatewayWrapper_PreparesDerivedArgsBeforeValidation(t *testing.T) {
	registry, err := NewRegistry([]core.ToolDescriptor{{
		Name:      "search",
		RiskLevel: core.RiskReadOnly,
		ArgsSchema: map[string]any{
			"type":     "object",
			"required": []any{"query"},
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	gateway := &preparingGateway{}
	wrapper := NewGatewayWrapper(gateway, registry, &DefaultPolicy{}, nil, 0)
	_, err = wrapper.CallValidated(context.Background(), core.ToolRequest{
		TaskID:   "task-1",
		StepID:   "step-1",
		ToolName: "search",
		Args:     map[string]any{},
	})
	if err != nil {
		t.Fatalf("prepared request was rejected: %v", err)
	}
	if gateway.lastRequest.Args["query"] != "CVE-2021-45340" {
		t.Fatalf("prepared args = %#v", gateway.lastRequest.Args)
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
