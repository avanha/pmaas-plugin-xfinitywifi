package xfinitywifi

import (
	"context"
	"fmt"

	"github.com/avanha/pmaas-plugin-xfinitywifi/config"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/http"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/monitor"
	spi "github.com/avanha/pmaas-spi"
)

type plugin struct {
	pluginConfig config.PluginConfig
	container    spi.IPMAASContainer
	httpHandler  *http.Handler
	monitor      *monitor.Monitor
	cancelFn     context.CancelFunc
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
		monitor:      monitor.NewMonitor(pluginConfig),
	}
}

func (p *plugin) Init(container spi.IPMAASContainer) {
	p.container = container
	p.httpHandler.Init(container, &entityStoreAdapter{parent: p})
}

func (p *plugin) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelFn = cancel
	p.monitor.SetRunning(true)
	go p.monitor.Run(ctx)
}

func (p *plugin) Stop() chan func() {
	fmt.Printf("%T Stopping...\n", p)
	p.monitor.SetRunning(false)

	if p.cancelFn != nil {
		p.cancelFn()
	}

	return p.container.ClosedCallbackChannel()
}
