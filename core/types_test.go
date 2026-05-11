package core

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.MaxTotalTurns != 40 {
		t.Errorf("MaxTotalTurns = %d, want 40", c.MaxTotalTurns)
	}
	if c.TaskTimeout != 10*time.Minute {
		t.Errorf("TaskTimeout = %v, want 10m", c.TaskTimeout)
	}
	if !c.EnableReflection {
		t.Error("EnableReflection should be true by default")
	}
}

func TestValidate_DefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if err := c.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestValidate_InvalidMaxTotalTurns(t *testing.T) {
	c := DefaultConfig()
	c.MaxTotalTurns = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error for MaxTotalTurns=0")
	}
}

func TestValidate_InvalidTaskTimeout(t *testing.T) {
	c := DefaultConfig()
	c.TaskTimeout = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error for TaskTimeout=0")
	}
}

func TestValidate_DangerousRequiresHighRisk(t *testing.T) {
	c := DefaultConfig()
	c.AllowDangerousTools = true
	c.AllowHighRiskTools = false
	if err := c.Validate(); err == nil {
		t.Error("expected error when AllowDangerousTools=true but AllowHighRiskTools=false")
	}
}

func TestValidate_AuditEnabledNeedsMaxAudits(t *testing.T) {
	c := DefaultConfig()
	c.EnableAudit = true
	c.MaxAudits = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error when EnableAudit=true but MaxAudits=0")
	}
}

func TestApplyPatch(t *testing.T) {
	c := DefaultConfig()
	newTurns := 100
	patch := ConfigPatch{MaxTotalTurns: &newTurns}
	result := c.ApplyPatch(patch)
	if result.MaxTotalTurns != 100 {
		t.Errorf("MaxTotalTurns = %d, want 100", result.MaxTotalTurns)
	}
	if c.MaxTotalTurns != 40 {
		t.Errorf("original MaxTotalTurns = %d, want 40", c.MaxTotalTurns)
	}
}

func TestApplyPatch_NilFields(t *testing.T) {
	c := DefaultConfig()
	patch := ConfigPatch{}
	result := c.ApplyPatch(patch)
	if result.MaxTotalTurns != c.MaxTotalTurns {
		t.Error("empty patch should not change config")
	}
}

func TestApplyPatch_DisabledTools(t *testing.T) {
	c := DefaultConfig()
	patch := ConfigPatch{DisabledTools: []string{"dangerous_tool"}}
	result := c.ApplyPatch(patch)
	if len(result.DisabledTools) != 1 || result.DisabledTools[0] != "dangerous_tool" {
		t.Errorf("DisabledTools = %v, want [dangerous_tool]", result.DisabledTools)
	}
}

func TestTaskStatusConstants(t *testing.T) {
	statuses := []TaskStatus{
		StatusInitializing, StatusPlanning, StatusPlanFailed,
		StatusRunning, StatusCompleted, StatusFailed,
		StatusInterrupted, StatusLimited,
	}
	seen := make(map[TaskStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate TaskStatus value: %q", s)
		}
		seen[s] = true
	}
}

func TestStepStatusConstants(t *testing.T) {
	statuses := []StepStatus{
		StepPending, StepRunning, StepCompleted, StepFailed, StepSkipped,
	}
	seen := make(map[StepStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate StepStatus value: %q", s)
		}
		seen[s] = true
	}
}
