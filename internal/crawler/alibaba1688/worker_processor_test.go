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
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
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
	if !processor.globalCalled {
		t.Fatal("account-bound task did not attempt the public processor path")
	}
}

func TestCrawler1688ProcessorSerializesSameAccountProfile(t *testing.T) {
	processor := &blockingProfileProcessor{
		started:   make(chan struct{}, 2),
		release:   make(chan struct{}),
		globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha")),
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
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
	service := newTestAlibaba1688Service(processor, nil)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if !processor.globalCalled {
		t.Fatal("account-bound task did not attempt the public processor path before resolver failure")
	}
}

func TestCrawler1688ProcessorDoesNotCallResolverWithoutTrustedTenant(t *testing.T) {
	resolver := &fakeAccountProfileResolver{}
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 0, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if resolver.called {
		t.Fatal("worker called the resolver without trusted tenant context")
	}
	if !processor.globalCalled {
		t.Fatal("account-bound task did not attempt public access")
	}
}

func TestCrawler1688ProcessorRejectsNegativeAccountIDBeforeResolverAndProcessor(t *testing.T) {
	resolver := &fakeAccountProfileResolver{}
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
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
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if !resolver.called || resolver.tenantID != 101 || resolver.accountID != 3001 {
		t.Fatalf("resolver = called %t tenant %d account %d, want called tenant 101 account 3001", resolver.called, resolver.tenantID, resolver.accountID)
	}
	if !processor.globalCalled {
		t.Fatal("public processor was not called before account resolution")
	}
	result, getErr := service.GetTask("task-3001")
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if result.SourceAccessMode != string(sourceAccessModeAccountAssisted) || result.SourceFallbackReason != "public_challenge" {
		t.Fatalf("failed result source metadata = (%q, %q), want account-assisted/public_challenge", result.SourceAccessMode, result.SourceFallbackReason)
	}
}

func TestCrawler1688ProcessorPreservesDisabledAccountError(t *testing.T) {
	resolver := &fakeAccountProfileResolver{err: newAccountProfileError(AccountProfileDisabled, "resolver says disabled")}
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))

	if AccountProfileErrorCode(err) != AccountProfileDisabled {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileDisabled)
	}
	if err.Error() != "source account is disabled" {
		t.Fatalf("error = %q, want stable disabled message", err)
	}
	if !processor.globalCalled {
		t.Fatal("public processor was not called before disabled account resolution")
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

func TestCrawler1688ProcessorPassesContextToPublicProcessor(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)
	ctx := context.WithValue(context.Background(), struct{}{}, "public")

	if err := (&Crawler1688Processor{service: service}).ProcessTask(ctx, crawler1688WorkerJob(t, 101, 0)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.publicCtx != ctx {
		t.Fatal("public processor did not receive the exact worker context")
	}
}

func TestCrawler1688ProcessorPassesContextToAccountProcessor(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{
		globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha")),
	}
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001"}}
	service := newTestAlibaba1688Service(processor, resolver)
	ctx := context.WithValue(context.Background(), struct{}{}, "account")

	if err := (&Crawler1688Processor{service: service}).ProcessTask(ctx, crawler1688WorkerJob(t, 101, 3001)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if processor.accountCtx != ctx {
		t.Fatal("account processor did not receive the exact worker context")
	}
}

func TestCrawler1688ProcessorRecordsPublicAccessMode(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)

	if err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 0)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	result, err := service.GetTask("task-3001")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if result.SourceAccessMode != "public" || result.SourceFallbackReason != "" {
		t.Fatalf("source metadata = (%q, %q), want public/no fallback", result.SourceAccessMode, result.SourceFallbackReason)
	}
}

func TestCrawler1688ProcessorFallsBackToAccountOnlyAfterRecoverablePublicFailure(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha"))}
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001"}}
	service := newTestAlibaba1688Service(processor, resolver)

	if err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001)); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if !processor.globalCalled || processor.profile == nil {
		t.Fatalf("processor calls = global:%t profile:%v, want public then account", processor.globalCalled, processor.profile)
	}
	result, err := service.GetTask("task-3001")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if result.SourceAccessMode != "account_assisted" || result.SourceFallbackReason != "public_challenge" {
		t.Fatalf("source metadata = (%q, %q), want account-assisted/public_challenge", result.SourceAccessMode, result.SourceFallbackReason)
	}
}

func TestCrawler1688ProcessorPreservesSourceMetadataWhenAccountCrawlFails(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{
		globalErr:  NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha")),
		profileErr: errors.New("account crawl failed"),
	}
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001"}}
	service := newTestAlibaba1688Service(processor, resolver)

	if err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001)); err == nil {
		t.Fatal("ProcessTask() error = nil, want account crawl failure")
	}
	result, getErr := service.GetTask("task-3001")
	if getErr != nil {
		t.Fatalf("GetTask() error = %v", getErr)
	}
	if result.SourceAccessMode != string(sourceAccessModeAccountAssisted) || result.SourceFallbackReason != "public_challenge" {
		t.Fatalf("failed result source metadata = (%q, %q), want account-assisted/public_challenge", result.SourceAccessMode, result.SourceFallbackReason)
	}
	if got := service.sourceAccessStats()[string(sourceAccessModeAccountAssisted)]; got != 1 {
		t.Fatalf("account-assisted access count = %d, want 1 after failed account crawl", got)
	}
}

func TestCrawler1688ProcessorDoesNotResolveAccountForNonRecoverablePublicFailure(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{globalErr: NewPublicAccessError(PublicAccessFailureInvalidURL, errors.New("bad url"))}
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001"}}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 101, 3001))
	if err == nil || AccountProfileErrorCode(err) != "source_public_unavailable" {
		t.Fatalf("ProcessTask() error = %v, want source_public_unavailable", err)
	}
	if resolver.called {
		t.Fatal("resolver was called for a non-recoverable public failure")
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
	result := shared.NewCrawlerResult("task-3001")
	result.TenantID = 101
	if err := service.StoreResult("task-3001", result); err != nil {
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
	publicCtx    context.Context
	accountCtx   context.Context
	globalErr    error
	profileErr   error
}

func (p *fakeAlibaba1688TaskProcessor) Process(ctx context.Context, _ string) (*model.Product1688, error) {
	p.globalCalled = true
	p.publicCtx = ctx
	return &model.Product1688{}, p.globalErr
}

func (p *fakeAlibaba1688TaskProcessor) ProcessWithAccountProfile(ctx context.Context, _ string, profile AccountProfile) (*model.Product1688, error) {
	p.profile = &profile
	p.accountCtx = ctx
	return &model.Product1688{}, p.profileErr
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
	globalErr error
}

func (p *blockingProfileProcessor) Process(context.Context, string) (*model.Product1688, error) {
	return &model.Product1688{}, p.globalErr
}

func (p *blockingProfileProcessor) ProcessWithAccountProfile(context.Context, string, AccountProfile) (*model.Product1688, error) {
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
