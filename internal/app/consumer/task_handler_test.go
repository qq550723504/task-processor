package consumer

import (
	"context"
	"testing"

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
