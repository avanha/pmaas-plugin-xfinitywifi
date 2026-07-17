package common

import "github.com/avanha/pmaas-plugin-xfinitywifi/data"

// StatusAndEntities aggregates the plugin status together with the most recent probe and login attempts.
type StatusAndEntities struct {
	Status    data.PluginStatus
	LastProbe *data.ProbeAttempt
	LastLogin *data.LoginAttempt
}

type EntityStore interface {
	GetStatusAndEntities() (StatusAndEntities, error)
}
