// Package consumer 提供各子服务的 lifecycle.Component 适配器
package consumer

import (
	"context"

	"task-processor/internal/core/lifecycle"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

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
	if err := c.svc.Start(ctx); err != nil {
		return err
	}
	c.SetRunning(true)
	return nil
}

func (c *reporterComponent) Stop(ctx context.Context) error {
	defer c.SetRunning(false)
	return c.svc.Stop(ctx)
}

// httpServerComponent 将 HTTPServerManager 适配为 lifecycle.Component。
type httpServerComponent struct {
	*lifecycle.BaseComponent
	svc *HTTPServerManager
}

// newHTTPServerComponent 创建 HTTP 服务器组件适配器（优先级 30，依赖 rabbitmq）。
func newHTTPServerComponent(svc *HTTPServerManager) lifecycle.Component {
	return &httpServerComponent{
		BaseComponent: lifecycle.NewBaseComponent("http-server", []string{"rabbitmq"}, 30),
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
