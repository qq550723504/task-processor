package referralregistration

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"task-processor/internal/referral"
	"testing"
	"time"
)

type firstStore struct {
	referral.Store
	calls     int
	failure   error
	intent    referral.Intent
	ipFailure error
}

func (s *firstStore) AllowIP(context.Context, string, time.Time) error { return s.ipFailure }

func (s *firstStore) Admit(_ context.Context, i referral.Intent, _ string) (referral.Intent, error) {
	s.calls++
	if s.failure != nil {
		return referral.Intent{}, s.failure
	}
	if s.intent.ID == "" {
		s.intent = i
	}
	if s.intent.Fingerprint != i.Fingerprint {
		return referral.Intent{}, referral.ErrConflict
	}
	return s.intent, nil
}
func (s *firstStore) Find(context.Context, string, string, string) (referral.Intent, error) {
	if s.intent.ID == "" {
		return referral.Intent{}, referral.ErrMissing
	}
	return s.intent, nil
}
func (s *firstStore) Claim(context.Context, string, time.Time) (bool, error) { return false, nil }
func (s *firstStore) Cleanup(context.Context, time.Time) error               { return nil }

type firstProvider struct{ creates int }

func (p *firstProvider) Create(context.Context, Creation) error { p.creates++; return nil }
func (p *firstProvider) Read(context.Context, string) (User, error) {
	return User{}, referral.ErrMissing
}
func firstService(st *firstStore, p *firstProvider) *Service {
	return &Service{Store: st, Provider: p, Issuer: "https://issuer.test", Instance: "instance", Organization: "signup", Keys: Keys{Active: "v1", Encryption: map[string][]byte{"v1": []byte("11111111111111111111111111111111")}, Proof: map[string][]byte{"v1": []byte("22222222222222222222222222222222")}, Lookup: []byte("33333333333333333333333333333333")}, Now: func() time.Time { return time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC) }}
}
func firstRequest() Request {
	return Request{Key: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Code: "valid-code", Email: "new@example.test", GivenName: "New", FamilyName: "User", ClientIP: "192.0.2.1"}
}

func TestIPAdmissionFailurePreventsDurableIntent(t *testing.T) {
	st := &firstStore{ipFailure: referral.ErrLimited}
	s := firstService(st, &firstProvider{})
	if _, err := s.Start(context.Background(), firstRequest()); !errors.Is(err, referral.ErrLimited) || st.calls != 0 {
		t.Fatalf("IP limit=%v admissions=%d", err, st.calls)
	}
}

func TestUnconfirmedIntentNeverCallsProvider(t *testing.T) {
	for _, failure := range []error{referral.ErrUnavailable, referral.ErrUnknown, context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			st, p := &firstStore{failure: failure}, &firstProvider{}
			_, err := firstService(st, p).Start(context.Background(), firstRequest())
			if !errors.Is(err, failure) || st.calls != 1 || p.creates != 0 {
				t.Fatalf("commit failure=%v error=%v durable attempts=%d provider creates=%d", failure, err, st.calls, p.creates)
			}
		})
	}
}
func TestAdmissionReplaySurvivesRestart(t *testing.T) {
	st, p := &firstStore{}, &firstProvider{}
	a, err := firstService(st, p).Start(context.Background(), firstRequest())
	if err != nil {
		t.Fatal(err)
	}
	b, err := firstService(st, p).Start(context.Background(), firstRequest())
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a.ResumeSecret == "" || a.Subject == "" {
		t.Fatal("must replay original encrypted admission receipt after restart")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(a.IntentID)
	if err != nil || len(decoded) != 32 {
		t.Fatal("intent ID must carry 256 random bits")
	}
	r := firstRequest()
	r.Code = "another"
	_, err = firstService(st, p).Start(context.Background(), r)
	if !errors.Is(err, referral.ErrConflict) {
		t.Fatalf("changed payload=%v", err)
	}
}

func TestRejectUnsupportedProviderInputBeforeAdmission(t *testing.T) {
	for _, change := range []func(*Request){
		func(r *Request) {
			r.Email = "new@" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + ".example.test"
		},
		func(r *Request) { r.GivenName = string([]byte{0xff}) },
	} {
		st, p := &firstStore{}, &firstProvider{}
		r := firstRequest()
		change(&r)
		if _, err := firstService(st, p).Start(context.Background(), r); !errors.Is(err, referral.ErrInvalid) || st.calls != 0 {
			t.Fatalf("unsupported input admitted: %v calls=%d", err, st.calls)
		}
	}
}

type blockingStore struct {
	*firstStore
	entered chan struct{}
	release chan struct{}
}

func (st *blockingStore) Admit(context.Context, referral.Intent, string) (referral.Intent, error) {
	st.entered <- struct{}{}
	<-st.release
	return referral.Intent{}, referral.ErrUnavailable
}
func TestInflightBoundRejectsWithoutQueue(t *testing.T) {
	st := &blockingStore{firstStore: &firstStore{}, entered: make(chan struct{}, 9), release: make(chan struct{})}
	s := firstService(st.firstStore, &firstProvider{})
	s.Store = st
	results := make(chan error, 9)
	for n := 0; n < 8; n++ {
		go func() { _, err := s.Start(context.Background(), firstRequest()); results <- err }()
	}
	for n := 0; n < 8; n++ {
		select {
		case <-st.entered:
		case <-time.After(time.Second):
			close(st.release)
			t.Fatal("first eight requests did not enter")
		}
	}
	go func() { _, err := s.Start(context.Background(), firstRequest()); results <- err }()
	select {
	case err := <-results:
		if !errors.Is(err, referral.ErrLimited) {
			t.Errorf("ninth request=%v", err)
		}
	case <-time.After(time.Second):
		t.Error("ninth request queued instead of rejected")
	}
	close(st.release)
}
