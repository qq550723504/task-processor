package taskstatus

import apptaskstatus "task-processor/internal/app/taskstatus"

type ImportTaskStatusClient = apptaskstatus.ImportTaskStatusClient
type UpdateInput = apptaskstatus.UpdateInput
type Service = apptaskstatus.Service

func NewService(component string, clientProvider func() ImportTaskStatusClient) *Service {
	return apptaskstatus.NewService(component, clientProvider)
}
