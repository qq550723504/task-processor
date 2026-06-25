package consumer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/ports/managementapi"
	"task-processor/internal/taskstatus"

	"github.com/sirupsen/logrus"
)

type stubProcessor struct {
	processCalls int
	onProcess    func()
}

func (s *stubProcessor) Start(ctx context.Context) error { return nil }

func (s *stubProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	s.processCalls++
	if s.onProcess != nil {
		s.onProcess()
	}
	return nil
}

func (s *stubProcessor) Close(ctx context.Context) {}

type stubProcessorWithManagement struct {
	stubProcessor
	runtime taskstatus.RuntimeWithTaskRPC
}

func (s *stubProcessorWithManagement) GetTaskStatusRuntime() taskstatus.RuntimeWithTaskRPC {
	return s.runtime
}

type stubRuntimeWithImportTask struct {
	task    *listingruntime.ImportTask
	status  *taskstatus.TaskStatusSnapshot
	updates int
}

func (s *stubRuntimeWithImportTask) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if s.task == nil {
		return fmt.Errorf("task not found")
	}
	if req.ExpectedCurrentStatus != nil && s.task.Status != *req.ExpectedCurrentStatus {
		return fmt.Errorf("Management API error 409")
	}
	s.updates++
	s.task.Status = req.Status
	return nil
}

func (s *stubRuntimeWithImportTask) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if s.status != nil {
		return s.status, nil
	}
	return &taskstatus.TaskStatusSnapshot{
		TaskID:          taskID,
		Status:          "PROCESSING",
		StatusKey:       "PROCESSING",
		CanonicalStatus: "processing",
	}, nil
}

func (s *stubRuntimeWithImportTask) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if s.task == nil || s.task.ID != taskID {
		return nil, fmt.Errorf("task %d not found", taskID)
	}
	taskCopy := *s.task
	return &taskCopy, nil
}

type stubStoreAPI struct {
	store *managementapi.StoreRespDTO
	err   error
}

func (s *stubStoreAPI) GetStore(id int64) (*managementapi.StoreRespDTO, error) {
	return s.store, s.err
}

func (s *stubStoreAPI) PageStores(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
	return &managementapi.PageResult[*managementapi.StoreRespDTO]{}, nil
}

func (s *stubStoreAPI) GetStoreCookie(id int64) (string, error) { return "", nil }

func (s *stubStoreAPI) UpdateStoreId(req *managementapi.StoreIdUpdateReqDTO) (bool, error) {
	return true, nil
}

func (s *stubStoreAPI) UpdateStoreStatus(req *managementapi.StoreStatusUpdateReqDTO) (bool, error) {
	return true, nil
}

func (s *stubStoreAPI) DeleteStoreCookie(id int64) (bool, error) { return true, nil }

func (s *stubStoreAPI) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	return true, nil
}

func (s *stubStoreAPI) GetStorePauseStatus(id int64) (bool, error) {
	return false, nil
}

func (s *stubStoreAPI) GetStorePauseStatusDetail(id int64) (*managementapi.StorePauseStatusRespDTO, error) {
	return &managementapi.StorePauseStatusRespDTO{}, nil
}

func TestTaskHandlerHandleMessage_SkipsDisabledStore(t *testing.T) {
	autoListingDisabled := false
	processor := &stubProcessor{}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                68,
				Name:              "disabled-store",
				Status:            storeStatusDisabled,
				EnableAutoListing: &autoListingDisabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-1",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(1001),
			"tenantId":       float64(2001),
			"storeId":        float64(68),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B012345678",
			"priority":       float64(5),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("expected disabled store message to be acked without error, got %v", err)
	}
	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called for disabled store, got %d calls", processor.processCalls)
	}
}

func TestTaskHandlerHandleMessage_ProcessesEnabledStore(t *testing.T) {
	autoListingEnabled := true
	processor := &stubProcessor{}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                177,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-2",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(1002),
			"tenantId":       float64(2002),
			"storeId":        float64(177),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B087654321",
			"priority":       float64(5),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("expected enabled store message to be processed, got %v", err)
	}
	if processor.processCalls != 1 {
		t.Fatalf("expected processor to be called once, got %d calls", processor.processCalls)
	}
}

