package xfinitywifi

import (
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/common"
	spi "github.com/avanha/pmaas-spi"
)

// entityStoreAdapter exposes the monitor's status snapshot to the HTTP handler.  The monitor guards its own
// state with a mutex, so the snapshot can be safely produced on the arbitrary goroutine that serves the HTTP
// request.
type entityStoreAdapter struct {
	parent *plugin
}

func (e entityStoreAdapter) GetStatusAndEntities() (common.StatusAndEntities, error) {
	// HTTP requests come in on arbitrary goroutines, so execute getStatusAndEntities on the main plugin goroutine to
	// get all states atomically.
	return spi.ExecValueFunctionOnPluginGoRoutine(
		e.parent.container,
		e.parent.getStatusAndEntities,
		func() common.StatusAndEntities { return common.StatusAndEntities{} },
		"unable to get status and entities")
}
