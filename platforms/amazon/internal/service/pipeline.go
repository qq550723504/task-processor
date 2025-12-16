// Package service 提供Amazon平台管道服务
package service

import (
	"fmt"
	"task-processor/platforms/amazon/internal/handler"
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// PipelineService 管道服务
type PipelineService struct {
	handlers []handler.Handler
	logger   *logrus.Entry
}

// NewPipelineService 创建管道服务
func NewPipelineService() *PipelineService {
	return &PipelineService{
		handlers: make([]handler.Handler, 0),
		logger:   logrus.WithField("service", "PipelineService"),
	}
}

// AddHandler 添加处理器
func (ps *PipelineService) AddHandler(h handler.Handler) {
	ps.handlers = append(ps.handlers, h)
	ps.logger.Infof("已添加处理器: %s", h.Name())
}

// Execute 执行管道
func (ps *PipelineService) Execute(services *model.Services, data map[string]any) error {
	ps.logger.Infof("开始执行管道，共 %d 个处理器", len(ps.handlers))

	for i, h := range ps.handlers {
		stepNum := i + 1
		ps.logger.Infof("执行步骤 [%d/%d]: %s", stepNum, len(ps.handlers), h.Name())

		if err := h.Execute(services, data); err != nil {
			ps.logger.Errorf("步骤执行失败 [%d/%d] [%s]: %v",
				stepNum, len(ps.handlers), h.Name(), err)
			return fmt.Errorf("步骤 [%s] 执行失败: %w", h.Name(), err)
		}

		ps.logger.Infof("步骤完成 [%d/%d]: %s", stepNum, len(ps.handlers), h.Name())
	}

	ps.logger.Info("管道执行完成")
	return nil
}

// GetHandlerCount 获取处理器数量
func (ps *PipelineService) GetHandlerCount() int {
	return len(ps.handlers)
}
