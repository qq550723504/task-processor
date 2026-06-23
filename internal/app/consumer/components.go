// Package consumer 提供各子服务的 lifecycle.Component 适配器
package consumer

import (
	"context"

	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

// SchedulerService 是 consumer 包对调度服务的最小依赖接口。
// 遵循"消费者定义接口"原则，避免直接依赖 app/runner 包。
type SchedulerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

// AutoShardService is the minimal interface for automatic store-shard coordination.
type AutoShardService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetStatus() map[string]any
}

// rabbitmqComponent 将 RabbitMQService 适配为 lifecycle.Component。
type rabbitmqComponent struct {
	*lifecycle.BaseComponent
	svc *RabbitMQService
}

// newRabbitMQComponent 创建 RabbitMQ 组件适配器（优先级 20，依赖 reporter）。
func newRabbitMQComponent(svc *RabbitMQService) lifecycle.Component {
	return &rabbitmqComponent{
		BaseComponent: lifecycle.NewBaseComponent("rabbitmq", []string{"reporter"}, 20),
		svc:           svc,
	}
}

func (c *rabbitmqComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *rabbitmqComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

// reporterComponent 将 ResultReporter 适配为 lifecycle.Component。
type reporterComponent struct {
	*lifecycle.BaseComponent
	svc *ResultReporter
}

// newReporterComponent 创建结果上报器组件适配器（优先级 10，无依赖）。
func newReporterComponent(svc *ResultReporter) lifecycle.Component {
	return &reporterComponent{
		BaseComponent: lifecycle.NewBaseComponent("reporter", nil, 10),
		svc:           svc,
	}
}

func (c *reporterComponent) Start(ctx context.Context) error {
	if c.svc == nil {
		c.SetRunning(true)
		return nil
	}
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *reporterComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	if c.svc == nil {
		return nil
	}
	return c.svc.Stop(ctx)
}

// httpServerComponent 将 HTTPServerManager 适配为 lifecycle.Component。
type httpServerComponent struct {
	*lifecycle.BaseComponent
	svc *HTTPServerManager
}

// newHTTPServerComponent 创建 HTTP 服务器组件适配器。
// 健康检查服务应尽早启动，这样依赖抖动时 Pod 仍能返回 liveness，
// readiness 则继续由 /ready 反映 RabbitMQ 是否已连通。
func newHTTPServerComponent(svc *HTTPServerManager) lifecycle.Component {
	return &httpServerComponent{
		BaseComponent: lifecycle.NewBaseComponent("http-server", []string{"load-monitor"}, 18),
		svc:           svc,
	}
}

func (c *httpServerComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *httpServerComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

// loadMonitorComponent 将 rabbitmq.LoadMonitor 适配为 lifecycle.Component。
type loadMonitorComponent struct {
	*lifecycle.BaseComponent
	svc    *rabbitmq.LoadMonitor
	logger *logrus.Logger
}

// newLoadMonitorComponent 创建负载监控组件适配器（优先级 15，依赖 reporter）。
func newLoadMonitorComponent(svc *rabbitmq.LoadMonitor, logger *logrus.Logger) lifecycle.Component {
	return &loadMonitorComponent{
		BaseComponent: lifecycle.NewBaseComponent("load-monitor", []string{"reporter"}, 15),
		svc:           svc,
		logger:        logger,
	}
}

func (c *loadMonitorComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *loadMonitorComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

type autoShardComponent struct {
	*lifecycle.BaseComponent
	svc AutoShardService
}

func newAutoShardComponent(svc AutoShardService) lifecycle.Component {
	return &autoShardComponent{
		BaseComponent: lifecycle.NewBaseComponent("auto-shard", []string{"reporter"}, 17),
		svc:           svc,
	}
}

func (c *autoShardComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *autoShardComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

// schedulerComponent 将 SchedulerService 适配为 lifecycle.Component。
type schedulerComponent struct {
	*lifecycle.BaseComponent
	svc SchedulerService
}

// newSchedulerComponent 创建调度器组件适配器（优先级 25，依赖 rabbitmq）。
func newSchedulerComponent(svc SchedulerService) lifecycle.Component {
	return &schedulerComponent{
		BaseComponent: lifecycle.NewBaseComponent("scheduler", []string{"rabbitmq"}, 25),
		svc:           svc,
	}
}

func (c *schedulerComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *schedulerComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

type processingTimeoutWatchdogComponent struct {
	*lifecycle.BaseComponent
	svc SchedulerService
}

func newProcessingTimeoutWatchdogComponent(svc SchedulerService) lifecycle.Component {
	return &processingTimeoutWatchdogComponent{
		BaseComponent: lifecycle.NewBaseComponent("processing-timeout-watchdog", []string{"reporter"}, 19),
		svc:           svc,
	}
}

func (c *processingTimeoutWatchdogComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *processingTimeoutWatchdogComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

type staleQueuedWatchdogComponent struct {
	*lifecycle.BaseComponent
	svc SchedulerService
}

func newStaleQueuedWatchdogComponent(svc SchedulerService) lifecycle.Component {
	return &staleQueuedWatchdogComponent{
		BaseComponent: lifecycle.NewBaseComponent("stale-queued-watchdog", []string{"reporter"}, 19),
		svc:           svc,
	}
}

func (c *staleQueuedWatchdogComponent) Start(ctx context.Context) error {
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *staleQueuedWatchdogComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}
