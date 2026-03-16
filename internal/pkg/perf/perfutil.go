// Package perf 提供性能跟踪与计时工具
package perf

import (
	"time"

	"github.com/sirupsen/logrus"
)

// Tracker 性能跟踪器
type Tracker struct {
	processName string
	logger      *logrus.Entry
	startTime   time.Time
	currentStep string
	stepStart   time.Time
}

// NewTracker 创建性能跟踪器
func NewTracker(processName string, logger *logrus.Entry) *Tracker {
	t := &Tracker{
		processName: processName,
		logger:      logger,
		startTime:   time.Now(),
	}
	if logger != nil {
		logger.WithField("process", processName).Info("🎯 开始流程")
	}
	return t
}

// StartStep 开始新步骤
func (t *Tracker) StartStep(stepName string) {
	if t.currentStep != "" {
		t.endCurrentStep()
	}
	t.currentStep = stepName
	t.stepStart = time.Now()
	if t.logger != nil {
		t.logger.WithFields(logrus.Fields{
			"process": t.processName,
			"step":    stepName,
		}).Info("📝 开始步骤")
	}
}

// EndStep 结束当前步骤
func (t *Tracker) EndStep() time.Duration {
	if t.currentStep == "" {
		return 0
	}
	d := t.endCurrentStep()
	t.currentStep = ""
	return d
}

func (t *Tracker) endCurrentStep() time.Duration {
	if t.currentStep == "" {
		return 0
	}
	d := time.Since(t.stepStart)
	if t.logger != nil {
		t.logger.WithFields(logrus.Fields{
			"process":     t.processName,
			"step":        t.currentStep,
			"duration":    d.String(),
			"duration_ms": d.Milliseconds(),
		}).Info("✅ 步骤完成")
	}
	return d
}

// Finish 完成整个流程
func (t *Tracker) Finish() time.Duration {
	if t.currentStep != "" {
		t.EndStep()
	}
	total := time.Since(t.startTime)
	if t.logger != nil {
		t.logger.WithFields(logrus.Fields{
			"process":           t.processName,
			"total_duration":    total.String(),
			"total_duration_ms": total.Milliseconds(),
		}).Info("🏁 流程完成")
	}
	return total
}

// TimeOperation 计时单个操作
func TimeOperation(name string, logger *logrus.Entry, fn func() error) error {
	start := time.Now()
	if logger != nil {
		logger.WithField("operation", name).Info("🚀 开始操作")
	}
	err := fn()
	d := time.Since(start)
	if logger != nil {
		fields := logrus.Fields{
			"operation":   name,
			"duration":    d.String(),
			"duration_ms": d.Milliseconds(),
			"success":     err == nil,
		}
		if err != nil {
			fields["error"] = err.Error()
		}
		lvl := logrus.InfoLevel
		if err != nil {
			lvl = logrus.ErrorLevel
		}
		logger.WithFields(fields).Log(lvl, "⏱️ 操作完成")
	}
	return err
}

// MeasureTime 简单耗时测量，返回结束函数
func MeasureTime(operation string, logger *logrus.Entry) func() {
	start := time.Now()
	if logger != nil {
		logger.WithField("operation", operation).Info("🚀 开始执行")
	}
	return func() {
		d := time.Since(start)
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"operation":   operation,
				"duration":    d.String(),
				"duration_ms": d.Milliseconds(),
			}).Info("⏱️ 执行完成")
		}
	}
}