func TestTaskHandlerHandleMessage_SkipsAutoListingDisabledStore(t *testing.T) {
	autoListingDisabled := false
	processor := &stubProcessor{}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                424,
				Name:              "status-enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingDisabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-status-enabled-auto-disabled",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(1003),
			"tenantId":       float64(246),
			"storeId":        float64(424),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0STATUS0",
			"priority":       float64(5),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("expected auto-listing-disabled store message to be skipped without error, got %v", err)
	}
	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called, got %d calls", processor.processCalls)
	}
}

func TestTaskHandlerHandleMessage_ClaimsQueuedTaskBeforeProcessing(t *testing.T) {
	autoListingEnabled := true
	var updateCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/import-task/update-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		atomic.AddInt32(&updateCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":true}`))
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("token", "1")

	processor := &stubProcessorWithManagement{runtime: management.NewTaskStatusRuntime(clientMgr)}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-claim-1",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812001),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("expected enabled store message to be processed, got %v", err)
	}
	if processor.processCalls != 1 {
		t.Fatalf("expected processor to be called once, got %d calls", processor.processCalls)
	}
	if atomic.LoadInt32(&updateCalls) != 1 {
		t.Fatalf("expected one claim update call, got %d", atomic.LoadInt32(&updateCalls))
	}
}

func TestTaskHandlerHandleMessageWithAck_AcksAfterClaimBeforeProcessing(t *testing.T) {
	autoListingEnabled := true
	var updateCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/import-task/update-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		atomic.AddInt32(&updateCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":true}`))
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("token", "1")

	var acked atomic.Bool
	var processSawAck atomic.Bool
	processor := &stubProcessorWithManagement{
		stubProcessor: stubProcessor{onProcess: func() {
			processSawAck.Store(acked.Load())
		}},
		runtime: management.NewTaskStatusRuntime(clientMgr),
	}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-early-ack-claim",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812021),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessageWithAck(context.Background(), msg, func() error {
		if atomic.LoadInt32(&updateCalls) != 1 {
			t.Fatalf("expected claim before ack, got %d claim calls", atomic.LoadInt32(&updateCalls))
		}
		acked.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("expected early-acked message to process, got %v", err)
	}
	if processor.processCalls != 1 {
		t.Fatalf("expected processor to be called once, got %d calls", processor.processCalls)
	}
	if !processSawAck.Load() {
		t.Fatal("expected processor to start after RabbitMQ ack")
	}
}

func TestTaskHandlerHandleMessageWithAck_StopsWhenAckFails(t *testing.T) {
	autoListingEnabled := true
	var updateCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/import-task/update-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		atomic.AddInt32(&updateCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":true}`))
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("token", "1")

	processor := &stubProcessorWithManagement{runtime: management.NewTaskStatusRuntime(clientMgr)}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-early-ack-fails",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812022),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	err := handler.HandleMessageWithAck(context.Background(), msg, func() error {
		return errors.New("ack channel closed")
	})
	if err == nil {
		t.Fatal("expected ack error to stop processing")
	}
	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to run after ack failure, got %d calls", processor.processCalls)
	}
	if atomic.LoadInt32(&updateCalls) != 1 {
		t.Fatalf("expected claim attempt before ack failure, got %d", atomic.LoadInt32(&updateCalls))
	}
}

