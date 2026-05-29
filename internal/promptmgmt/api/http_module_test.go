package api

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestNewHTTPModuleRegistersPromptRoutes(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewHTTPModule(stubHTTPRouteHandler{})

	require.Equal(t, "listing-kit-prompts", module.Name())
	require.True(t, module.Enabled(nil))
	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"GET /api/v1/listing-kits/prompts/catalog",
		"GET /api/v1/listing-kits/prompts/schema/:key",
		"GET /api/v1/listing-kits/prompts",
		"PUT /api/v1/listing-kits/prompts",
		"PATCH /api/v1/listing-kits/prompts/:key/status",
	}, routeKeys(reg.Routes()))
}

func TestBuildModuleReturnsHandlerAndModule(t *testing.T) {
	t.Parallel()

	result := BuildModule(nil)
	require.NotNil(t, result)
	require.NotNil(t, result.Handler)
	require.NotNil(t, result.Module)
}

type stubHTTPRouteHandler struct{}

func (stubHTTPRouteHandler) ListPromptTemplateCatalog(*gin.Context) {}
func (stubHTTPRouteHandler) GetPromptTemplateSchema(*gin.Context)   {}
func (stubHTTPRouteHandler) ListPromptTemplates(*gin.Context)       {}
func (stubHTTPRouteHandler) UpsertPromptTemplate(*gin.Context)      {}
func (stubHTTPRouteHandler) SetPromptTemplateStatus(*gin.Context)   {}

func routeKeys(routes []httproute.Descriptor) []string {
	keys := make([]string, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", route.Method, route.Path))
	}
	return keys
}
