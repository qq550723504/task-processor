package scheduler

import appscheduler "task-processor/internal/app/scheduler"

type TaskType = appscheduler.TaskType

const (
	TaskTypePricing     = appscheduler.TaskTypePricing
	TaskTypeProductSync = appscheduler.TaskTypeProductSync
	TaskTypeInventory   = appscheduler.TaskTypeInventory
	TaskTypeActivity    = appscheduler.TaskTypeActivity
)

type TaskStatus = appscheduler.TaskStatus

const (
	TaskStatusRunning = appscheduler.TaskStatusRunning
	TaskStatusStopped = appscheduler.TaskStatusStopped
	TaskStatusError   = appscheduler.TaskStatusError
)

type Task = appscheduler.Task
type TaskConfig = appscheduler.TaskConfig
type TaskFactory = appscheduler.TaskFactory
type TaskResult = appscheduler.TaskResult
