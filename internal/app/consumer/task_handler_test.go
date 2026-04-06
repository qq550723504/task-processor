package consumer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

type stubProcessor struct {
	processCalls int
}

func (s *stubProcessor) Start(ctx context.Context) error { return nil }

func (s *stubProcessor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	s.processCalls++
	return nil
}

func (s *stubProcessor) Close(ctx context.Context) {}

type stubProcessorWithManagement struct {
	stubProcessor
	clientMgr *management.ClientManager
}

func (s *stubProcessorWithManagement) GetManagementClient() *management.ClientManager {
	return s.clientMgr
}

type stubStoreAPI struct {
	store *managementapi.StoreRespDTO
	err   error
}

func (s *stubStoreAPI) GetStore(id int64) (*managementapi.StoreRespDTO, error) {
	return s.store, s.err
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

	processor := &stubProcessorWithManagement{clientMgr: clientMgr}
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

func TestTaskHandlerHandleMessage_StopsWhenClaimRejected(t *testing.T) {
	autoListingEnabled := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/import-task/update-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":false}`))
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("token", "1")

	processor := &stubProcessorWithManagement{clientMgr: clientMgr}
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
