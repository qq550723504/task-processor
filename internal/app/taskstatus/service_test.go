package taskstatus

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

type stubImportTaskClient struct {
	lastReq *managementapi.ProductImportTaskUpdateReqDTO
}

func (s *stubImportTaskClient) UpdateTaskStatus(req *managementapi.ProductImportTaskUpdateReqDTO) error {
	copied := *req
	s.lastReq = &copied
	return nil
}

func TestServiceUpdateSyncWithInputIncludesRetryMetadata(t *testing.T) {
	client := &stubImportTaskClient{}
	service := NewService("test", func() ImportTaskStatusClient { return client })
	retryCount := 2
	priority := 80

	err := service.UpdateSyncWithInput(UpdateInput{
		TaskID:       1,
		Status:       model.TaskStatusPendingRetry,
		ErrorMessage: "retry later",
		RetryCount:   &retryCount,
		Priority:     &priority,
	})
	if err != nil {
		t.Fatalf("UpdateSyncWithInput returned error: %v", err)
	}
	if client.lastReq == nil {
		t.Fatal("UpdateSyncWithInput should call UpdateTaskStatus")
	}
	if client.lastReq.RetryCount == nil || *client.lastReq.RetryCount != retryCount {
		t.Fatal("UpdateSyncWithInput should include retryCount")
	}
	if client.lastReq.Priority == nil || *client.lastReq.Priority != priority {
		t.Fatal("UpdateSyncWithInput should include priority")
	}
}

func TestServiceTransitionSyncWithInputPreservesRetryMetadata(t *testing.T) {
	client := &stubImportTaskClient{}
	service := NewService("test", func() ImportTaskStatusClient { return client })
	retryCount := 3
	priority := 70

	err := service.TransitionSyncWithInput(model.TaskStatusProcessing, UpdateInput{
		TaskID:       2,
		Status:       model.TaskStatusTerminated,
		ErrorMessage: "max retries",
		RetryCount:   &retryCount,
		Priority:     &priority,
	})
	if err != nil {
		t.Fatalf("TransitionSyncWithInput returned error: %v", err)
	}
	if client.lastReq == nil {
		t.Fatal("TransitionSyncWithInput should call UpdateTaskStatus")
	}
	if client.lastReq.Status != model.TaskStatusTerminated.Int16() {
		t.Fatalf("TransitionSyncWithInput status = %d, want %d", client.lastReq.Status, model.TaskStatusTerminated.Int16())
	}
	if client.lastReq.RetryCount == nil || *client.lastReq.RetryCount != retryCount {
		t.Fatal("TransitionSyncWithInput should include retryCount")
	}
	if client.lastReq.Priority == nil || *client.lastReq.Priority != priority {
		t.Fatal("TransitionSyncWithInput should include priority")
	}
}
