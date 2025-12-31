// Package utils 提供简单的性能监控工具
package utils

import (
	"time"

	"github.com/sirupsen/logrus"
)

// PerformanceTracker 性能跟踪器
type PerformanceTracker struct {
	processName string
	logger      *logrus.Entry
	startTime   time.Time
	currentStep string
	stepStart   time.Time
}

// NewPerformanceTracker 创建性能跟踪器
func NewPerformanceTracker(processName string, logger *logrus.Entry) *PerformanceTracker {
	tracker := &PerformanceTracker{
		processName: processName,
		logger:      logger,
		startTime:   time.Now(),
	}

	if logger != nil {
		logger.WithField("process", processName).Info("🎯 开始流程")
	}

	return tracker
}

// StartStep 开始新步骤
func (pt *PerformanceTracker) StartStep(stepName string) {
	// 结束当前步骤
	if pt.currentStep != "" {
		pt.endCurrentStep()
	}

	// 开始新步骤
	pt.currentStep = stepName
	pt.stepStart = time.Now()

	if pt.logger != nil {
		pt.logger.WithFields(logrus.Fields{
			"process": pt.processName,
			"step":    stepName,
		}).Info("📝 开始步骤")
	}
}

// EndStep 结束当前步骤
func (pt *PerformanceTracker) EndStep() time.Duration {
	if pt.currentStep == "" {
		return 0
	}

	duration := pt.endCurrentStep()
	pt.currentStep = ""
	return duration
}

// endCurrentStep 内部方法：结束当前步骤
func (pt *PerformanceTracker) endCurrentStep() time.Duration {
	if pt.currentStep == "" {
		return 0
	}

	duration := time.Since(pt.stepStart)

	if pt.logger != nil {
		pt.logger.WithFields(logrus.Fields{
			"process":     pt.processName,
			"step":        pt.currentStep,
			"duration":    duration.String(),
			"duration_ms": duration.Milliseconds(),
		}).Info("✅ 步骤完成")
	}

	return duration
}

// Finish 完成整个流程
func (pt *PerformanceTracker) Finish() time.Duration {
	// 结束当前步骤
	if pt.currentStep != "" {
		pt.EndStep()
	}

	totalDuration := time.Since(pt.startTime)

	if pt.logger != nil {
		pt.logger.WithFields(logrus.Fields{
			"process":           pt.processName,
			"total_duration":    totalDuration.String(),
			"total_duration_ms": totalDuration.Milliseconds(),
		}).Info("🏁 流程完成")
	}

	return totalDuration
}

// TimeOperation 计时单个操作
func TimeOperation(operationName string, logger *logrus.Entry, fn func() error) error {
	start := time.Now()

	if logger != nil {
		logger.WithField("operation", operationName).Info("🚀 开始操作")
	}

	err := fn()
	duration := time.Since(start)

	if logger != nil {
		fields := logrus.Fields{
			"operation":   operationName,
			"duration":    duration.String(),
			"duration_ms": duration.Milliseconds(),
			"success":     err == nil,
		}

		if err != nil {
			fields["error"] = err.Error()
		}

		level := logrus.InfoLevel
		if err != nil {
			level = logrus.ErrorLevel
		}

		logger.WithFields(fields).Log(level, "⏱️ 操作完成")
	}

	return err
}

// TimeOperationWithResult 计时带返回值的操作
func TimeOperationWithResult(operationName string, logger *logrus.Entry, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()

	if logger != nil {
		logger.WithField("operation", operationName).Info("🚀 开始操作")
	}

	result, err := fn()
	duration := time.Since(start)

	if logger != nil {
		fields := logrus.Fields{
			"operation":   operationName,
			"duration":    duration.String(),
			"duration_ms": duration.Milliseconds(),
			"success":     err == nil,
		}

		if err != nil {
			fields["error"] = err.Error()
		}

		level := logrus.InfoLevel
		if err != nil {
			level = logrus.ErrorLevel
		}

		logger.WithFields(fields).Log(level, "⏱️ 操作完成")
	}

	return result, err
}

// MeasureTime 简单的耗时测量装饰器
func MeasureTime(operation string, logger *logrus.Entry) func() {
	start := time.Now()

	if logger != nil {
		logger.WithField("operation", operation).Info("🚀 开始执行")
	}

	return func() {
		duration := time.Since(start)
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"operation":   operation,
				"duration":    duration.String(),
				"duration_ms": duration.Milliseconds(),
			}).Info("⏱️ 执行完成")
		}
	}
}