func TestTaskHandlerHandleMessage_StopsWhenClaimRejected(t *testing.T) {
	autoListingEnabled := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rpc-api/listing/import-task/update-status":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":false}`))
		case "/rpc-api/listing/task/status":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"taskId":7812001,"status":"PROCESSING","statusKey":"PROCESSING","statusName":"处理中","canonicalStatus":"processing"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("token", "1")

	processor := &stubProcessorWithManagement{runtime: management.NewTaskStatusRuntime(clientMgr)}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-claim-2",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812001),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err == nil {
		t.Fatal("expected claim rejection to stop processing")
	}
	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called when claim is rejected, got %d calls", processor.processCalls)
	}
}

func TestTaskHandlerHandleMessage_ReloadsTaskFromRuntimeBeforeParsingLegacyPayload(t *testing.T) {
	autoListingEnabled := true
	runtime := &stubRuntimeWithImportTask{
		task: &listingruntime.ImportTask{
			ID:         7812010,
			TenantID:   286,
			StoreID:    846,
			Platform:   "shein",
			Region:     "US",
			ProductID:  "B0BGPRQ6N9",
			Status:     model.TaskStatusQueued.Int16(),
			RetryCount: 0,
			Priority:   10,
			CreateTime: 1710000000000,
		},
	}
	processor := &stubProcessorWithManagement{runtime: runtime}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-legacy-created-at",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812010),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0OLDPAYLOAD",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"createdAt":      "58429-01-24 23:20:12",
			"status":         "queued",
		},
	}

	if err := handler.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("expected runtime-loaded task to process despite legacy payload, got %v", err)
	}
	if processor.processCalls != 1 {
		t.Fatalf("expected processor to be called once, got %d", processor.processCalls)
	}
	if runtime.updates != 1 {
		t.Fatalf("expected one claim update, got %d", runtime.updates)
	}
}

func TestTaskHandlerHandleMessage_DiscardsRuntimeProcessingTask(t *testing.T) {
	autoListingEnabled := true
	runtime := &stubRuntimeWithImportTask{
		task: &listingruntime.ImportTask{
			ID:         7812011,
			TenantID:   286,
			StoreID:    846,
			Platform:   "shein",
			Region:     "US",
			ProductID:  "B0BGPRQ6N9",
			Status:     model.TaskStatusProcessing.Int16(),
			RetryCount: 0,
			Priority:   10,
			CreateTime: 1710000000000,
		},
	}
	processor := &stubProcessorWithManagement{runtime: runtime}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-processing",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812011),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected processing task message to be discarded")
	}
	type discardable interface {
		ShouldDiscard() bool
	}
	var discardErr discardable
	if !errors.As(err, &discardErr) || !discardErr.ShouldDiscard() {
		t.Fatalf("expected discardable error, got %v", err)
	}
	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called, got %d calls", processor.processCalls)
	}
	if runtime.updates != 0 {
		t.Fatalf("expected no claim update, got %d", runtime.updates)
	}
}

func TestTaskHandlerHandleMessage_DiscardsPausedMessageBeforeClaim(t *testing.T) {
	autoListingEnabled := true
	processor := &stubProcessor{}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-paused-1",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812002),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0BGPRQ6N9",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "paused",
		},
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected paused message to be discarded")
	}

	type discardable interface {
		ShouldDiscard() bool
	}
	var discardErr discardable
	if !errors.As(err, &discardErr) || !discardErr.ShouldDiscard() {
		t.Fatalf("expected discardable error, got %v", err)
	}

	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called for paused message, got %d calls", processor.processCalls)
	}
}

func TestTaskHandlerHandleMessage_DiscardsNonDebugASINBeforeProcessing(t *testing.T) {
	const envKey = "TASK_PROCESSOR_DEBUG_ONLY_ASIN"
	original := os.Getenv(envKey)
	if err := os.Setenv(envKey, "B0BPF6V5V6"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	t.Cleanup(func() {
		if original == "" {
			_ = os.Unsetenv(envKey)
			return
		}
		_ = os.Setenv(envKey, original)
	})

	autoListingEnabled := true
	processor := &stubProcessor{}
	handler := NewTaskHandler(TaskHandlerConfig{
		Platform:  "shein",
		Processor: processor,
		StoreAPI: &stubStoreAPI{
			store: &managementapi.StoreRespDTO{
				ID:                846,
				Name:              "enabled-store",
				Status:            storeStatusEnabled,
				EnableAutoListing: &autoListingEnabled,
			},
		},
		Logger: logrus.New(),
	})

	msg := &rabbitmq.Message{
		ID:   "msg-debug-asin-1",
		Type: "task",
		Payload: map[string]any{
			"taskId":         float64(7812003),
			"tenantId":       float64(286),
			"storeId":        float64(846),
			"sourcePlatform": "amazon",
			"targetPlatform": "shein",
			"region":         "US",
			"productId":      "B0DC9NVDBY",
			"priority":       float64(10),
			"retryCount":     float64(0),
			"maxRetryCount":  float64(3),
			"status":         "queued",
		},
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected non-debug ASIN message to be discarded")
	}

	type discardable interface {
		ShouldDiscard() bool
	}
	var discardErr discardable
	if !errors.As(err, &discardErr) || !discardErr.ShouldDiscard() {
		t.Fatalf("expected discardable error, got %v", err)
	}

	if processor.processCalls != 0 {
		t.Fatalf("expected processor not to be called for non-debug ASIN, got %d calls", processor.processCalls)
	}
}
