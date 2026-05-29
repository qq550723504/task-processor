package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
	"task-processor/internal/prompt"
	"task-processor/internal/promptmgmt"
)

type HTTPRouteHandler interface {
	ListPromptTemplateCatalog(c *gin.Context)
	GetPromptTemplateSchema(c *gin.Context)
	ListPromptTemplates(c *gin.Context)
	UpsertPromptTemplate(c *gin.Context)
	SetPromptTemplateStatus(c *gin.Context)
}

type BuildResult struct {
	Handler HTTPRouteHandler
	Module  module.Module
}

const httpModuleName = "listing-kit-prompts"

type httpModule struct {
	register func(reg *module.Registry) error
}

func BuildHandler(store prompt.TenantPromptStore) *Handler {
	return NewHandler(promptmgmt.NewService(store))
}

func BuildModule(store prompt.TenantPromptStore) *BuildResult {
	handler := BuildHandler(store)
	return &BuildResult{
		Handler: handler,
		Module:  NewHTTPModule(handler),
	}
}

func NewHTTPModule(handler HTTPRouteHandler) module.Module {
	return httpModule{
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendRouteDescriptors(nil, handler)...)
			return nil
		},
	}
}

func (m httpModule) Name() string {
	return httpModuleName
}

func (httpModule) Enabled(*config.Config) bool {
	return true
}

func (m httpModule) Register(reg *module.Registry) error {
	if m.register != nil {
		return m.register(reg)
	}
	return nil
}

func AppendRouteDescriptors(routes []httproute.Descriptor, handler HTTPRouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts/catalog", Module: "listing-kit-prompts", Handler: handler.ListPromptTemplateCatalog},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts/schema/:key", Module: "listing-kit-prompts", Handler: handler.GetPromptTemplateSchema},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/listing-kits/prompts", Module: "listing-kit-prompts", Handler: handler.ListPromptTemplates},
		httproute.Descriptor{Method: http.MethodPut, Path: "/api/v1/listing-kits/prompts", Module: "listing-kit-prompts", Handler: handler.UpsertPromptTemplate},
		httproute.Descriptor{Method: http.MethodPatch, Path: "/api/v1/listing-kits/prompts/:key/status", Module: "listing-kit-prompts", Handler: handler.SetPromptTemplateStatus},
	)
}
