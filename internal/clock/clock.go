package clock

import "time"

// RealClock implements agentruntime.Clock using time.Now().
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
