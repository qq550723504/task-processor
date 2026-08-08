package alibaba1688

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

func TestCrawler1688ProcessorUsesTrustedTenantAccountProfile(t *testing.T) {
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001", ProxyServer: "http://proxy:8080"}}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if resolver.tenantID != 101 || resolver.accountID != 3001 {
		t.Fatalf("resolver = tenant %d account %d, want tenant 101 account 3001", resolver.tenantID, resolver.accountID)
	}
	if processor.profile == nil || processor.profile.ProfileDir != "C:/profiles/101/3001" || processor.profile.ProxyServer != "http://proxy:8080" {
		t.Fatalf("profile-aware processor received %+v", processor.profile)
	}
	if processor.globalCalled {
		t.Fatal("account-bound task used the global processor path")
	}
}

func TestCrawler1688ProcessorSerializesSameAccountProfile(t *testing.T) {
	processor := &blockingProfileProcessor{
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	resolver := &staticAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001"}}
	service := newTestAlibaba1688Service(processor, resolver)
	p := &Crawler1688Processor{service: service}

	firstDone := make(chan error, 1)
	go func() { firstDone <- p.ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001)) }()
	select {
	case <-processor.started:
	case <-time.After(time.Second):
		t.Fatal("first profile crawl did not start")
	}

	secondDone := make(chan error, 1)
	go func() { secondDone <- p.ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001)) }()
	select {
	case <-processor.started:
		t.Fatal("same profile started concurrently")
	case <-time.After(50 * time.Millisecond):
	}
	close(processor.release)

	if err := <-firstDone; err != nil {
		t.Fatalf("first ProcessTask() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second ProcessTask() error = %v", err)
	}
	if processor.maxActive != 1 {
		t.Fatalf("max concurrent profile crawls = %d, want 1", processor.maxActive)
	}
}

func TestCrawler1688ProcessorFailsClosedWithoutResolverForAccountBoundTask(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if processor.called() {
		t.Fatal("account-bound task fell back to the global processor path")
	}
}

func TestCrawler1688ProcessorDoesNotCallResolverWithoutTrustedTenant(t *testing.T) {
	resolver := &fakeAccountProfileResolver{}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 0, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if resolver.called {
		t.Fatal("worker called the resolver without trusted tenant context")
	}
	if processor.called() {
		t.Fatal("account-bound task used a processor without trusted account resolution")
	}
}

func TestCrawler1688ProcessorRejectsNegativeAccountIDBeforeResolverAndProcessor(t *testing.T) {
	resolver := &fakeAccountProfileResolver{}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, -1))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if resolver.called {
		t.Fatal("worker called resolver for negative source account id")
	}
	if processor.called() {
		t.Fatal("worker used a processor for negative source account id")
	}
}

func TestCrawler1688ProcessorStopsBeforeProcessingWhenAccountResolutionFails(t *testing.T) {
	resolver := &fakeAccountProfileResolver{err: errors.New("repository unavailable")}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if !resolver.called || resolver.tenantID != 101 || resolver.accountID != 3001 {
		t.Fatalf("resolver = called %t tenant %d account %d, want called tenant 101 account 3001", resolver.called, resolver.tenantID, resolver.accountID)
	}
	if processor.called() {
		t.Fatal("processor was called after account resolution failed")
	}
}

func TestCrawler1688ProcessorPreservesDisabledAccountError(t *testing.T) {
	resolver := &fakeAccountProfileResolver{err: newAccountProfileError(AccountProfileDisabled, "resolver says disabled")}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileDisabled {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileDisabled)
	}
	if err.Error() != "1688 account is disabled" {
		t.Fatalf("error = %q, want stable disabled message", err)
	}
	if processor.called() {
		t.Fatal("processor was called after disabled account resolution")
	}
}

func TestCrawler1688ProcessorUsesGlobalFallbackWithoutAccountID(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 0, 0))

	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if !processor.globalCalled || processor.profile != nil {
		t.Fatal("unbound task did not use the global processor path")
	}
}

func TestCrawler1688WorkerTaskJSONRetainsAccountIDWithoutSecrets(t *testing.T) {
	task := shared.NewCrawlerTask("https://detail.1688.com/offer/3001.html")
	task.SourceAccountID = 3001
	task.TenantID = 101

	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	var restored shared.CrawlerTask
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}
	if restored.SourceAccountID != 3001 {
		t.Fatalf("restored source account id = %d, want 3001", restored.SourceAccountID)
	}
	if restored.TenantID != 101 {
		t.Fatalf("restored tenant id = %d, want 101", restored.TenantID)
	}
	lowerPayload := strings.ToLower(string(payload))
	for _, forbidden := range []string{"password", "cookie", "proxy"} {
		if strings.Contains(lowerPayload, forbidden) {
			t.Fatalf("crawler task JSON contains forbidden %q field", forbidden)
		}
	}
}

func crawler1688WorkerJob(t *testing.T, tenantID, accountID int64) worker.WorkerJob {
	t.Helper()
	task := shared.NewCrawlerTask("https://detail.1688.com/offer/3001.html")
	task.TaskID = "task-3001"
	task.TenantID = tenantID
	task.SourceAccountID = accountID
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal worker task: %v", err)
	}
	return worker.WorkerJob{TaskID: 1, TaskData: string(payload)}
}

func newTestAlibaba1688Service(processor alibaba1688TaskProcessor, resolver AccountProfileResolver) *Service {
	service := &Service{processor1688: processor, accountProfileResolver: resolver}
	if err := service.StoreResult("task-3001", shared.NewCrawlerResult("task-3001")); err != nil {
		panic(err)
	}
	return service
}

type fakeAccountProfileResolver struct {
	profile   AccountProfile
	err       error
	tenantID  int64
	accountID int64
	called    bool
}

type staticAccountProfileResolver struct{ profile AccountProfile }

func (r *staticAccountProfileResolver) ResolveAlibaba1688Account(context.Context, int64, int64) (AccountProfile, error) {
	return r.profile, nil
}

func (r *fakeAccountProfileResolver) ResolveAlibaba1688Account(_ context.Context, tenantID, accountID int64) (AccountProfile, error) {
	r.called = true
	r.tenantID = tenantID
	r.accountID = accountID
	if r.err != nil {
		return AccountProfile{}, r.err
	}
	return r.profile, nil
}

type fakeAlibaba1688TaskProcessor struct {
	globalCalled bool
	profile      *AccountProfile
}

func (p *fakeAlibaba1688TaskProcessor) Process(string) (*model.Product1688, error) {
	p.globalCalled = true
	return &model.Product1688{}, nil
}

func (p *fakeAlibaba1688TaskProcessor) ProcessWithAccountProfile(_ string, profile AccountProfile) (*model.Product1688, error) {
	p.profile = &profile
	return &model.Product1688{}, nil
}

func (p *fakeAlibaba1688TaskProcessor) Shutdown() {}

func (p *fakeAlibaba1688TaskProcessor) called() bool {
	return p.globalCalled || p.profile != nil
}

type blockingProfileProcessor struct {
	mu        sync.Mutex
	active    int
	maxActive int
	started   chan struct{}
	release   chan struct{}
}

func (p *blockingProfileProcessor) Process(string) (*model.Product1688, error) {
	return &model.Product1688{}, nil
}

func (p *blockingProfileProcessor) ProcessWithAccountProfile(string, AccountProfile) (*model.Product1688, error) {
	p.mu.Lock()
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()
	p.started <- struct{}{}
	<-p.release
	p.mu.Lock()
	p.active--
	p.mu.Unlock()
	return &model.Product1688{}, nil
}

func (p *blockingProfileProcessor) Shutdown() {}
