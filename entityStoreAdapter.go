package xfinitywifi

import (
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/common"
)

// entityStoreAdapter exposes the monitor's status snapshot to the HTTP handler.  The monitor guards its own
// state with a mutex, so the snapshot can be safely produced on the arbitrary goroutine that serves the HTTP
// request.
type entityStoreAdapter struct {
	parent *plugin
}

func (e entityStoreAdapter) GetStatusAndEntities() (common.StatusAndEntities, error) {
	return e.parent.monitor.Snapshot(), nil
}
