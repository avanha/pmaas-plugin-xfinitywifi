package common

type StatsTracker interface {
	TrackProbeAttempt() (bool, error)
}
