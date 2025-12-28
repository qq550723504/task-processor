// Package handlers 提供TEMU平台专用的Handler接口
package handlers

import (
	temucontext "task-processor/internal/platforms/temu/context"
)

// TemuHandler TEMU平台专用的处理器接口
// 直接接收强类型上下文，无需类型断言
type TemuHandler interface {
	Name() string
	HandleTemu(*temucontext.TemuTaskContext) error
}
