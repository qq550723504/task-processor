package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestZitadelSMSHandlerFlowsThroughExistingRuntimeInjection(t *testing.T) {
	calls := 0
	runtime := RuntimeDependencies{ZitadelSMSHandler: func(c *gin.Context) { calls++; c.Status(http.StatusNoContent) }}
	input := buildRuntimeServiceInput(nil, runtime)
	task := buildTaskModule(newTaskModuleInput(input, &builtRepositories{}))
	deliver := task.handlerDependencies.ZitadelSMSHandler
	require.NotNil(t, deliver)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	deliver(c)
	c.Writer.WriteHeaderNow()
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, 1, calls)
}
