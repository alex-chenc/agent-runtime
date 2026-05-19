package tool

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
)

// GatewayWrapper wraps an core.ToolGateway with pre-call validation.
type GatewayWrapper struct {
	gateway  core.ToolGateway
	registry *Registry
	policy   core.ToolPolicy
	idGen    core.IDGenerator
	timeout  time.Duration
}

// NewGatewayWrapper creates a new tool gateway wrapper.
func NewGatewayWrapper(
	gw core.ToolGateway,
	registry *Registry,
	policy core.ToolPolicy,
	idGen core.IDGenerator,
	defaultTimeout time.Duration,
) *GatewayWrapper {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &GatewayWrapper{
		gateway:  gw,
		registry: registry,
		policy:   policy,
		idGen:    idGen,
		timeout:  defaultTimeout,
	}
}

// CallValidated validates and executes a tool call.
// It checks: tool exists, policy allows, then calls the gateway.
func (w *GatewayWrapper) CallValidated(ctx context.Context, req core.ToolRequest) (core.ToolResponse, error) {
	// 1. Check tool exists
	desc, err := w.registry.Get(req.ToolName)
	if err != nil {
		return core.ToolResponse{}, fmt.Errorf("tool validation: %w", err)
	}
	if err := validateArgs(desc.ArgsSchema, req.Args); err != nil {
		return core.ToolResponse{}, fmt.Errorf("tool validation: %w", err)
	}

	// 2. Check policy
	if w.policy != nil {
		decision, err := w.policy.Evaluate(ctx, core.ToolPolicyRequest{
			TaskID:    req.TaskID,
			StepID:    req.StepID,
			ToolName:  req.ToolName,
			Args:      req.Args,
			RiskLevel: desc.RiskLevel,
		})
		if err != nil {
			return core.ToolResponse{}, fmt.Errorf("tool policy evaluation: %w", err)
		}
		if decision != core.PolicyAllow {
			return core.ToolResponse{}, fmt.Errorf("tool %q denied by policy: %s", req.ToolName, decision)
		}
	}

	// 3. Set defaults
	if req.CallID == "" {
		req.CallID = w.idGen.Generate()
	}
	if req.Timeout <= 0 {
		if desc.DefaultTimeout > 0 {
			req.Timeout = desc.DefaultTimeout
		} else {
			req.Timeout = w.timeout
		}
	}
	req.RiskLevel = desc.RiskLevel

	// 4. Call gateway
	return w.gateway.Call(ctx, req)
}

// Cancel delegates to the underlying gateway.
func (w *GatewayWrapper) Cancel(ctx context.Context, taskID string, callID string) error {
	return w.gateway.Cancel(ctx, taskID, callID)
}

func validateArgs(schema map[string]any, args map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	return validateValue("$", args, schema)
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func matchesJSONType(value any, want string) bool {
	if value == nil {
		return false
	}
	switch want {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true
		default:
			return false
		}
	case "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "object":
		return reflect.ValueOf(value).Kind() == reflect.Map
	case "array":
		return reflect.ValueOf(value).Kind() == reflect.Slice
	default:
		return true
	}
}

func validateValue(path string, value any, schema map[string]any) error {
	for _, allowed := range anySlice(schema["enum"]) {
		if reflect.DeepEqual(value, allowed) {
			return nil
		}
	}
	if enumValues := anySlice(schema["enum"]); len(enumValues) > 0 {
		return fmt.Errorf("%s value %v is not in enum", path, value)
	}

	want, _ := schema["type"].(string)
	if want == "" && schema["properties"] != nil {
		want = "object"
	}
	if want != "" && !matchesJSONType(value, want) {
		return fmt.Errorf("%s has type %s, want %s", path, jsonType(value), want)
	}

	switch want {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s has type %s, want object", path, jsonType(value))
		}
		for _, field := range stringSlice(schema["required"]) {
			if _, ok := obj[field]; !ok {
				return fmt.Errorf("missing required argument %q", trimPath(path, field))
			}
		}
		props, _ := schema["properties"].(map[string]any)
		for name, rawSpec := range props {
			child, ok := obj[name]
			if !ok {
				continue
			}
			spec, _ := rawSpec.(map[string]any)
			if len(spec) == 0 {
				continue
			}
			if err := validateValue(path+"."+name, child, spec); err != nil {
				return err
			}
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			for name := range obj {
				if _, ok := props[name]; !ok {
					return fmt.Errorf("%s.%s is not allowed by schema", path, name)
				}
			}
		}
	case "array":
		items, ok := toSlice(value)
		if !ok {
			return fmt.Errorf("%s has type %s, want array", path, jsonType(value))
		}
		itemSchema, _ := schema["items"].(map[string]any)
		if len(itemSchema) == 0 {
			return nil
		}
		for i, item := range items {
			if err := validateValue(fmt.Sprintf("%s[%d]", path, i), item, itemSchema); err != nil {
				return err
			}
		}
	case "number", "integer":
		if min, ok := asFloat(schema["minimum"]); ok {
			actual, _ := asFloat(value)
			if actual < min {
				return fmt.Errorf("%s must be >= %v", path, min)
			}
		}
		if max, ok := asFloat(schema["maximum"]); ok {
			actual, _ := asFloat(value)
			if actual > max {
				return fmt.Errorf("%s must be <= %v", path, max)
			}
		}
	}
	return nil
}

func anySlice(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func toSlice(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, true
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() || rv.Kind() != reflect.Slice {
			return nil, false
		}
		out := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out = append(out, rv.Index(i).Interface())
		}
		return out, true
	}
}

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func trimPath(path, field string) string {
	if path == "$" {
		return field
	}
	return path + "." + field
}

func jsonType(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	default:
		kind := reflect.ValueOf(value).Kind()
		if kind == reflect.Map {
			return "object"
		}
		if kind == reflect.Slice {
			return "array"
		}
		return kind.String()
	}
}
