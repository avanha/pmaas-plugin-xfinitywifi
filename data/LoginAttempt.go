package data

import "time"

// LoginAttempt captures information about the most recent captive portal login attempt.
type LoginAttempt struct {
	Time      time.Time
	Success   bool
	ActionURL string
	Message   string
	Error     string
}
