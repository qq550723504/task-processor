package listing

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/worker"
	"task-processor/internal/model"
)

func TestBuildDebugModelTaskMapsImportTaskDTO(t *testing.T) {
	dto := &managementapi.ProductImportTaskRespDTO{
		ID:            8189311,
		TenantID:      2001,
		StoreID:       3002,
		Platform:      "shein",
		Region:        "US",
		CategoryID:    4003,
		ProductID:     "B0F17JCXFJ",
		Status:        2,
		ErrorMessage:  "old error",
		RetryCount:    1,
		MaxRetryCount: 3,
		Remark:        "variant",
		Priority:      5,
		CreateTime:    111,
		UpdateTime:    222,
		Creator:       "tester",
		Updater:       "tester2",
	}

	task := buildDebugModelTask(dto)

	if task.ID != dto.ID || task.TenantID != dto.TenantID || task.StoreID != dto.StoreID {
		t.Fatalf("task identity = %+v, want dto identity %+v", task, dto)
	}
	if task.ProductID != dto.ProductID || task.Platform != dto.Platform || task.Region != dto.Region {
		t.Fatalf("task routing = %+v, want dto routing %+v", task, dto)
	}
	if task.ErrorMessage != dto.ErrorMessage || task.Remark != dto.Remark || task.Updater != dto.Updater {
		t.Fatalf("task metadata = %+v, want dto metadata %+v", task, dto)
	}
}

func TestBuildDebugWorkerJobBuildsDirectProcessorPayload(t *testing.T) {
	task := model.Task{
		ID:        8189311,
		TenantID:  2001,
		StoreID:   3002,
		Platform:  "shein",
		ProductID: "B0F17JCXFJ",
	}

	job, err := buildDebugWorkerJob(task)
	if err != nil {
		t.Fatalf("buildDebugWorkerJob() error = %v", err)
	}
	if job.TenantID != "2001" || job.ShopID != "3002" {
		t.Fatalf("job routing = %+v, want tenant/store ids", job)
	}

	var decoded model.Task
	if err := json.Unmarshal([]byte(job.TaskData), &decoded); err != nil {
		t.Fatalf("unmarshal task data: %v", err)
	}
	if decoded.ID != task.ID || decoded.ProductID != task.ProductID {
		t.Fatalf("decoded task = %+v, want %+v", decoded, task)
	}
}

func TestDebugTaskRunnerRunStartsProcessesAndCloses(t *testing.T) {
	processor := &stubDebugTaskProcessor{}
	runner := debugTaskRunner{
		displayName: "SHEIN",
		logger:      stubDebugLogger{},
		taskLoader: staticDebugTaskLoader{task: &managementapi.ProductImportTaskRespDTO{
			ID:        8189311,
			TenantID:  2001,
			StoreID:   3002,
			Platform:  "shein",
			ProductID: "B0F17JCXFJ",
		}},
		processor: processor,
	}

	if err := runner.run(context.Background(), 8189311); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !processor.started || !processor.closed {
		t.Fatalf("processor lifecycle = %+v, want started and closed", processor)
	}
	if processor.lastJob.TenantID != "2001" || processor.lastJob.ShopID != "3002" {
		t.Fatalf("processor job = %+v, want tenant/store routing", processor.lastJob)
	}
}

func TestDebugTaskRunnerRunReturnsTaskNotFound(t *testing.T) {
	runner := debugTaskRunner{
		displayName: "SHEIN",
		logger:      stubDebugLogger{},
		taskLoader:  staticDebugTaskLoader{},
		processor:   &stubDebugTaskProcessor{},
	}

	err := runner.run(context.Background(), 123)
	if err == nil || err.Error() != "debug task 123 not found" {
		t.Fatalf("run() error = %v, want task not found", err)
	}
}

func TestDebugTaskRunnerRunPropagatesProcessorFailure(t *testing.T) {
	processor := &stubDebugTaskProcessor{processErr: errors.New("boom")}
	runner := debugTaskRunner{
		displayName: "SHEIN",
		logger:      stubDebugLogger{},
		taskLoader: staticDebugTaskLoader{task: &managementapi.ProductImportTaskRespDTO{
			ID:        8189311,
			TenantID:  2001,
			StoreID:   3002,
			Platform:  "shein",
			ProductID: "B0F17JCXFJ",
		}},
		processor: processor,
	}

	err := runner.run(context.Background(), 8189311)
	if err == nil || err.Error() != "process debug task 8189311 failed: boom" {
		t.Fatalf("run() error = %v, want processor failure", err)
	}
	if !processor.closed {
		t.Fatal("processor should still be closed when processing fails")
	}
}

type stubDebugTaskProcessor struct {
	started    bool
	closed     bool
	lastJob    worker.WorkerJob
	startErr   error
	processErr error
}

func (p *stubDebugTaskProcessor) Start(context.Context) error {
	p.started = true
	return p.startErr
}

func (p *stubDebugTaskProcessor) ProcessTask(_ context.Context, job worker.WorkerJob) error {
	p.lastJob = job
	return p.processErr
}

func (p *stubDebugTaskProcessor) Close(context.Context) {
	p.closed = true
}

type stubDebugLogger struct{}

func (stubDebugLogger) Infof(string, ...any) {}
