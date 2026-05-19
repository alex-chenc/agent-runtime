package tool

import (
	"context"
	"slices"

	"github.com/alex-chenc/agent-runtime/core"
)

// DefaultPolicy implements core.ToolPolicy with standard risk-based rules.
type DefaultPolicy struct {
	AllowHighRisk  bool
	AllowDangerous bool
	DisabledTools  []string
}

// Evaluate checks whether a tool call is allowed under the policy.
func (p *DefaultPolicy) Evaluate(_ context.Context, req core.ToolPolicyRequest) (core.ToolPolicyDecision, error) {
	// Check disabled tools
	if slices.Contains(p.DisabledTools, req.ToolName) {
		return core.PolicyDeny, nil
	}

	// Check risk levels
	switch req.RiskLevel {
	case core.RiskReadOnly, core.RiskLow:
		return core.PolicyAllow, nil
	case core.RiskHigh:
		if p.AllowHighRisk {
			return core.PolicyAllow, nil
		}
		return core.PolicyDeny, nil
	case core.RiskDangerous:
		if p.AllowDangerous {
			return core.PolicyAllow, nil
		}
		return core.PolicyDeny, nil
	default:
		return core.PolicyDeny, nil
	}
}
