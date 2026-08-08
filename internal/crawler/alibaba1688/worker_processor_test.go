package alibaba1688

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/crawler/shared"
	"task-processor/internal/infra/worker"
)

func TestCrawler1688ProcessorUsesAccountBoundProfile(t *testing.T) {
	resolver := &fakeAccountProfileResolver{profile: AccountProfile{ID: 3001, TenantID: 101, ProfileDir: "C:/profiles/101/3001", ProxyServer: "http://proxy:8080"}}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 3001))

	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if resolver.accountID != 3001 {
		t.Fatalf("resolver account id = %d, want 3001", resolver.accountID)
	}
	if processor.profile == nil || processor.profile.ProfileDir != "C:/profiles/101/3001" || processor.profile.ProxyServer != "http://proxy:8080" {
		t.Fatalf("profile-aware processor received %+v", processor.profile)
	}
	if processor.globalCalled {
		t.Fatal("account-bound task used the global processor path")
	}
}

func TestCrawler1688ProcessorFailsClosedWithoutResolverForAccountBoundTask(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if processor.called() {
		t.Fatal("account-bound task fell back to the global processor path")
	}
}

func TestCrawler1688ProcessorDoesNotInventTenantForTenantScopedResolver(t *testing.T) {
	resolver := &tenantScopedOnlyAccountProfileResolver{}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if resolver.called {
		t.Fatal("worker attempted tenant-scoped resolution without trusted tenant context")
	}
	if processor.called() {
		t.Fatal("account-bound task used a processor without trusted account resolution")
	}
}

func TestCrawler1688ProcessorStopsBeforeProcessingWhenAccountResolutionFails(t *testing.T) {
	resolver := &fakeAccountProfileResolver{err: errors.New("repository unavailable")}
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, resolver)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 3001))

	if AccountProfileErrorCode(err) != AccountProfileUnavailable {
		t.Fatalf("account profile error code = %q, want %q", AccountProfileErrorCode(err), AccountProfileUnavailable)
	}
	if processor.called() {
		t.Fatal("processor was called after account resolution failed")
	}
}

func TestCrawler1688ProcessorUsesGlobalFallbackWithoutAccountID(t *testing.T) {
	processor := &fakeAlibaba1688TaskProcessor{}
	service := newTestAlibaba1688Service(processor, nil)

	err := (&Crawler1688Processor{service: service}).ProcessTask(context.Background(), crawler1688WorkerJob(t, 0))

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
	lowerPayload := strings.ToLower(string(payload))
	for _, forbidden := range []string{"password", "cookie", "proxy"} {
		if strings.Contains(lowerPayload, forbidden) {
			t.Fatalf("crawler task JSON contains forbidden %q field", forbidden)
		}
	}
}

func crawler1688WorkerJob(t *testing.T, accountID int64) worker.WorkerJob {
	t.Helper()
	task := shared.NewCrawlerTask("https://detail.1688.com/offer/3001.html")
	task.TaskID = "task-3001"
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
	accountID int64
}

type tenantScopedOnlyAccountProfileResolver struct {
	called bool
}

func (r *tenantScopedOnlyAccountProfileResolver) ResolveAlibaba1688Account(context.Context, int64, int64) (AccountProfile, error) {
	r.called = true
	return AccountProfile{}, errors.New("unexpected tenant-scoped resolution")
}

func (r *fakeAccountProfileResolver) ResolveAlibaba1688Account(context.Context, int64, int64) (AccountProfile, error) {
	return AccountProfile{}, errors.New("tenant-scoped resolver must not be used without trusted tenant context")
}

func (r *fakeAccountProfileResolver) ResolveAlibaba1688AccountByUniqueID(_ context.Context, accountID int64) (AccountProfile, error) {
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
