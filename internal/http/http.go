package http

import (
	"embed"
	"fmt"
	"net/http"
	"reflect"

	"github.com/avanha/pmaas-plugin-xfinitywifi/data"
	"github.com/avanha/pmaas-plugin-xfinitywifi/internal/common"
	spi "github.com/avanha/pmaas-spi"
)

//go:embed content/static content/templates
var contentFS embed.FS

var statusTemplate = spi.TemplateInfo{
	Name:   "xfinitywifi_status",
	Paths:  []string{"templates/xfinitywifi_status.htmlt"},
	Styles: []string{"css/xfinitywifi_status.css"},
}

var probeTemplate = spi.TemplateInfo{
	Name:   "xfinitywifi_probe",
	Paths:  []string{"templates/xfinitywifi_probe.htmlt"},
	Styles: []string{"css/xfinitywifi_card.css"},
}

var loginTemplate = spi.TemplateInfo{
	Name:   "xfinitywifi_login",
	Paths:  []string{"templates/xfinitywifi_login.htmlt"},
	Styles: []string{"css/xfinitywifi_card.css"},
}

type Handler struct {
	container   spi.IPMAASContainer
	entityStore common.EntityStore
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Init(container spi.IPMAASContainer, entityStore common.EntityStore) {
	h.container = container
	h.entityStore = entityStore
	container.ProvideContentFS(&contentFS, "content")
	container.EnableStaticContent("static")
	container.AddRoute("/plugins/xfinitywifi/", h.handleHttpListRequest)
	container.RegisterEntityRenderer(
		reflect.TypeOf((*data.PluginStatus)(nil)).Elem(),
		h.statusDataRendererFactory)
	container.RegisterEntityRenderer(
		reflect.TypeOf((*data.ProbeAttempt)(nil)).Elem(),
		h.probeDataRendererFactory)
	container.RegisterEntityRenderer(
		reflect.TypeOf((*data.LoginAttempt)(nil)).Elem(),
		h.loginDataRendererFactory)
}

func (h *Handler) handleHttpListRequest(writer http.ResponseWriter, request *http.Request) {
	result, err := h.entityStore.GetStatusAndEntities()

	if err != nil {
		fmt.Printf("xfinitywifi.http handleHttpListRequest: Error retrieving status: %s\n", err)
		result = common.StatusAndEntities{}
	}

	entityPointers := make([]any, 0, 2)

	if result.LastProbe != nil {
		entityPointers = append(entityPointers, result.LastProbe)
	}

	if result.LastLogin != nil {
		entityPointers = append(entityPointers, result.LastLogin)
	}

	h.container.RenderList(
		writer,
		request,
		spi.RenderListOptions{
			Title:  "Xfinity WiFi",
			Header: &result.Status,
		},
		entityPointers)
}

func (h *Handler) statusDataRendererFactory() (spi.EntityRenderer, error) {
	return spi.TemplateBasedRendererFactory(
		h.container,
		&statusTemplate,
		func(entity any) bool { _, ok := entity.(*data.PluginStatus); return ok },
		"*PluginStatus")
}

func (h *Handler) probeDataRendererFactory() (spi.EntityRenderer, error) {
	return spi.TemplateBasedRendererFactory(
		h.container,
		&probeTemplate,
		func(entity any) bool { _, ok := entity.(*data.ProbeAttempt); return ok },
		"*ProbeAttempt")
}

func (h *Handler) loginDataRendererFactory() (spi.EntityRenderer, error) {
	return spi.TemplateBasedRendererFactory(
		h.container,
		&loginTemplate,
		func(entity any) bool { _, ok := entity.(*data.LoginAttempt); return ok },
		"*LoginAttempt")
}
