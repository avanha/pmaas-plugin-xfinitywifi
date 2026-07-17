package data

import "time"

// ProbeTargetResult captures the outcome of probing a single target.
type ProbeTargetResult struct {
	Target     string
	StatusCode int
	Success    bool
	Redirected bool
	Error      string
}

// ProbeAttempt captures information about the most recent connectivity probe.
type ProbeAttempt struct {
	Time                  time.Time
	Connected             bool
	CaptivePortalDetected bool
	SuccessCount          int
	RequiredSuccesses     int
	Targets               []ProbeTargetResult
}
