// Package validators 提供配置验证功能
package config

// ValidateWorkerConfig 验证工作池配置
func ValidateWorkerConfig(worker *WorkerConfig) []error {
	var errors []error

	if worker.Concurrency <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数必须大于 0",
		})
	}

	if worker.Concurrency > 100 {
		errors = append(errors, &ValidationError{
			Field:   "worker.concurrency",
			Message: "并发数不应超过 100",
		})
	}

	if worker.BufferSize <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.bufferSize",
			Message: "缓冲区大小必须大于 0",
		})
	}

	if worker.TaskInterval <= 0 {
		errors = append(errors, &ValidationError{
			Field:   "worker.taskInterval",
			Message: "任务获取间隔必须大于 0",
		})
	}

	return errors
}
