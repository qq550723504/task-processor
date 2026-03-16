// Package service 提供状态监控功能
package runner

import (
	"time"

	"github.com/sirupsen/logrus"
)

// startStatusMonitor 启动状态监控
func (s *processorServiceImpl) startStatusMonitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("状态监控已停止")
			return
		case <-ticker.C:
			s.logProcessorStatus()
		}
	}
}

// logProcessorStatus 记录处理器状态
func (s *processorServiceImpl) logProcessorStatus() {
	status := s.GetStatus()
	s.logger.WithFields(logrus.Fields{
		"status": status,
	}).Info("处理器状态监控")
}
