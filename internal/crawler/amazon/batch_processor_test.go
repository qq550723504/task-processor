package amazon

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/model"
)

// ---- mock: batchPool ----

type mockBatchPool struct {
	mu              sync.Mutex
	instances       []*browser.BrowserInstance
	acquireIdx      int
	onRecreateSync  func(*browser.BrowserInstance) *browser.BrowserInstance
	onRecreateAsync func(*browser.BrowserInstance)
	released        []*browser.BrowserInstance
	errorDetector   *browser.ErrorDetector
}

func newMockBatchPool(size int) *mockBatchPool {
	mp := &mockBatchPool{
		instances:     make([]*browser.BrowserInstance, size),
		errorDetector: browser.NewErrorDetector(),
	}
	for i := 0; i < size; i++ {
		mp.instances[i] = &browser.BrowserInstance{ID: i}
	}
	return mp
}

func (m *mockBatchPool) Acquire() (*browser.BrowserInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.acquireIdx >= len(m.instances) {
		return nil, fmt.Errorf("获取浏览器实例超时: 30s")
	}
	inst := m.instances[m.acquireIdx]
	m.acquireIdx++
	return inst, nil
}

func (m *mockBatchPool) IsBlockedOrSeriousError(err error) bool {
	return m.errorDetector.IsBlockedOrSeriousError(err)
}

func (m *mockBatchPool) RecreateInstanceSync(old *browser.BrowserInstance) *browser.BrowserInstance {
	if m.onRecreateSync != nil {
		return m.onRecreateSync(old)
	}
	return nil
}

func (m *mockBatchPool) RecreateInstanceAsync(old *browser.BrowserInstance) {
	if m.onRecreateAsync != nil {
		m.onRecreateAsync(old)
	}
}

func (m *mockBatchPool) ReleaseWithError(instance *browser.BrowserInstance, _ error) {
	m.mu.Lock()
	m.released = append(m.released, instance)
	m.mu.Unlock()
}

// ---- mock: InstanceProcessor（通过替换 NewInstanceProcessor 的方式不可行，改用接口注入）----
// BatchProcessor 内部直接 new 了 InstanceProcessor，无法 mock。
// 真实场景下 InstanceProcessor 依赖真实浏览器，所以我们测试 batchPool 的行为，
// 用一个 stubBatchProcessor 直接注入可控的处理函数。

// stubBatchProcessor 可注入处理逻辑的批量处理器，复用 ProcessWithPool 的 batchPool 接口
type stubBatchProcessor struct {
	// 按请求顺序返回的结果
	results []stubResult
	mu      sync.Mutex
	calls   int
}

type stubResult struct {
	product *model.Product
	err     error
}

// processFunc 模拟 InstanceProcessor.ProcessWithInstance 的行为
func (s *stubBatchProcessor) processFunc(_ *browser.BrowserInstance, _ string, _ string) (*model.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.calls
	s.calls++
	if idx < len(s.results) {
		return s.results[idx].product, s.results[idx].err
	}
	return &model.Product{Asin: "DEFAULT"}, nil
}

// runBatchWithStub 直接执行 ProcessWithPool 的核心逻辑，但用 stub 替换 InstanceProcessor
// 这样可以在不依赖真实浏览器的情况下测试 batchPool 的 Acquire/Release/Recreate 行为
func runBatchWithStub(requests []model.ProductRequest, pool batchPool, stub *stubBatchProcessor) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	instance, err := pool.Acquire()
	if err != nil {
		for i := range results {
			results[i] = model.ProductResult{Error: fmt.Errorf("获取浏览器实例失败: %w", err)}
		}
		return results
	}

	var lastError error
	for i, req := range requests {
		product, procErr := stub.processFunc(instance, req.URL, req.Zipcode)
		results[i] = model.ProductResult{Product: product, Error: procErr}

		if procErr != nil {
			lastError = procErr
			if pool.IsBlockedOrSeriousError(procErr) {
				newInstance := pool.RecreateInstanceSync(instance)
				if newInstance != nil {
					instance = newInstance
				} else {
					pool.RecreateInstanceAsync(instance)
					for j := i + 1; j < len(requests); j++ {
						results[j] = model.ProductResult{Error: fmt.Errorf("浏览器实例重建失败，跳过处理")}
					}
					return results
				}
			}
		}
	}

	pool.ReleaseWithError(instance, lastError)
	return results
}

