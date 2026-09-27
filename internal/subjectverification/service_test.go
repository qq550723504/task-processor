package subjectverification

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryStore struct {
	sync.Mutex
	apps     map[string]Application
	messages map[string]string
}

func (m *memoryStore) Reserve(_ context.Context, a Application) (Application, bool, error) {
	m.Lock()
	defer m.Unlock()
	if old, ok := m.apps[a.OrganizationID]; ok {
		return old, false, nil
	}
	m.apps[a.OrganizationID] = a
	return a, true, nil
}
func (m *memoryStore) Read(_ context.Context, org string) (Application, error) {
	m.Lock()
	defer m.Unlock()
	a, ok := m.apps[org]
	if !ok {
		return a, ErrNotFound
	}
	return a, nil
}
func (m *memoryStore) SaveLink(_ context.Context, id string, b []byte, e time.Time) error {
	m.Lock()
	defer m.Unlock()
	for org, a := range m.apps {
		if a.ID == id && a.State == Unknown {
			a.EncryptedURL = b
			a.ExpiresAt = e
			a.State = Pending
			m.apps[org] = a
		}
	}
	return nil
}
func (m *memoryStore) Apply(_ context.Context, r Receipt, f func(*Application) string) error {
	m.Lock()
	defer m.Unlock()
	key := r.Scope + ":" + r.MessageID
	if d, ok := m.messages[key]; ok {
		if d != r.Digest {
			return ErrConflict
		}
		return nil
	}
	for org, a := range m.apps {
		if a.Scope == r.Scope && a.Correlation == r.Correlation {
			f(&a)
			m.apps[org] = a
			m.messages[key] = r.Digest
			return nil
		}
	}
	return ErrNotFound
}

type providerFunc func(context.Context, CreateRequest) (Link, error)

func (f providerFunc) Create(c context.Context, r CreateRequest) (Link, error) { return f(c, r) }

type testProtection struct{}

func (testProtection) Digest(p, v string) string        { return p + ":" + v }
func (testProtection) Seal(a, v string) ([]byte, error) { return []byte(a + "|" + v), nil }
func (testProtection) Open(a string, b []byte) (string, error) {
	prefix := a + "|"
	if len(b) < len(prefix) || string(b[:len(prefix)]) != prefix {
		return "", ErrInvalid
	}
	return string(b[len(prefix):]), nil
}

var testNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
var testActor = Actor{"org-1", "user-1", "+8613800000001"}
var testInput = Input{"测试企业", "91310000MA00000001", "", true, "11111111-1111-4111-8111-111111111111"}

func newTestService(p Provider) *Service {
	return &Service{Store: &memoryStore{apps: map[string]Application{}, messages: map[string]string{}}, Provider: p, Protection: testProtection{}, Scope: "fixture-app", Now: func() time.Time { return testNow }}
}
func readyLink() (Link, error) {
	return Link{"https://qian.tencent.cn/verify?code=fixture", testNow.Add(24 * time.Hour)}, nil
}

func TestEmptyCallbackCorrelationStillChecksKnownMessage(t *testing.T) {
	s := newTestService(providerFunc(func(context.Context, CreateRequest) (Link, error) { return readyLink() }))
	s.Store.(*memoryStore).messages["fixture-app:known-message"] = "original-body"
	event := Event{MessageID: "known-message", Digest: "changed-body"}
	if err := s.Observe(context.Background(), event); !errors.Is(err, ErrConflict) {
		t.Fatalf("known message with empty correlation: %v", err)
	}
	event.MessageID = "unrelated-message"
	if err := s.Observe(context.Background(), event); !errors.Is(err, ErrNotFound) {
		t.Fatalf("new uncorrelated callback: %v", err)
	}
}

func TestConcurrentSameApplicationCreatesOneProviderLink(t *testing.T) {
	var calls atomic.Int32
	s := newTestService(providerFunc(func(context.Context, CreateRequest) (Link, error) { calls.Add(1); return readyLink() }))
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := s.Start(context.Background(), testActor, testInput)
			if err != nil || a.ID == "" {
				t.Errorf("start=%+v err=%v", a, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("provider creates=%d, want 1", calls.Load())
	}
	changed := testInput
	changed.CompanyName = "另一企业"
	if _, err := s.Start(context.Background(), testActor, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("different payload=%v", err)
	}
	other := testActor
	other.UserID = "other-user"
	if _, err := s.Start(context.Background(), other, testInput); !errors.Is(err, ErrConflict) {
		t.Fatalf("different actor=%v", err)
	}
}
func TestUnknownCreationIsDurableAndNeverRetried(t *testing.T) {
	var calls int
	s := newTestService(providerFunc(func(context.Context, CreateRequest) (Link, error) { calls++; return Link{}, context.DeadlineExceeded }))
	a, err := s.Start(context.Background(), testActor, testInput)
	if err != nil || a.State != Unknown {
		t.Fatalf("start=%+v err=%v", a, err)
	}
	restarted := *s
	if _, err = restarted.Start(context.Background(), testActor, testInput); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("restarted created %d times", calls)
	}
}
func TestCallbackIdentityAndResponseRace(t *testing.T) {
	var s *Service
	s = newTestService(providerFunc(func(ctx context.Context, r CreateRequest) (Link, error) {
		for i, phone := range []string{"138****0001", "13800000002", "13800000001"} {
			e := Event{MessageID: fmt.Sprint(i), Digest: fmt.Sprint(i), Correlation: r.Correlation, CompanyName: r.CompanyName, CreditCode: r.CreditCode, Phone: phone, ProviderOrganizationID: "tencent-org", ProviderAdminID: "tencent-admin", VerifiedAt: testNow}
			if err := s.Observe(ctx, e); err != nil {
				t.Fatal(err)
			}
			a, _ := s.Store.Read(ctx, testActor.OrganizationID)
			if i < 2 && a.State == Verified {
				t.Fatal("wrong or masked phone certified")
			}
		}
		return readyLink()
	}))
	a, err := s.Start(context.Background(), testActor, testInput)
	if err != nil || a.State != Verified || len(a.EncryptedURL) != 0 {
		t.Fatalf("callback must survive create response: %+v / %v", a, err)
	}
	e := Event{MessageID: "2", Digest: "changed", Correlation: a.Correlation}
	if err = s.Observe(context.Background(), e); !errors.Is(err, ErrConflict) {
		t.Fatalf("same message different payload=%v", err)
	}
}
func TestReadDoesNotLeakLinkAcrossActorOrOrganization(t *testing.T) {
	s := newTestService(providerFunc(func(context.Context, CreateRequest) (Link, error) { return readyLink() }))
	if _, err := s.Start(context.Background(), testActor, testInput); err != nil {
		t.Fatal(err)
	}
	_, link, err := s.Read(context.Background(), testActor)
	if err != nil || link == "" {
		t.Fatalf("owner read %q %v", link, err)
	}
	other := testActor
	other.UserID = "other-admin"
	_, link, err = s.Read(context.Background(), other)
	if err != nil || link != "" {
		t.Fatalf("other admin link %q %v", link, err)
	}
	other.OrganizationID = "other-org"
	if _, _, err = s.Read(context.Background(), other); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross org=%v", err)
	}
}
