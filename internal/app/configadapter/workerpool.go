package configadapter

import (
	coreconfig "task-processor/internal/core/config"
	workerpool "task-processor/internal/platform/workerpool"
)

// WorkerPool translates the application worker schema while preserving legacy defaults.
func WorkerPool(cfg coreconfig.WorkerConfig) workerpool.PoolConfig {
	result := workerpool.DefaultPoolConfig()
	result.Concurrency = cfg.Concurrency
	result.BufferSize = cfg.BufferSize
	return result
}
