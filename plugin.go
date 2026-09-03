package xfinitywifi

import (
	"context"
	"fmt"
	"sync"

	"github.com/avanha/pmaas-plugin-xfinitywifi/config"
	"github.com/avanha/pmaas-plugin-xfinitywifi/data"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/common"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/http"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/worker"
	spi "github.com/avanha/pmaas-spi"
)

type plugin struct {
	pluginConfig config.PluginConfig
	container    spi.IPMAASContainer
	httpHandler  *http.Handler
	monitor      *worker.Worker
	cancelFn     context.CancelFunc
	worker       *worker.Worker
	workersWg    sync.WaitGroup
	running      bool
	status       data.PluginStatus
}

func NewPluginConfig() config.PluginConfig {
	return config.NewPluginConfig()
}

type Plugin interface {
	spi.IPMAASPlugin
}

func NewPlugin(pluginConfig config.PluginConfig) Plugin {
	return &plugin{
		pluginConfig: pluginConfig,
		httpHandler:  http.NewHandler(),
	}
}

func (p *plugin) Init(container spi.IPMAASContainer) {
	p.container = container
	p.processConfig()
	p.httpHandler.Init(container, &entityStoreAdapter{parent: p})
	p.worker = worker.NewWorker(p.pluginConfig, &statsTrackerAdapter{parent: p})
}

func (p *plugin) Start() {
	p.registerEntities()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelFn = cancel
	p.workersWg.Go(func() { p.worker.Run(ctx) })
	p.running = true
}

func (p *plugin) Stop() chan func() {
	fmt.Printf("%T Stopping...\n", p)
	p.running = false
	p.cancelFn()
	callbackCh := make(chan func())
	go func() {
		fmt.Printf("%T Waiting for worker(s) to finish...\n", p)
		p.workersWg.Wait()
		callbackCh <- func() { p.onWorkersStopped(callbackCh) }
	}()

	return callbackCh
}

func (p *plugin) processConfig() {

}

func (p *plugin) onWorkersStopped(callbackCh chan func()) {
	fmt.Printf("%T Workers stopped, deregistering entities...\n", p)
	p.deregisterEntities()
	close(callbackCh)
}

func (p *plugin) registerEntities() {

}

func (p *plugin) deregisterEntities() {

}

func (p *plugin) trackProbeAttempt(probeAttempt *data.ProbeAttempt, connected bool, captivePortal bool) bool {
	return true
}

func (p *plugin) getStatusAndEntities() common.StatusAndEntities {
	return common.StatusAndEntities{Status: p.status}
}
