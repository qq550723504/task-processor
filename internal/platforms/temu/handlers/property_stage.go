// Package handlers 提供属性处理阶段的统一接口定义
package handlers

import (
	"fmt"
)

// PropertyStage 属性处理阶段接口
type PropertyStage interface {
	// Process 处理属性
	Process(ctx *PropertyContext) error

	// GetName 获取阶段名称
	GetName() string

	// GetOrder 获取执行顺序（数字越小越先执行）
	GetOrder() int

	// IsEnabled 是否启用该阶段
	IsEnabled(ctx *PropertyContext) bool
}

// BasePropertyStage 基础属性处理阶段
type BasePropertyStage struct {
	name    string
	order   int
	enabled bool
}

// NewBasePropertyStage 创建基础属性处理阶段
func NewBasePropertyStage(name string, order int) *BasePropertyStage {
	return &BasePropertyStage{
		name:    name,
		order:   order,
		enabled: true,
	}
}

// GetName 获取阶段名称
func (s *BasePropertyStage) GetName() string {
	return s.name
}

// GetOrder 获取执行顺序
func (s *BasePropertyStage) GetOrder() int {
	return s.order
}

// IsEnabled 是否启用该阶段
func (s *BasePropertyStage) IsEnabled(ctx *PropertyContext) bool {
	return s.enabled
}

// SetEnabled 设置是否启用
func (s *BasePropertyStage) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// StageError 阶段处理错误
type StageError struct {
	StageName string
	Message   string
	Cause     error
}

// Error 实现error接口
func (e *StageError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("阶段 [%s] 处理失败: %s, 原因: %v", e.StageName, e.Message, e.Cause)
	}
	return fmt.Sprintf("阶段 [%s] 处理失败: %s", e.StageName, e.Message)
}

// Unwrap 支持错误链
func (e *StageError) Unwrap() error {
	return e.Cause
}

// NewStageError 创建阶段错误
func NewStageError(stageName, message string, cause error) *StageError {
	return &StageError{
		StageName: stageName,
		Message:   message,
		Cause:     cause,
	}
}

// StageResult 阶段处理结果
type StageResult struct {
	StageName      string
	Success        bool
	ProcessedCount int
	FixedCount     int
	SkippedCount   int
	ErrorCount     int
	Message        string
	Error          error
}

// NewStageResult 创建阶段结果
func NewStageResult(stageName string) *StageResult {
	return &StageResult{
		StageName: stageName,
		Success:   true,
	}
}

// SetError 设置错误
func (r *StageResult) SetError(err error) {
	r.Success = false
	r.Error = err
	r.ErrorCount++
}

// AddProcessed 增加处理数量
func (r *StageResult) AddProcessed(count int) {
	r.ProcessedCount += count
}

// AddFixed 增加修复数量
func (r *StageResult) AddFixed(count int) {
	r.FixedCount += count
}

// AddSkipped 增加跳过数量
func (r *StageResult) AddSkipped(count int) {
	r.SkippedCount += count
}

// PropertyStageChain 属性处理阶段链
type PropertyStageChain struct {
	stages []PropertyStage
}

// NewPropertyStageChain 创建属性处理阶段链
func NewPropertyStageChain() *PropertyStageChain {
	return &PropertyStageChain{
		stages: make([]PropertyStage, 0),
	}
}

// AddStage 添加处理阶段
func (c *PropertyStageChain) AddStage(stage PropertyStage) {
	c.stages = append(c.stages, stage)
}

// GetStages 获取所有阶段
func (c *PropertyStageChain) GetStages() []PropertyStage {
	return c.stages
}

// Execute 执行所有阶段
func (c *PropertyStageChain) Execute(ctx *PropertyContext) error {
	for _, stage := range c.stages {
		// 检查阶段是否启用
		if !stage.IsEnabled(ctx) {
			ctx.Logger.Debugf("跳过禁用的阶段: %s", stage.GetName())
			continue
		}

		// 记录阶段开始
		ctx.Statistics.RecordStageStart(stage.GetName())
		ctx.Logger.Infof("🔄 开始执行阶段: %s", stage.GetName())

		// 执行阶段
		if err := stage.Process(ctx); err != nil {
			ctx.Statistics.RecordStageEnd(stage.GetName(), 0, 0, 1)
			return NewStageError(stage.GetName(), "阶段执行失败", err)
		}

		// 记录阶段结束
		ctx.Statistics.RecordStageEnd(stage.GetName(), 1, 1, 0)
		ctx.Logger.Infof("✅ 阶段执行完成: %s", stage.GetName())
	}

	return nil
}
