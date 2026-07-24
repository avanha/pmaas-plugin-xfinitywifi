package xfinitywifi

import spi "github.com/avanha/pmaas-spi"

type statsTrackerAdapter struct {
	parent *plugin
}

func (s *statsTrackerAdapter) TrackProbeAttempt() (bool, error) {
	return spi.ExecValueFunctionOnPluginGoRoutine(
		s.parent.container,
		s.parent.trackProbeAttempt,
		func() bool { return true },
		"Unable to track probe attempt")
}
