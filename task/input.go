package task

import "github.com/alex-chenc/agent-runtime/core"

// TaskInput is the input for a single task execution.
type TaskInput struct {
	TaskID            string                `json:"task_id"`
	UserInput         string                `json:"user_input"`
	UserContext       map[string]any        `json:"user_context,omitempty"`
	InitialExperience []core.ExperienceItem `json:"initial_experience,omitempty"`
	Metadata          map[string]string     `json:"metadata,omitempty"`
	ConfigPatch       *core.ConfigPatch     `json:"config_patch,omitempty"`
}