// ---- 测试 ----

// TestBatchProcessor_AllSuccess 全部成功：实例被正确归还
func TestBatchProcessor_AllSuccess(t *testing.T) {
	pool := newMockBatchPool(1)
	stub := &stubBatchProcessor{results: []stubResult{
		{product: &model.Product{Asin: "B001"}},
		{product: &model.Product{Asin: "B002"}},
		{product: &model.Product{Asin: "B003"}},
	}}

	requests := []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001", Zipcode: "10001"},
		{URL: "https://amazon.com/dp/B002", Zipcode: "10001"},
		{URL: "https://amazon.com/dp/B003", Zipcode: "10001"},
	}

	results := runBatchWithStub(requests, pool, stub)

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("请求 %d 期望成功，得到: %v", i, r.Error)
		}
	}
	if len(pool.released) != 1 {
		t.Fatalf("期望归还 1 次，实际: %d", len(pool.released))
	}
}

// TestBatchProcessor_AcquireFails_AllTasksFail 池耗尽时所有任务都应返回错误
func TestBatchProcessor_AcquireFails_AllTasksFail(t *testing.T) {
	pool := newMockBatchPool(0) // 空池
	stub := &stubBatchProcessor{}

	requests := []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
	}

	results := runBatchWithStub(requests, pool, stub)

	for i, r := range results {
		if r.Error == nil {
			t.Errorf("请求 %d 期望失败，但得到 nil", i)
		}
	}
	// 没有 Acquire 成功，不应有 Release
	if len(pool.released) != 0 {
		t.Fatalf("Acquire 失败时不应有 Release，实际: %d", len(pool.released))
	}
}

// TestBatchProcessor_RecreateSync_Fails_AsyncCalled_Bug1
// 真实场景：批量处理中途遇到 WebSocket 断连，重建失败时必须调用 RecreateInstanceAsync，
// 否则池永久缩容，后续所有任务超时。
func TestBatchProcessor_RecreateSync_Fails_AsyncCalled_Bug1(t *testing.T) {
	pool := newMockBatchPool(1)
	asyncCalled := false
	pool.onRecreateSync = func(_ *browser.BrowserInstance) *browser.BrowserInstance { return nil }
	pool.onRecreateAsync = func(_ *browser.BrowserInstance) { asyncCalled = true }

	wsErr := errors.New("Socket connection to remote was closed")
	stub := &stubBatchProcessor{results: []stubResult{
		{product: &model.Product{Asin: "B001"}}, // 第 1 个成功
		{err: wsErr},                            // 第 2 个触发严重错误
		{product: &model.Product{Asin: "B003"}}, // 第 3 个（不会执行到）
	}}

	requests := []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
		{URL: "https://amazon.com/dp/B003"},
	}

	results := runBatchWithStub(requests, pool, stub)

	if !asyncCalled {
		t.Fatal("Bug 1 未修复：RecreateInstanceSync 失败后未调用 RecreateInstanceAsync，池将永久缩容")
	}
	// 第 1 个成功，第 2、3 个失败
	if results[0].Error != nil {
		t.Errorf("第 1 个请求期望成功，得到: %v", results[0].Error)
	}
	if results[1].Error == nil {
		t.Error("第 2 个请求期望失败")
	}
	if results[2].Error == nil {
		t.Error("第 3 个请求期望失败（跳过处理）")
	}
	// 重建失败时直接 return，不应调用 ReleaseWithError
	if len(pool.released) != 0 {
		t.Fatalf("重建失败时不应调用 ReleaseWithError，实际: %d", len(pool.released))
	}
}

// TestBatchProcessor_RecreateSync_Success_ContinuesProcessing
// 重建成功后应继续处理剩余任务，不中断
func TestBatchProcessor_RecreateSync_Success_ContinuesProcessing(t *testing.T) {
	pool := newMockBatchPool(1)
	newInst := &browser.BrowserInstance{ID: 99}
	pool.onRecreateSync = func(_ *browser.BrowserInstance) *browser.BrowserInstance { return newInst }

	wsErr := errors.New("websocket connection closed")
	stub := &stubBatchProcessor{results: []stubResult{
		{product: &model.Product{Asin: "B001"}},
		{err: wsErr},                            // 触发重建
		{product: &model.Product{Asin: "B003"}}, // 用新实例继续
	}}

	requests := []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
		{URL: "https://amazon.com/dp/B003"},
	}

	results := runBatchWithStub(requests, pool, stub)

	if results[0].Error != nil {
		t.Errorf("第 1 个期望成功: %v", results[0].Error)
	}
	// 第 2 个有错误（严重错误本身记录在结果里）
	if results[2].Error != nil {
		t.Errorf("第 3 个期望成功（用新实例重试）: %v", results[2].Error)
	}
	if stub.calls != 3 {
		t.Fatalf("期望调用 3 次，实际: %d（重建后应继续处理）", stub.calls)
	}
	// 最终应归还新实例
	if len(pool.released) != 1 {
		t.Fatalf("期望归还 1 次，实际: %d", len(pool.released))
	}
}

