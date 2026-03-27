// Package pipeline 提供公共处理器基础实现
package pipeline

import (
	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// BaseHandler 提供处理器名称和日志的最小公共实现，通过嵌入复用。
type BaseHandler struct {
	name   string
	logger *logrus.Entry
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(name string) *BaseHandler {
	return &BaseHandler{
		name:   name,
		logger: logger.GetGlobalLogger("pipeline/base_handler.go").WithField("handler", name),
	}
}

// Name 返回处理器名称
func (bh *BaseHandler) Name() string {
	return bh.name
}

// Logger 获取日志器
func (bh *BaseHandler) Logger() *logrus.Entry {
	return bh.logger
}
