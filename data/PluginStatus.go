package data

import "time"

// PluginStatus describes the current, overall state of the xfinitywifi plugin.
type PluginStatus struct {
	Running               bool
	Connected             bool
	CaptivePortalDetected bool
	Username              string
	CheckInterval         time.Duration
	LastCheckTime         time.Time
	TotalChecks           int
	TotalConnectedChecks  int
	TotalReauthAttempts   int
	TotalReauthSuccesses  int
	LastErrorMessage      string
	LastErrorTime         time.Time
}
