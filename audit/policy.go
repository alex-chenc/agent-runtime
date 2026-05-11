package audit

// Policy determines when to trigger an audit.
type Policy struct {
	everyNSteps int
	everyNTurns int
}

// NewPolicy creates an audit policy.
func NewPolicy(everyNSteps, everyNTurns int) *Policy {
	return &Policy{everyNSteps: everyNSteps, everyNTurns: everyNTurns}
}

// ShouldAuditBySteps returns true if an audit should run based on completed steps.
func (p *Policy) ShouldAuditBySteps(completedSteps int) bool {
	if p.everyNSteps <= 0 {
		return false
	}
	return completedSteps > 0 && completedSteps%p.everyNSteps == 0
}

// ShouldAuditByTurns returns true if an audit should run based on total turns.
func (p *Policy) ShouldAuditByTurns(totalTurns int) bool {
	if p.everyNTurns <= 0 {
		return false
	}
	return totalTurns > 0 && totalTurns%p.everyNTurns == 0
}
