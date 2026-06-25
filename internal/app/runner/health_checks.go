// Package runner 提供处理器运行时的健康检查实现。
package runner

import (
	"context"

	"task-processor/internal/app/task"
	"task-processor/internal/core/config"
	"task-processor/internal/core/errors"
)

// ConfigHealthCheck 验证关键配置项是否合法。
type ConfigHealthCheck struct {
	config *config.Config
}

func (c *ConfigHealthCheck) Name() string { return "config" }

func (c *ConfigHealthCheck) Check(_ context.Context) error {
	if c.config == nil {
		return errors.New(errors.ErrCodeConfig, "配置未加载")
	}
	if c.config.Worker.Concurrency <= 0 {
		return errors.New(errors.ErrCodeConfig, "工作池并发数配置无效")
	}
	if c.config.Management.BaseURL == "" {
		return errors.New(errors.ErrCodeConfig, "管理系统URL未配置")
	}
	return nil
}

// ProcessorRuntimeHealthCheck 验证处理器运行时是否已初始化。
type ProcessorRuntimeHealthCheck struct {
	runtime processorRuntimeProvider
}

func (p *ProcessorRuntimeHealthCheck) Name() string { return "processor_runtime" }

func (p *ProcessorRuntimeHealthCheck) Check(_ context.Context) error {
	if p.runtime == nil {
		return errors.New(errors.ErrCodeExternalAPI, "处理器运行时未初始化")
	}
	return nil
}

// ProcessorHealthCheck 验证平台处理器及其工作池的健康状态。
type ProcessorHealthCheck struct {
	name      string
	processor task.PlatformProcessor
}

func (p *ProcessorHealthCheck) Name() string { return "processor_" + p.name }

func (p *ProcessorHealthCheck) Check(_ context.Context) error {
	if p.processor == nil {
		return errors.Newf(errors.ErrCodeSystem, "%s处理器未初始化", p.name)
	}
	workerPool := p.processor.GetWorkerPool()
	if workerPool == nil {
		return errors.Newf(errors.ErrCodeSystem, "%s处理器工作池未初始化", p.name)
	}
	stats := workerPool.GetQueueStats()
	if stats.UsagePercent > 95 {
		return errors.Newf(errors.ErrCodeResourceLimit, "%s处理器队列使用率过高: %.1f%%", p.name, stats.UsagePercent)
	}
	return nil
}
