// Package shein 提供 SHEIN 平台类型（通过 shein/context 包定义）
package shein

import (
"task-processor/internal/model"
sheinctx "task-processor/internal/shein/context"
)

// Task 使用公共的Task类型
type Task = model.Task

// TaskContext 任务处理上下文
type TaskContext = sheinctx.TaskContext

// StepHandler 任务处理步骤接口
type StepHandler = sheinctx.StepHandler

// VariantFilterInfo 变体过滤信息
type VariantFilterInfo = sheinctx.VariantFilterInfo

// PreValidResult 预验证结果
type PreValidResult = sheinctx.PreValidResult

// SkcErrorMessage SKC错误信息
type SkcErrorMessage = sheinctx.SkcErrorMessage

// NewTaskContext 创建新的任务上下文
var NewTaskContext = sheinctx.NewTaskContext