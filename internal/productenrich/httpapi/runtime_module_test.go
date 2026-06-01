package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/infra/worker"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestRuntimeModuleRegistersRoutesAndWorkerPools(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewRuntimeModule(&Module{
		Handler: stubProductHandler{},
		Pool:    stubWorkerPool{},
	}, &productimagehttpapi.Module{
		Handler: stubImageHandler{},
		Pool:    stubWorkerPool{},
	})

	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"POST /api/v1/products/generate",
		"GET /api/v1/products/tasks/:task_id",
		"POST /api/v1/images/process",
		"GET /api/v1/images/tasks/:task_id",
		"POST /api/v1/images/tasks/:task_id/review",
	}, routeKeys(reg.Routes()))

	pools := reg.WorkerPools()
	require.Len(t, pools, 2)
	require.Equal(t, "product_enrich", pools[0].Name)
	require.Equal(t, "product_image", pools[1].Name)
}

type stubWorkerPool struct{}

func (stubWorkerPool) Start(context.Context)            {}
func (stubWorkerPool) Stop(context.Context)             {}
func (stubWorkerPool) Submit(worker.WorkerJob) error    { return nil }
func (stubWorkerPool) AvailableSlots() int              { return 0 }
func (stubWorkerPool) GetQueueStats() worker.QueueStats { return worker.QueueStats{} }
func (stubWorkerPool) SetJobHandler(worker.JobHandler)  {}
func (stubWorkerPool) GetMetrics() *worker.Metrics      { return nil }
