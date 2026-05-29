package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestRuntimeModuleRegistersRoutesAndWorkerPool(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	module := NewRuntimeModule(&Module{
		Handler: stubHandler{},
		Pool:    stubWorkerPool{},
	})

	require.NoError(t, module.Register(reg))
	require.Equal(t, []string{
		"POST /api/v1/amazon/listings/generate",
		"GET /api/v1/amazon/listings/tasks",
		"GET /api/v1/amazon/listings/tasks/:task_id",
		"GET /api/v1/amazon/listings/tasks/:task_id/workbench",
		"POST /api/v1/amazon/listings/tasks/:task_id/review",
		"POST /api/v1/amazon/listings/tasks/:task_id/submit",
	}, routeKeys(reg.Routes()))

	pools := reg.WorkerPools()
	require.Len(t, pools, 1)
	require.Equal(t, "amazon_listing", pools[0].Name)
}

type stubWorkerPool struct{}

func (stubWorkerPool) Start(context.Context)            {}
func (stubWorkerPool) Stop(context.Context)             {}
func (stubWorkerPool) Submit(worker.WorkerJob) error    { return nil }
func (stubWorkerPool) AvailableSlots() int              { return 0 }
func (stubWorkerPool) GetQueueStats() worker.QueueStats { return worker.QueueStats{} }
func (stubWorkerPool) SetJobHandler(worker.JobHandler)  {}
func (stubWorkerPool) GetMetrics() *worker.Metrics      { return nil }
