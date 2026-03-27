package publish

import (
	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type TaskStatusNotifier struct {
	component string
	logger    *logrus.Entry
}

func NewTaskStatusNotifier(component string, log *logrus.Entry) *TaskStatusNotifier {
	if log == nil {
		log = logger.GetGlobalLogger(component)
	}
	return &TaskStatusNotifier{
		component: component,
		logger:    log,
	}
}

func (n *TaskStatusNotifier) Notify(input *TaskStatusUpdateInput, targetStatus model.TaskStatus, errorMsg string) {
	if input == nil {
		n.logger.Warn("task status update input is nil, skip task status update")
		return
	}
	if input.ManagementClientMgr == nil {
		n.logger.Warn("management client manager is nil, skip task status update")
		return
	}
	if input.Task == nil {
		n.logger.Warn("task is nil, skip task status update")
		return
	}

	statusService := taskstatus.NewService(n.component, func() taskstatus.ImportTaskStatusClient {
		return input.ManagementClientMgr.GetImportTaskClient()
	})

	if err := statusService.TransitionAsync(input.Task.ID, model.TaskStatusProcessing, targetStatus, errorMsg); err != nil {
		n.logger.WithError(err).WithField("task_id", input.Task.ID).Error("update task status failed")
	}
}
