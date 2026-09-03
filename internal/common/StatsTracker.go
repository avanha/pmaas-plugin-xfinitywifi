package common

import "github.com/avanha/pmaas-plugin-xfinitywifi/data"

type StatsTracker interface {
	TrackProbeAttempt(probeAttempt *data.ProbeAttempt, connected bool, captivePortal bool) (bool, error)
}
