package taskstatus

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
)

type stubImportTaskClient struct {
	lastReq   *managementapi.ProductImportTaskUpdateReqDTO
	err       error
	attempts  int32
	failures  int32
	handlerFn func(*managementapi.ProductImportTaskUpdateReqDTO) error
}

func (s *stubImportTaskClient) UpdateTaskStatus(req *managementapi.ProductImportTaskUpdateReqDTO) error {
	copied := *req
	s.lastReq = &copied
	atomic.AddInt32(&s.attempts, 1)
	if s.handlerFn != nil {
		return s.handlerFn(req)
	}
	if s.failures > 0 {
		s.failures--
		return fmt.Errorf("temporary management failure")
	}
	if s.err != nil {
		return s.err
	}
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

func TestServiceUpdateSyncWithInputParsesReasonCodeAndStageFromErrorMessage(t *testing.T) {
	client := &stubImportTaskClient{}
	service := NewService("test", func() ImportTaskStatusClient { return client })

	err := service.UpdateSyncWithInput(UpdateInput{
		TaskID:       3,
		Status:       model.TaskStatusPaused,
		ErrorMessage: "[stage:check_daily_limit] [DAILY_LIMIT_REACHED] daily limit reached",
	})
	if err != nil {
		t.Fatalf("UpdateSyncWithInput returned error: %v", err)
	}
	if client.lastReq == nil {
		t.Fatal("UpdateSyncWithInput should call UpdateTaskStatus")
	}
	if client.lastReq.ReasonCode != "DAILY_LIMIT_REACHED" {
		t.Fatalf("ReasonCode = %q, want DAILY_LIMIT_REACHED", client.lastReq.ReasonCode)
	}
	if client.lastReq.Stage != "check_daily_limit" {
		t.Fatalf("Stage = %q, want check_daily_limit", client.lastReq.Stage)
	}
}

func TestServiceUpdateSyncWithInputPrefersExplicitReasonCodeAndStage(t *testing.T) {
	client := &stubImportTaskClient{}
	service := NewService("test", func() ImportTaskStatusClient { return client })

	err := service.UpdateSyncWithInput(UpdateInput{
		TaskID:       4,
		Status:       model.TaskStatusTerminated,
		ErrorMessage: "[stage:publish_product] [SKU_DUPLICATED] duplicate sku",
		ReasonCode:   "MANUAL_OVERRIDE",
		Stage:        "custom_stage",
	})
	if err != nil {
		t.Fatalf("UpdateSyncWithInput returned error: %v", err)
	}
	if client.lastReq == nil {
		t.Fatal("UpdateSyncWithInput should call UpdateTaskStatus")
	}
	if client.lastReq.ReasonCode != "MANUAL_OVERRIDE" {
		t.Fatalf("ReasonCode = %q, want MANUAL_OVERRIDE", client.lastReq.ReasonCode)
	}
	if client.lastReq.Stage != "custom_stage" {
		t.Fatalf("Stage = %q, want custom_stage", client.lastReq.Stage)
	}
}

func TestServiceTransitionSyncWithInputIgnoresConflictWhenConfigured(t *testing.T) {
	client := &stubImportTaskClient{err: fmt.Errorf("更新任务状态失败: Management API error 409: ")}
	service := NewService("test", func() ImportTaskStatusClient { return client })

	err := service.TransitionSyncWithInput(model.TaskStatusProcessing, UpdateInput{
		TaskID:         5,
		Status:         model.TaskStatusTerminated,
		ErrorMessage:   "already handled",
		IgnoreConflict: true,
	})
	if err != nil {
		t.Fatalf("TransitionSyncWithInput returned error: %v", err)
	}
	if client.lastReq == nil {
		t.Fatal("TransitionSyncWithInput should still call UpdateTaskStatus")
	}
}

func TestServiceUpdateSyncWithInputRetriesTemporaryFailureAndEventuallySucceeds(t *testing.T) {
	client := &stubImportTaskClient{failures: 1}
	service := NewService("test", func() ImportTaskStatusClient { return client })
	service.maxRetries = 2

	start := time.Now()
	err := service.UpdateSyncWithInput(UpdateInput{
		TaskID:       6,
		Status:       model.TaskStatusPendingRetry,
		ErrorMessage: "transient failure",
	})
	if err != nil {
		t.Fatalf("UpdateSyncWithInput returned error: %v", err)
	}

	attempts := atomic.LoadInt32(&client.attempts)
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
		t.Fatalf("retry path took too long: %s", elapsed)
	}
	if client.lastReq == nil {
		t.Fatal("UpdateSyncWithInput should eventually send request")
	}
}
