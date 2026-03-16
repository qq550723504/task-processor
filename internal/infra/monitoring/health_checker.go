// Package monitoring 提供健康检查功能
package monitoring

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/core/errors"
	"task-processor/internal/core/lifecycle"
	"task-processor/internal/pkg/contextutil"

	"github.com/sirupsen/logrus"
)

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// HealthChecker 健康检查器
type HealthChecker struct {
	*lifecycle.BaseComponent
	logger   *logrus.Logger
	checks   map[string]HealthCheck
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(logger *logrus.Logger, interval time.Duration) *HealthChecker {
	if interval <= 0 {
		interval = 60 * time.Second
	}

	return &HealthChecker{
		BaseComponent: lifecycle.NewBaseComponent("HealthChecker", []string{}, 100),
		logger:        logger,
		checks:        make(map[string]HealthCheck),
		interval:      interval,
	}
}

// Start 启动健康检查器
func (h *HealthChecker) Start(ctx context.Context) error {
	if h.IsRunning() {
		return errors.New(errors.ErrCodeSystem, "HealthChecker已在运行")
	}

	h.logger.Info("启动健康检查器...")

	h.ctx, h.cancel = context.WithCancel(ctx)

	// 启动健康检查循环
	go h.checkLoop()

	h.SetRunning(true)
	h.logger.Info("健康检查器启动完成")
	return nil
}

// Stop 停止健康检查器
func (h *HealthChecker) Stop(ctx context.Context) error {
	if !h.IsRunning() {
		return nil
	}

	h.logger.Info("停止健康检查器...")

	if h.cancel != nil {
		h.cancel()
	}

	h.SetRunning(false)
	h.logger.Info("健康检查器停止完成")
	return nil
}

// RegisterCheck 注册健康检查
func (h *HealthChecker) RegisterCheck(check HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.checks[check.Name()] = check
	h.logger.Infof("注册健康检查: %s", check.Name())
}

// checkLoop 健康检查循环
func (h *HealthChecker) checkLoop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info("健康检查循环停止")
			return
		case <-ticker.C:
			h.runHealthChecks()
		}
	}
}

// runHealthChecks 运行健康检查
func (h *HealthChecker) runHealthChecks() {
	h.mu.RLock()
	checks := make(map[string]HealthCheck)
	for k, v := range h.checks {
		checks[k] = v
	}
	h.mu.RUnlock()

	if len(checks) == 0 {
		return
	}

	h.logger.Debug("开始健康检查...")

	for _, check := range checks {
		status := h.runSingleCheck(check)
		h.logHealthStatus(status)
	}
}

// runSingleCheck 运行单个健康检查
func (h *HealthChecker) runSingleCheck(check HealthCheck) *HealthStatus {
	ctx, cancel := contextutil.WithHealthTimeout(h.ctx)
	defer cancel()

	status := &HealthStatus{
		Name:      check.Name(),
		Timestamp: time.Now(),
	}

	if err := check.Check(ctx); err != nil {
		status.Status = "unhealthy"
		status.Error = err.Error()
	} else {
		status.Status = "healthy"
	}

	return status
}

// logHealthStatus 记录健康状态
func (h *HealthChecker) logHealthStatus(status *HealthStatus) {
	fields := logrus.Fields{
		"check":     status.Name,
		"status":    status.Status,
		"timestamp": status.Timestamp,
	}

	if status.Error != "" {
		fields["error"] = status.Error
		h.logger.WithFields(fields).Warn("健康检查失败")
	} else {
		h.logger.WithFields(fields).Debug("健康检查通过")
	}
}
