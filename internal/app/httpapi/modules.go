package httpapi

import (
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	sdsbootstrap "task-processor/internal/sds/httpbootstrap"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	sheinclient "task-processor/internal/shein/client"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"
)

var newSDSSyncServiceForHTTPAPI = sdsbootstrap.NewSyncService

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	if err != nil {
		return nil, err
	}
	sheinclient.ConfigureLoginAccountFromConfig(deps.shared.cfg)

	composition, err := newHTTPFeatureCompositionBuilder().build(logger, deps)
	if err != nil {
		return nil, err
	}

	runtimeBundle, err := composition.buildRuntimeBundle(deps.shared.cfg)
	if err != nil {
		return nil, err
	}

	server, routes := runtimeBundle.buildServerBundle(options.Port)
	return &appBootstrap{
		productHandler: composition.productHandler(),
		imageHandler:   composition.imageHandler(),
		server:         server,
		routes:         routes,
		pools:          runtimeBundle.pools(),
		closers:        deps.shared.closers,
	}, nil
}

func buildSheinLoginModuleResult(deps *runtimeDeps) (*sheinloginbootstrap.BuildResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}

	result, err := sheinloginbootstrap.BuildHandler(sheinloginbootstrap.BuildInput{
		Config:                   deps.shared.cfg,
		ManagementClient:         deps.managementClient(),
		AccountRepositoryBuilder: listingkithttpapi.BuildListingAdminStoreRepository,
	})
	if err != nil {
		return nil, nil, err
	}
	if result == nil {
		return nil, nil, nil
	}
	return result, result.Close, nil
}

func buildSDSLoginModuleResult(deps *runtimeDeps) (*sdsloginbootstrap.BuildResult, func() error, error) {
	if deps == nil {
		return nil, nil, nil
	}
	result, err := sdsloginbootstrap.BuildHandler(deps.shared.cfg)
	if err != nil {
		return nil, nil, err
	}
	if result == nil || result.StatusProvider == nil {
		return nil, nil, nil
	}
	return result, nil, nil
}

func BuildHandlers(logger *logrus.Logger, options Options) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}

func newWorkerPool(processor worker.Processor, cfg *config.Config) worker.WorkerPool {
	return worker.NewPoolWithConfig(processor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
}

func buildLocalTaskHealthProvider(pools map[string]worker.WorkerPool) taskrpcapi.LocalStatusProvider {
	return func() map[string]any {
		summary := map[string]any{
			"poolCount":           0,
			"totalQueueSize":      0,
			"totalBufferSize":     0,
			"totalAvailableSlots": 0,
			"totalSubmitted":      int64(0),
			"totalProcessed":      int64(0),
			"totalSucceeded":      int64(0),
			"totalFailed":         int64(0),
			"totalPanicked":       int64(0),
			"queueFullCount":      int64(0),
		}
		poolSnapshots := make(map[string]any, len(pools))

		for name, pool := range pools {
			if pool == nil {
				continue
			}

			queueStats := pool.GetQueueStats()
			poolSnapshot := map[string]any{
				"queueSize":      queueStats.QueueSize,
				"bufferSize":     queueStats.BufferSize,
				"availableSlots": queueStats.AvailableSlots,
				"usagePercent":   queueStats.UsagePercent,
			}

			summary["poolCount"] = summary["poolCount"].(int) + 1
			summary["totalQueueSize"] = summary["totalQueueSize"].(int) + queueStats.QueueSize
			summary["totalBufferSize"] = summary["totalBufferSize"].(int) + queueStats.BufferSize
			summary["totalAvailableSlots"] = summary["totalAvailableSlots"].(int) + queueStats.AvailableSlots

			if metrics := pool.GetMetrics(); metrics != nil {
				snapshot := metrics.GetSnapshot()
				poolSnapshot["metrics"] = map[string]any{
					"totalSubmitted": snapshot.TotalSubmitted,
					"totalProcessed": snapshot.TotalProcessed,
					"totalSucceeded": snapshot.TotalSucceeded,
					"totalFailed":    snapshot.TotalFailed,
					"totalPanicked":  snapshot.TotalPanicked,
					"queueFullCount": snapshot.QueueFullCount,
					"successRate":    snapshot.SuccessRate(),
					"failureRate":    snapshot.FailureRate(),
					"panicRate":      snapshot.PanicRate(),
					"uptimeSeconds":  int64(snapshot.Uptime.Seconds()),
				}
				summary["totalSubmitted"] = summary["totalSubmitted"].(int64) + snapshot.TotalSubmitted
				summary["totalProcessed"] = summary["totalProcessed"].(int64) + snapshot.TotalProcessed
				summary["totalSucceeded"] = summary["totalSucceeded"].(int64) + snapshot.TotalSucceeded
				summary["totalFailed"] = summary["totalFailed"].(int64) + snapshot.TotalFailed
				summary["totalPanicked"] = summary["totalPanicked"].(int64) + snapshot.TotalPanicked
				summary["queueFullCount"] = summary["queueFullCount"].(int64) + snapshot.QueueFullCount
			}

			poolSnapshots[name] = poolSnapshot
		}

		return map[string]any{
			"summary": summary,
			"pools":   poolSnapshots,
		}
	}
}
