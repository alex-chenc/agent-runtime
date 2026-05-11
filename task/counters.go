package task

// Counters tracks runtime counters for a task.
type Counters struct {
	ToolCalls       int `json:"tool_calls"`
	ToolFailures    int `json:"tool_failures"`
	ModelCalls      int `json:"model_calls"`
	ModelFailures   int `json:"model_failures"`
	ParseFailures   int `json:"parse_failures"`
	NoProgressTurns int `json:"no_progress_turns"`
	TotalTurns      int `json:"total_turns"`
	Audits          int `json:"audits"`
	Reflections     int `json:"reflections"`
	Corrections     int `json:"corrections"`
}
