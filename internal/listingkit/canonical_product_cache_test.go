package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestRunWorkflowReusesCanonicalProductCacheWithoutSkippingDownstreamAssembly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newCanonicalProductCacheTestRepo()
	productSvc := &canonicalProductCacheProductService{
		product: &productenrich.ProductJSON{
			Title:         "Base Product Title",
			Category:      []string{"home", "decor"},
			Description:   "cached product description",
			Attributes:    map[string]string{"color": "black"},
			SellingPoints: []string{"silent movement"},
		},
	}
	assembler := &canonicalProductCacheAssembler{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: productSvc,
		Assembler:      assembler,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	firstTask, err := svc.CreateGenerateTask(ctx, canonicalProductCacheTestRequest("SDS Specific Title"))
	if err != nil {
		t.Fatalf("CreateGenerateTask(first) error = %v", err)
	}
	firstResult, err := svc.ProcessListingKit(ctx, firstTask)
	if err != nil {
		t.Fatalf("ProcessListingKit(first) error = %v", err)
	}
	if firstResult.CanonicalProduct == nil || firstResult.CanonicalProduct.Title != "SDS Specific Title" {
		t.Fatalf("first canonical title = %+v, want SDS-specific title", firstResult.CanonicalProduct)
	}
	if productSvc.createCalls != 1 || productSvc.processCalls != 1 {
		t.Fatalf("first workflow calls = create %d process %d, want 1/1", productSvc.createCalls, productSvc.processCalls)
	}

	productSvc.failIfCalled = true
	secondTask, err := svc.CreateGenerateTask(ctx, canonicalProductCacheTestRequest(""))
	if err != nil {
		t.Fatalf("CreateGenerateTask(second) error = %v", err)
	}
	secondResult, err := svc.ProcessListingKit(ctx, secondTask)
	if err != nil {
		t.Fatalf("ProcessListingKit(second) error = %v", err)
	}

	if productSvc.createCalls != 1 || productSvc.processCalls != 1 {
		t.Fatalf("cached workflow calls = create %d process %d, want still 1/1", productSvc.createCalls, productSvc.processCalls)
	}
	if assembler.calls != 2 {
		t.Fatalf("assembler calls = %d, want 2 so downstream workflow still runs", assembler.calls)
	}
	if secondResult.TaskID != secondTask.ID {
		t.Fatalf("second result task id = %q, want %q", secondResult.TaskID, secondTask.ID)
	}
	if secondResult.CanonicalProduct == nil || secondResult.CanonicalProduct.Title != "Base Product Title" {
		t.Fatalf("second canonical title = %+v, want base cached canonical without first SDS title", secondResult.CanonicalProduct)
	}
	if len(secondResult.ChildTasks) == 0 || secondResult.ChildTasks[0].Kind != "product_enrich" || secondResult.ChildTasks[0].Status != string(productenrich.TaskStatusCompleted) {
		t.Fatalf("second child tasks = %+v, want completed product_enrich cache hit", secondResult.ChildTasks)
	}
}

func canonicalProductCacheTestRequest(sdsTitle string) *GenerateRequest {
	req := &GenerateRequest{
		Text:      "same source product",
		Platforms: []string{"shein"},
		Country:   "US",
		Language:  "en",
		Options:   &GenerateOptions{ProcessImages: false},
	}
	if sdsTitle != "" {
		req.Options.SDS = &SDSSyncOptions{ProductName: sdsTitle}
	}
	return req
}

type canonicalProductCacheProductService struct {
	mu           sync.Mutex
	product      *productenrich.ProductJSON
	failIfCalled bool
	createCalls  int
	processCalls int
}

func (s *canonicalProductCacheProductService) CreateGenerateTask(_ context.Context, req *productenrich.GenerateRequest) (*productenrich.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failIfCalled {
		return nil, errors.New("product enrich task was called instead of canonical cache")
	}
	s.createCalls++
	return &productenrich.Task{
		ID:      fmt.Sprintf("product-task-%d", s.createCalls),
		Request: req,
	}, nil
}

func (s *canonicalProductCacheProductService) GetTaskResult(context.Context, string) (*productenrich.TaskResult, error) {
	return nil, nil
}

func (s *canonicalProductCacheProductService) ProcessProduct(context.Context, *productenrich.Task) (*productenrich.ProductJSON, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failIfCalled {
		return nil, errors.New("product enrich processing was called instead of canonical cache")
	}
	s.processCalls++
	return s.product, nil
}

type canonicalProductCacheAssembler struct {
	calls int
}

func (a *canonicalProductCacheAssembler) Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult {
	a.calls++
	return &ListingKitResult{
		TaskID:           task.ID,
		Status:           string(TaskStatusProcessing),
		Platforms:        append([]string(nil), task.Request.Platforms...),
		CanonicalProduct: cloneCanonicalProductForCacheTest(canonical),
		Summary:          &GenerationSummary{SourceType: "text", NeedsReview: false},
	}
}

type canonicalProductCacheTestRepo struct {
	mu             sync.Mutex
	tasks          map[string]*Task
	canonicalCache map[string]*canonical.Product
}

func newCanonicalProductCacheTestRepo() *canonicalProductCacheTestRepo {
	return &canonicalProductCacheTestRepo{
		tasks:          map[string]*Task{},
		canonicalCache: map[string]*canonical.Product{},
	}
}

func (r *canonicalProductCacheTestRepo) CreateTask(_ context.Context, task *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = cloneCanonicalProductCacheTestTask(task)
	return nil
}

func (r *canonicalProductCacheTestRepo) GetTask(_ context.Context, taskID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return cloneCanonicalProductCacheTestTask(task), nil
}

func (r *canonicalProductCacheTestRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}

func (r *canonicalProductCacheTestRepo) MarkProcessing(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if task.Status != TaskStatusPending {
		return ErrTaskNotPending
	}
	task.Status = TaskStatusProcessing
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[taskID]
	task.Status = TaskStatusCompleted
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[taskID]
	task.Status = TaskStatusNeedsReview
	task.Error = reason
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) PrepareRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.RetryCount++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) SaveTaskResult(_ context.Context, taskID string, result *ListingKitResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

func (r *canonicalProductCacheTestRepo) GetCanonicalProductCache(_ context.Context, fingerprint string) (*canonical.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneCanonicalProductForCacheTest(r.canonicalCache[fingerprint]), nil
}

func (r *canonicalProductCacheTestRepo) SaveCanonicalProductCache(_ context.Context, fingerprint string, product *canonical.Product, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.canonicalCache[fingerprint] = cloneCanonicalProductForCacheTest(product)
	return nil
}

func cloneCanonicalProductCacheTestTask(task *Task) *Task {
	if task == nil {
		return nil
	}
	copied := *task
	return &copied
}

func cloneCanonicalProductForCacheTest(product *canonical.Product) *canonical.Product {
	if product == nil {
		return nil
	}
	raw, _ := json.Marshal(product)
	var cloned canonical.Product
	_ = json.Unmarshal(raw, &cloned)
	return &cloned
}
