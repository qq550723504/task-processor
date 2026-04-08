package amazon

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type memProductDedupeStore struct {
	mu      sync.Mutex
	values  map[string]string
	expires map[string]time.Time
}

func newMemProductDedupeStore() *memProductDedupeStore {
	return &memProductDedupeStore{
		values:  make(map[string]string),
		expires: make(map[string]time.Time),
	}
}

func (m *memProductDedupeStore) cleanupLocked(key string) {
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		delete(m.values, key)
		delete(m.expires, key)
	}
}

func (m *memProductDedupeStore) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(key)
	value, ok := m.values[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

func (m *memProductDedupeStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	m.expires[key] = time.Now().Add(ttl)
	return nil
}

func (m *memProductDedupeStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(key)
	if _, exists := m.values[key]; exists {
		return false, nil
	}
	m.values[key] = value
	m.expires[key] = time.Now().Add(ttl)
	return true, nil
}

func (m *memProductDedupeStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	delete(m.expires, key)
	return nil
}

func TestServiceFetchProductDeduplicatesConcurrentRequests(t *testing.T) {
	store := newMemProductDedupeStore()
	cfg := &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:      true,
			CrawlTimeout: 30,
			Zipcodes: map[string]string{
				"us": "10001",
			},
		},
	}

	service := &Service{
		config:         cfg,
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		dedupeStore:    store,
	}

	var mu sync.Mutex
	calls := 0
	service.processProduct = func(ctx context.Context, url, zipcode string) (*model.Product, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(150 * time.Millisecond)
		return &model.Product{
			Asin:  "B001234567",
			Title: "Demo Product",
			URL:   url,
		}, nil
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]*model.Product, 2)
	errs := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], _, errs[index] = service.FetchProduct(ctx, "", "B001234567", "us", "")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d returned error: %v", i, err)
		}
	}
	for i, product := range results {
		if product == nil || product.Asin != "B001234567" {
			t.Fatalf("request %d returned unexpected product: %+v", i, product)
		}
	}
	if calls != 1 {
		t.Fatalf("expected 1 real crawl, got %d", calls)
	}
}

func TestServiceFetchProductBlocksRegionWhenGuardIsOpen(t *testing.T) {
	cfg := &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:      true,
			CrawlTimeout: 30,
			RegionGuard: config.AmazonRegionGuardConfig{
				Enabled:                 true,
				FailureThreshold:        1,
				EvaluationWindowSeconds: 60,
				CooldownSeconds:         30,
			},
			Zipcodes: map[string]string{
				"us": "10001",
			},
		},
	}

	service := &Service{
		config:         cfg,
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		metrics:        newServiceMetrics(),
		regionGuard:    newRegionGuard(cfg.Amazon.RegionGuard),
	}
	service.processProduct = func(ctx context.Context, url, zipcode string) (*model.Product, error) {
		return nil, fmt.Errorf("captcha challenge detected")
	}

	if _, _, err := service.FetchProduct(context.Background(), "https://www.amazon.com/dp/B001234567", "", "us", ""); err == nil {
		t.Fatalf("expected initial captcha failure")
	}

	_, _, err := service.FetchProduct(context.Background(), "https://www.amazon.com/dp/B001234567", "", "us", "")
	if err == nil {
		t.Fatalf("expected region guard to block second request")
	}

	classified := ClassifyFetchError(err)
	if classified == nil || classified.ErrorType() != FetchErrorTypeRegionCircuitOpen {
		t.Fatalf("expected region_circuit_open, got %v", err)
	}
}

func TestServiceFetchProductReturnsSystemBusyWhenConcurrencyAcquireTimesOut(t *testing.T) {
	cfg := &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:      true,
			CrawlTimeout: 30,
			ConcurrencyControl: config.AmazonConcurrencyControlConfig{
				Enabled:               true,
				MaxInFlight:           1,
				MaxWaiting:            10,
				AcquireTimeoutSeconds: 1,
			},
			Zipcodes: map[string]string{
				"us": "10001",
			},
		},
	}

	service := &Service{
		config:         cfg,
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		metrics:        newServiceMetrics(),
		regionGuard:    newRegionGuard(cfg.Amazon.RegionGuard),
		concurrency:    newConcurrencyControl(cfg.Amazon.ConcurrencyControl),
	}

	blocker := make(chan struct{})
	service.processProduct = func(ctx context.Context, url, zipcode string) (*model.Product, error) {
		select {
		case <-blocker:
			return &model.Product{Asin: "B001", Title: "Demo Product", URL: url}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _, _ = service.FetchProduct(context.Background(), "https://www.amazon.com/dp/B001", "", "us", "")
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, _, err := service.FetchProduct(ctx, "https://www.amazon.com/dp/B002", "", "us", "")
	close(blocker)
	<-firstDone

	if err == nil {
		t.Fatal("expected system busy error")
	}

	classified := ClassifyFetchError(err)
	if classified == nil || classified.ErrorType() != FetchErrorTypeSystemBusy {
		t.Fatalf("expected system_busy, got %v", err)
	}
}

func TestServiceStatsIncludeConcurrencySnapshot(t *testing.T) {
	cfg := &config.Config{
		Amazon: config.AmazonConfig{
			Enabled:      true,
			CrawlTimeout: 30,
			ConcurrencyControl: config.AmazonConcurrencyControlConfig{
				Enabled:               true,
				MaxInFlight:           2,
				MaxWaiting:            5,
				AcquireTimeoutSeconds: 5,
				PerRegion: map[string]int{
					"us": 1,
				},
			},
		},
	}

	service := &Service{
		config:      cfg,
		logger:      logrus.New(),
		metrics:     newServiceMetrics(),
		concurrency: newConcurrencyControl(cfg.Amazon.ConcurrencyControl),
	}

	stats := service.GetStats()
	if _, ok := stats["concurrency_global_limit"]; !ok {
		t.Fatalf("expected concurrency stats in snapshot, got %#v", stats)
	}
	if _, ok := stats["concurrency_region_limit_by_region"]; !ok {
		t.Fatalf("expected region concurrency stats in snapshot, got %#v", stats)
	}
}

func TestServiceResolveFetchInputsDoesNotAutoFillUSZipcode(t *testing.T) {
	service := &Service{
		config:         &config.Config{},
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		processProduct: func(ctx context.Context, url, zipcode string) (*model.Product, error) { return nil, nil },
	}

	url, zipcode, err := service.resolveFetchInputs("", "B001234567", "us", "")
	if err != nil {
		t.Fatalf("resolve fetch inputs failed: %v", err)
	}
	if url == "" {
		t.Fatalf("expected resolved url")
	}
	if zipcode != "" {
		t.Fatalf("expected empty zipcode for us by default, got %q", zipcode)
	}
}

func TestServiceResolveFetchInputsKeepsExplicitZipcode(t *testing.T) {
	service := &Service{
		config:         &config.Config{},
		logger:         logrus.New(),
		domainResolver: NewDomainResolver(),
		processProduct: func(ctx context.Context, url, zipcode string) (*model.Product, error) { return nil, nil },
	}

	_, zipcode, err := service.resolveFetchInputs("", "B001234567", "us", "10001")
	if err != nil {
		t.Fatalf("resolve fetch inputs failed: %v", err)
	}
	if zipcode != "10001" {
		t.Fatalf("expected explicit zipcode to be preserved, got %q", zipcode)
	}
}
