package task

import (
	"sync"
	"testing"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestContext_SetInterrupted_IsInterrupted(t *testing.T) {
	c := &Context{}
	if c.IsInterrupted() {
		t.Error("should not be interrupted initially")
	}
	c.SetInterrupted("user cancel")
	if !c.IsInterrupted() {
		t.Error("should be interrupted after SetInterrupted")
	}
}

func TestContext_SetStatus(t *testing.T) {
	c := &Context{}
	c.SetStatus(core.StatusRunning)
	c.Lock()
	got := c.Status
	c.Unlock()
	if got != core.StatusRunning {
		t.Errorf("Status = %q, want %q", got, core.StatusRunning)
	}
}

func TestContext_SetConfigField(t *testing.T) {
	c := &Context{}
	c.SetConfigField("MaxTotalTurns", "50", "100")
	c.Lock()
	n := len(c.ConfigChanges)
	c.Unlock()
	if n != 1 {
		t.Errorf("ConfigChanges count = %d, want 1", n)
	}
	c.Lock()
	cc := c.ConfigChanges[0]
	c.Unlock()
	if cc.Field != "MaxTotalTurns" || cc.OldValue != "50" || cc.NewValue != "100" {
		t.Errorf("ConfigChange = %+v", cc)
	}
}

func TestContext_BatchUpdate(t *testing.T) {
	c := &Context{
		TaskID: "task-1",
		Input:  TaskInput{UserInput: "test"},
	}
	c.Steps = []core.StepExecution{
		{StepID: "s1", Status: core.StepCompleted},
		{StepID: "s2", Status: core.StepFailed},
		{StepID: "s3", Status: core.StepRunning},
	}
	c.Counters.ToolCalls = 5
	c.Counters.ModelCalls = 3

	snap := c.BatchUpdate(func() {
		c.Status = core.StatusRunning
		c.CurrentStepID = "s3"
	})

	if snap.TaskID != "task-1" {
		t.Errorf("snap.TaskID = %q", snap.TaskID)
	}
	if snap.CompletedSteps != 1 {
		t.Errorf("CompletedSteps = %d, want 1", snap.CompletedSteps)
	}
	if snap.FailedSteps != 1 {
		t.Errorf("FailedSteps = %d, want 1", snap.FailedSteps)
	}
	if snap.TotalToolCalls != 5 {
		t.Errorf("TotalToolCalls = %d, want 5", snap.TotalToolCalls)
	}
	if snap.TotalModelCalls != 3 {
		t.Errorf("TotalModelCalls = %d, want 3", snap.TotalModelCalls)
	}
	if snap.Status != core.StatusRunning {
		t.Errorf("Status = %q", snap.Status)
	}
}

func TestContext_Snapshot_NilPlan(t *testing.T) {
	c := &Context{TaskID: "t"}
	snap := c.Snapshot()
	if snap.CurrentPlan != nil {
		t.Error("CurrentPlan should be nil")
	}
}

func TestContext_Snapshot_PlanDeepCopy(t *testing.T) {
	c := &Context{
		TaskID: "t",
		CurrentPlan: &core.Plan{
			PlanID:  "p1",
			Version: 1,
			Steps: []core.PlanStep{
				{StepID: "s1", Title: "step 1"},
				{StepID: "s2", Title: "step 2"},
			},
		},
	}
	snap := c.Snapshot()

	c.CurrentPlan.Steps[0].Title = "modified"
	c.CurrentPlan.Version = 99

	if snap.CurrentPlan.Version != 1 {
		t.Errorf("snapshot Version = %d, want 1", snap.CurrentPlan.Version)
	}
	if snap.CurrentPlan.Steps[0].Title != "step 1" {
		t.Errorf("snapshot Steps[0].Title = %q, want \"step 1\"", snap.CurrentPlan.Steps[0].Title)
	}
}

func TestContext_Snapshot_RecentErrorsTruncation(t *testing.T) {
	c := &Context{TaskID: "t"}
	for i := 0; i < 10; i++ {
		c.Errors = append(c.Errors, core.RuntimeError{
			ErrorID: "err-" + string(rune('0'+i)),
			Message: "error",
		})
	}
	snap := c.Snapshot()
	if len(snap.RecentErrors) != 5 {
		t.Errorf("RecentErrors count = %d, want 5", len(snap.RecentErrors))
	}
}

func TestContext_Snapshot_MetadataCopy(t *testing.T) {
	c := &Context{
		TaskID: "t",
		Input: TaskInput{
			Metadata: map[string]string{"key": "value"},
		},
	}
	snap := c.Snapshot()
	snap.Metadata["key"] = "modified"

	c.Lock()
	orig := c.Input.Metadata["key"]
	c.Unlock()
	if orig != "value" {
		t.Errorf("original metadata was modified: %q", orig)
	}
}

func TestContext_ConcurrentSafety(t *testing.T) {
	c := &Context{TaskID: "t"}
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.SetStatus(core.StatusRunning)
			c.SetInterrupted("test")
			c.SetConfigField("f", "old", "new")
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.IsInterrupted()
			_ = c.Snapshot()
		}()
	}

	wg.Wait()
}

func TestContext_BatchUpdate_NilSteps(t *testing.T) {
	c := &Context{TaskID: "t"}
	snap := c.BatchUpdate(func() {
		c.Status = core.StatusCompleted
	})
	if snap.CompletedSteps != 0 {
		t.Errorf("CompletedSteps = %d, want 0", snap.CompletedSteps)
	}
}

func TestContext_SetConfigField_Multiple(t *testing.T) {
	c := &Context{}
	c.SetConfigField("A", "1", "2")
	c.SetConfigField("B", "3", "4")
	c.SetConfigField("C", "5", "6")
	c.Lock()
	n := len(c.ConfigChanges)
	c.Unlock()
	if n != 3 {
		t.Errorf("ConfigChanges count = %d, want 3", n)
	}
}

func TestContext_Snapshot_StartedAt(t *testing.T) {
	now := time.Now()
	c := &Context{
		TaskID:    "t",
		StartedAt: now,
	}
	snap := c.Snapshot()
	if !snap.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", snap.StartedAt, now)
	}
}
