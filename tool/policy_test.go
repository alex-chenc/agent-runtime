package tool

import (
	"context"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestDefaultPolicy_ReadOnly(t *testing.T) {
	p := &DefaultPolicy{}
	decision, err := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		ToolName:  "grep",
		RiskLevel: core.RiskReadOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision != core.PolicyAllow {
		t.Errorf("decision = %q, want allow", decision)
	}
}

func TestDefaultPolicy_Low(t *testing.T) {
	p := &DefaultPolicy{}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		RiskLevel: core.RiskLow,
	})
	if decision != core.PolicyAllow {
		t.Errorf("decision = %q, want allow", decision)
	}
}

func TestDefaultPolicy_High_Denied(t *testing.T) {
	p := &DefaultPolicy{AllowHighRisk: false}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		RiskLevel: core.RiskHigh,
	})
	if decision != core.PolicyDeny {
		t.Errorf("decision = %q, want deny", decision)
	}
}

func TestDefaultPolicy_High_Allowed(t *testing.T) {
	p := &DefaultPolicy{AllowHighRisk: true}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		RiskLevel: core.RiskHigh,
	})
	if decision != core.PolicyAllow {
		t.Errorf("decision = %q, want allow", decision)
	}
}

func TestDefaultPolicy_Dangerous_Denied(t *testing.T) {
	p := &DefaultPolicy{AllowHighRisk: true, AllowDangerous: false}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		RiskLevel: core.RiskDangerous,
	})
	if decision != core.PolicyDeny {
		t.Errorf("decision = %q, want deny", decision)
	}
}

func TestDefaultPolicy_Dangerous_Allowed(t *testing.T) {
	p := &DefaultPolicy{AllowHighRisk: true, AllowDangerous: true}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		RiskLevel: core.RiskDangerous,
	})
	if decision != core.PolicyAllow {
		t.Errorf("decision = %q, want allow", decision)
	}
}

func TestDefaultPolicy_DisabledTool(t *testing.T) {
	p := &DefaultPolicy{DisabledTools: []string{"evil_cmd"}}
	decision, _ := p.Evaluate(context.Background(), core.ToolPolicyRequest{
		ToolName:  "evil_cmd",
		RiskLevel: core.RiskReadOnly,
	})
	if decision != core.PolicyDeny {
		t.Errorf("decision = %q, want deny for disabled tool", decision)
	}
}
