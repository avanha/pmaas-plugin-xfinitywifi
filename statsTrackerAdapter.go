package xfinitywifi

import (
	"github.com/avanha/pmaas-plugin-xfinitywifi/data"
	spi "github.com/avanha/pmaas-spi"
)

type statsTrackerAdapter struct {
	parent *plugin
}

func (s *statsTrackerAdapter) TrackProbeAttempt(probeAttempt *data.ProbeAttempt, connect bool, captivePortal bool) (bool, error) {
	return spi.ExecValueFunctionOnPluginGoRoutine(
		s.parent.container,
		func() bool { return s.parent.trackProbeAttempt(probeAttempt, connect, captivePortal) },
		func() bool { return true },
		"Unable to track probe attempt")
}
