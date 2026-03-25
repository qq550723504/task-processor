// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

// TaskFactoryCreator 定义平台任务工厂的创建函数。
type TaskFactoryCreator func(cfg *config.Config) scheduler.TaskFactory

// SchedulerDependencies 描述调度服务需要的可注入依赖。
type SchedulerDependencies struct {
	TemuFactoryCreator  TaskFactoryCreator
	SheinFactoryCreator TaskFactoryCreator
}

func (s *schedulerServiceImpl) resolveTemuFactoryCreator() TaskFactoryCreator {
	return s.temuFactoryCreator
}

func (s *schedulerServiceImpl) resolveSheinFactoryCreator() TaskFactoryCreator {
	return s.sheinFactoryCreator
}
