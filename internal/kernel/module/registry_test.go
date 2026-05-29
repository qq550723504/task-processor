package module

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
)

func TestRegistryAddRoutesPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	handler := func(c *gin.Context) {}

	reg.AddRoutes(
		httproute.Descriptor{Method: http.MethodGet, Path: "/health", Module: "system", Handler: handler},
		httproute.Descriptor{Method: http.MethodPost, Path: "/jobs", Module: "jobs", Handler: handler},
	)

	routes := reg.Routes()
	require.Len(t, routes, 2)
	require.Equal(t, "/health", routes[0].Path)
	require.Equal(t, "/jobs", routes[1].Path)
}

func TestRegistryRoutesReturnsDefensiveCopy(t *testing.T) {
	reg := NewRegistry()
	handler := func(c *gin.Context) {}

	reg.AddRoutes(
		httproute.Descriptor{Method: http.MethodGet, Path: "/health", Module: "system", Handler: handler},
	)

	routes := reg.Routes()
	routes[0].Path = "/mutated"

	again := reg.Routes()
	require.Equal(t, "/health", again[0].Path)
}

func TestRegistryRegisterTaskHandlerRejectsDuplicateTaskType(t *testing.T) {
	reg := NewRegistry()

	err := reg.RegisterTaskHandler(stubTaskHandler{name: "product_enrich"})
	require.NoError(t, err)

	err = reg.RegisterTaskHandler(stubTaskHandler{name: "product_enrich"})
	require.ErrorContains(t, err, "task handler already registered")
}

func TestRegistryRegisterTaskHandlerRejectsNilHandler(t *testing.T) {
	reg := NewRegistry()

	var handler TaskHandler
	err := reg.RegisterTaskHandler(handler)
	require.ErrorContains(t, err, "task handler is nil")
}

func TestRegistryAddWorkerPoolPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	first := stubWorkerPool{}
	second := stubWorkerPool{}

	err := reg.AddWorkerPool("product_enrich", first)
	require.NoError(t, err)
	err = reg.AddWorkerPool("product_image", second)
	require.NoError(t, err)

	pools := reg.WorkerPools()
	require.Len(t, pools, 2)
	require.Equal(t, "product_enrich", pools[0].Name)
	require.Equal(t, first, pools[0].Pool)
	require.Equal(t, "product_image", pools[1].Name)
	require.Equal(t, second, pools[1].Pool)
}

func TestRegistryAddWorkerPoolRejectsDuplicateNames(t *testing.T) {
	reg := NewRegistry()

	err := reg.AddWorkerPool("product_enrich", stubWorkerPool{})
	require.NoError(t, err)

	err = reg.AddWorkerPool("product_enrich", stubWorkerPool{})
	require.ErrorContains(t, err, "worker pool already registered")
}

func TestRegistryAddWorkerPoolRejectsNilPool(t *testing.T) {
	reg := NewRegistry()

	var pool worker.WorkerPool
	err := reg.AddWorkerPool("product_enrich", pool)
	require.ErrorContains(t, err, "worker pool is nil")
}

func TestRegistryRegisterWorkflowHandlerRejectsDuplicateWorkflow(t *testing.T) {
	reg := NewRegistry()

	err := reg.RegisterWorkflowHandler(stubWorkflowHandler{name: "publish"})
	require.NoError(t, err)

	err = reg.RegisterWorkflowHandler(stubWorkflowHandler{name: "publish"})
	require.ErrorContains(t, err, "workflow handler already registered")
}

func TestRegistryRegisterWorkflowHandlerRejectsNilHandler(t *testing.T) {
	reg := NewRegistry()

	var handler WorkflowHandler
	err := reg.RegisterWorkflowHandler(handler)
	require.ErrorContains(t, err, "workflow handler is nil")
}

func TestRegistryRegisterWorkflowHandlerRejectsMismatchedWorkflowName(t *testing.T) {
	reg := NewRegistry()

	err := reg.RegisterWorkflowHandler(stubWorkflowHandler{
		name:         "publish",
		registerName: "adapt",
	})
	require.ErrorContains(t, err, "workflow handler registered unexpected name")
}

func TestRegistryRegisterWorkflowHandlerRejectsMissingRegistration(t *testing.T) {
	reg := NewRegistry()

	err := reg.RegisterWorkflowHandler(stubWorkflowHandler{
		name:             "publish",
		skipRegistration: true,
	})
	require.ErrorContains(t, err, "workflow handler did not register workflow")
}

type stubTaskHandler struct {
	name string
}

func (h stubTaskHandler) TaskType() string {
	return h.name
}

func (h stubTaskHandler) Validate(context.Context, any) error {
	return nil
}

func (h stubTaskHandler) Execute(context.Context, any) (any, error) {
	return nil, nil
}

type stubWorkflowHandler struct {
	name             string
	registerName     string
	skipRegistration bool
}

func (h stubWorkflowHandler) WorkflowName() string {
	return h.name
}

func (h stubWorkflowHandler) RegisterWorkflow(reg *WorkflowRegistry) error {
	if h.skipRegistration {
		return nil
	}
	if h.registerName != "" {
		return reg.Register(h.registerName)
	}
	return reg.Register(h.name)
}

type stubWorkerPool struct{}

func (stubWorkerPool) Start(context.Context)            {}
func (stubWorkerPool) Stop(context.Context)             {}
func (stubWorkerPool) Submit(worker.WorkerJob) error    { return nil }
func (stubWorkerPool) AvailableSlots() int              { return 0 }
func (stubWorkerPool) GetQueueStats() worker.QueueStats { return worker.QueueStats{} }
func (stubWorkerPool) SetJobHandler(worker.JobHandler)  {}
func (stubWorkerPool) GetMetrics() *worker.Metrics      { return nil }