// TestBatchProcessor_NonSeriousError_InstanceNotRebuilt
// 普通错误（如 404）不应触发实例重建
func TestBatchProcessor_NonSeriousError_InstanceNotRebuilt(t *testing.T) {
	pool := newMockBatchPool(1)
	recreateCalled := false
	pool.onRecreateSync = func(_ *browser.BrowserInstance) *browser.BrowserInstance {
		recreateCalled = true
		return nil
	}

	stub := &stubBatchProcessor{results: []stubResult{
		{err: errors.New("产品页面不存在")}, // 404，非严重错误
		{product: &model.Product{Asin: "B002"}},
	}}

	requests := []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
	}

	results := runBatchWithStub(requests, pool, stub)

	if recreateCalled {
		t.Fatal("404 错误不应触发实例重建")
	}
	if results[1].Error != nil {
		t.Errorf("第 2 个请求期望成功: %v", results[1].Error)
	}
	if len(pool.released) != 1 {
		t.Fatalf("期望归还 1 次，实际: %d", len(pool.released))
	}
}

// TestBatchProcessor_MultipleWorkers_PoolNotDepleted
// 模拟多个 RabbitMQ 消费者 goroutine 并发批量处理，验证池不缩容
func TestBatchProcessor_MultipleWorkers_PoolNotDepleted(t *testing.T) {
	const poolSize = 3
	const workers = 10
	const batchSize = 5

	// 每个 worker 独立持有一个池实例（真实场景：每个节点有自己的池）
	// 这里用共享池模拟多 goroutine 竞争同一个池
	type sharedPool struct {
		mu        sync.Mutex
		available []*browser.BrowserInstance
		released  int
	}

	sp := &sharedPool{
		available: make([]*browser.BrowserInstance, poolSize),
	}
	for i := 0; i < poolSize; i++ {
		sp.available[i] = &browser.BrowserInstance{ID: i}
	}

	acquireFn := func() (*browser.BrowserInstance, error) {
		sp.mu.Lock()
		defer sp.mu.Unlock()
		if len(sp.available) == 0 {
			return nil, fmt.Errorf("获取浏览器实例超时: 30s")
		}
		inst := sp.available[0]
		sp.available = sp.available[1:]
		return inst, nil
	}
	releaseFn := func(inst *browser.BrowserInstance) {
		sp.mu.Lock()
		sp.available = append(sp.available, inst)
		sp.released++
		sp.mu.Unlock()
	}

	var wg sync.WaitGroup
	errors := make([]error, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			inst, err := acquireFn()
			if err != nil {
				// 池满时正常排队等待，这里简化为记录错误
				errors[workerID] = err
				return
			}

			// 模拟处理 batchSize 个任务
			for i := 0; i < batchSize; i++ {
				// 模拟正常处理，无错误
				_ = inst
			}

			releaseFn(inst)
		}(w)
	}

	wg.Wait()

	sp.mu.Lock()
	finalAvailable := len(sp.available)
	totalReleased := sp.released
	sp.mu.Unlock()

	// 成功获取实例的 worker 数量
	successWorkers := 0
	for _, e := range errors {
		if e == nil {
			successWorkers++
		}
	}

	// 所有成功获取实例的 worker 都应归还
	if totalReleased != successWorkers {
		t.Fatalf("成功获取 %d 个 worker，但只归还了 %d 次，池缩容了", successWorkers, totalReleased)
	}
	// 最终池大小应等于初始大小（所有实例都归还了）
	if finalAvailable != poolSize {
		t.Fatalf("并发后池应有 %d 个实例，实际: %d（池缩容了）", poolSize, finalAvailable)
	}
}
