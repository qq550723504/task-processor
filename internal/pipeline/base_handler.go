// Package pipeline 提供公共处理器基类
package pipeline

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// BaseHandler 公共处理器基类
// 提供通用功能，减少重复代码
type BaseHandler struct {
	name   string
	logger *logrus.Entry
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(name string) *BaseHandler {
	return &BaseHandler{
		name:   name,
		logger: logrus.WithField("handler", name),
	}
}

// Name 返回处理器名称
func (bh *BaseHandler) Name() string {
	return bh.name
}

// GetLogger 获取日志器
func (bh *BaseHandler) GetLogger() *logrus.Entry {
	return bh.logger
}

// ValidateContext 验证上下文
func (bh *BaseHandler) ValidateContext(ctx TaskContext) error {
	if ctx == nil {
		return fmt.Errorf("任务上下文为空")
	}
	if ctx.GetTask() == nil {
		return fmt.Errorf("任务信息为空")
	}
	return nil
}

// GetRequiredData 获取必需的数据
func (bh *BaseHandler) GetRequiredData(ctx TaskContext, key string) (any, error) {
	value, exists := ctx.GetData(key)
	if !exists {
		return nil, fmt.Errorf("缺少必需数据: %s", key)
	}
	return value, nil
}

// GetOptionalData 获取可选数据
func (bh *BaseHandler) GetOptionalData(ctx TaskContext, key string, defaultValue any) any {
	if value, exists := ctx.GetData(key); exists {
		return value
	}
	return defaultValue
}

// SetResult 设置处理结果
func (bh *BaseHandler) SetResult(ctx TaskContext, key string, value any) {
	ctx.SetData(key, value)
	bh.logger.Debugf("设置结果: %s = %v", key, value)
}

// LogStart 记录处理开始
func (bh *BaseHandler) LogStart() {
	bh.logger.Infof("开始执行处理器: %s", bh.name)
}

// LogSuccess 记录处理成功
func (bh *BaseHandler) LogSuccess() {
	bh.logger.Infof("处理器执行成功: %s", bh.name)
}

// LogError 记录处理错误
func (bh *BaseHandler) LogError(err error) {
	bh.logger.Errorf("处理器执行失败: %s, 错误: %v", bh.name, err)
}
