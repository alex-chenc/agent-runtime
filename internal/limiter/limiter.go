package limiter

// Limiter tracks counts and enforces limits.
type Limiter struct {
	toolCalls       int
	toolFailures    int
	modelCalls      int
	modelFailures   int
	parseFailures   int
	noProgressTurns int
	totalTurns      int
	audits          int
	reflections     int
	corrections     int
}

func (l *Limiter) IncrToolCalls()      { l.toolCalls++ }
func (l *Limiter) IncrToolFailures()   { l.toolFailures++ }
func (l *Limiter) IncrModelCalls()     { l.modelCalls++ }
func (l *Limiter) IncrModelFailures()  { l.modelFailures++ }
func (l *Limiter) IncrParseFailures()  { l.parseFailures++ }
func (l *Limiter) ResetParseFailures() { l.parseFailures = 0 }
func (l *Limiter) IncrNoProgress()     { l.noProgressTurns++ }
func (l *Limiter) ResetNoProgress()    { l.noProgressTurns = 0 }
func (l *Limiter) IncrTotalTurns()     { l.totalTurns++ }
func (l *Limiter) IncrAudits()         { l.audits++ }
func (l *Limiter) IncrReflections()    { l.reflections++ }
func (l *Limiter) IncrCorrections()    { l.corrections++ }

func (l *Limiter) ToolCalls() int       { return l.toolCalls }
func (l *Limiter) ToolFailures() int    { return l.toolFailures }
func (l *Limiter) ModelCalls() int      { return l.modelCalls }
func (l *Limiter) ModelFailures() int   { return l.modelFailures }
func (l *Limiter) ParseFailures() int   { return l.parseFailures }
func (l *Limiter) NoProgressTurns() int { return l.noProgressTurns }
func (l *Limiter) TotalTurns() int      { return l.totalTurns }
func (l *Limiter) Audits() int          { return l.audits }
func (l *Limiter) Reflections() int     { return l.reflections }
func (l *Limiter) Corrections() int     { return l.corrections }

// ExceedsToolCalls returns true if tool calls exceed the limit.
func (l *Limiter) ExceedsToolCalls(max int) bool     { return l.toolCalls >= max }
func (l *Limiter) ExceedsToolFailures(max int) bool  { return l.toolFailures >= max }
func (l *Limiter) ExceedsModelFailures(max int) bool { return l.modelFailures >= max }
func (l *Limiter) ExceedsParseFailures(max int) bool { return l.parseFailures >= max }
func (l *Limiter) ExceedsNoProgress(max int) bool    { return l.noProgressTurns >= max }
func (l *Limiter) ExceedsTotalTurns(max int) bool    { return l.totalTurns >= max }
func (l *Limiter) ExceedsAudits(max int) bool        { return l.audits >= max }
func (l *Limiter) ExceedsReflections(max int) bool   { return l.reflections >= max }
func (l *Limiter) ExceedsCorrections(max int) bool   { return l.corrections >= max }
