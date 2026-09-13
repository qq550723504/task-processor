package referralregistration

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/referral"
)

type continuationStore struct {
	*firstStore
	receipt     referral.Receipt
	consumed    int
	selfSubject string
}

func (st *continuationStore) Read(_ context.Context, _ string, subject string) (referral.Projection, error) {
	st.selfSubject = subject
	return referral.Projection{}, nil
}
func (st *continuationStore) CreateCode(_ context.Context, _ string, subject, code string) (string, error) {
	st.selfSubject = subject
	return code, nil
}

func (st *continuationStore) Claim(context.Context, string, time.Time) (bool, error) {
	return true, nil
}
func (st *continuationStore) Created(context.Context, string) error {
	st.intent.State = "CREATED"
	return nil
}
func (st *continuationStore) Receipt(context.Context, string, string) (referral.Receipt, error) {
	if st.receipt.IntentID == "" {
		return referral.Receipt{}, referral.ErrMissing
	}
	return st.receipt, nil
}
func (st *continuationStore) Consume(_ context.Context, i referral.Intent, now time.Time) (referral.Receipt, error) {
	st.consumed++
	st.receipt = referral.Receipt{IntentID: i.ID, Issuer: i.Issuer, Subject: i.Subject, Referrer: i.Referrer, Fingerprint: i.Fingerprint, BoundAt: now}
	return st.receipt, nil
}

type continuationProvider struct {
	user                   User
	creates, reads         int
	createError, readError error
}

func (p *continuationProvider) Create(_ context.Context, c Creation) error {
	p.creates++
	p.user = User{Subject: c.Subject, Organization: c.Organization, Email: c.Email, Proof: c.Proof}
	return p.createError
}
func (p *continuationProvider) Read(context.Context, string) (User, error) {
	p.reads++
	if p.readError != nil {
		return User{}, p.readError
	}
	if p.user.Subject == "" {
		return User{}, referral.ErrMissing
	}
	return p.user, nil
}
func continuation(t *testing.T) (*Service, *continuationStore, *continuationProvider, Admission) {
	t.Helper()
	st := &continuationStore{firstStore: &firstStore{}}
	p := &continuationProvider{}
	s := firstService(st.firstStore, &firstProvider{})
	s.Store = st
	s.Provider = p
	a, err := s.Start(context.Background(), firstRequest())
	if err != nil {
		t.Fatal(err)
	}
	return s, st, p, a
}
func authenticated(s *Service, subject string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: subject, TokenExpiresAt: s.Now().Add(time.Hour)})
}
func TestLostCreateResponseUsesFixedSubject(t *testing.T) {
	s, st, p, a := continuation(t)
	p.createError = referral.ErrUnknown
	if err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret); err != nil {
		t.Fatal(err)
	}
	if err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret); err != nil {
		t.Fatal(err)
	}
	if p.creates != 1 || p.user.Subject != a.Subject || st.intent.State != "CREATED" {
		t.Fatal("lost response must recover original subject without another create")
	}
}
func TestExpiredCreateOnlyReadsFixedSubject(t *testing.T) {
	s, _, p, a := continuation(t)
	s.Now = func() time.Time { return a.CreateExpiresAt }
	if err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret); !errors.Is(err, referral.ErrExpired) {
		t.Fatalf("got %v", err)
	}
	if p.creates != 0 || p.reads != 1 {
		t.Fatal("expiry permits readback only")
	}
}
func TestSuccessfulReceiptPrecedesProviderAndExpiry(t *testing.T) {
	s, st, p, a := continuation(t)
	if err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret); err != nil {
		t.Fatal(err)
	}
	p.user.Verified = true
	want, err := s.Complete(authenticated(s, a.Subject))
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return a.CompletionExpiresAt.Add(time.Hour) }
	p.readError = referral.ErrUnavailable
	reads := p.reads
	got, err := s.Complete(authenticated(s, a.Subject))
	if err != nil || got != want || p.reads != reads || st.consumed != 1 {
		t.Fatalf("receipt replay got=%+v err=%v reads=%d", got, err, p.reads)
	}
	if _, err = s.Complete(context.Background()); !errors.Is(err, referral.ErrUnauthenticated) {
		t.Fatalf("anonymous receipt replay=%v", err)
	}
}

func TestUnverifiedOrMismatchedIdentityNeverConsumes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Service, *continuationStore, *continuationProvider)
	}{
		{"unverified", func(*Service, *continuationStore, *continuationProvider) {}},
		{"wrong email", func(_ *Service, _ *continuationStore, p *continuationProvider) {
			p.user.Verified = true
			p.user.Email = "other@example.test"
		}},
		{"copied proof", func(_ *Service, _ *continuationStore, p *continuationProvider) {
			p.user.Verified = true
			p.user.Proof = "v1:copied"
		}},
		{"wrong provider subject", func(_ *Service, _ *continuationStore, p *continuationProvider) {
			p.user.Verified = true
			p.user.Subject = "another"
		}},
		{"wrong issuer", func(_ *Service, st *continuationStore, p *continuationProvider) {
			p.user.Verified = true
			st.intent.Issuer = "other"
		}},
		{"wrong signup organization", func(_ *Service, _ *continuationStore, p *continuationProvider) {
			p.user.Verified = true
			p.user.Organization = "other"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, p, a := continuation(t)
			if err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret); err != nil {
				t.Fatal(err)
			}
			tc.change(s, st, p)
			if _, err := s.Complete(authenticated(s, a.Subject)); err == nil || st.consumed != 0 {
				t.Fatalf("invalid identity consumed=%d err=%v", st.consumed, err)
			}
		})
	}
}

func TestResumeRejectsTamperingAndKeepsOriginalKeys(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(*Service, *continuationStore, *Admission)
		wantSuccess bool
	}{
		{"wrong secret", func(_ *Service, _ *continuationStore, a *Admission) { a.ResumeSecret = "wrong" }, false},
		{"tampered ciphertext", func(_ *Service, st *continuationStore, _ *Admission) { st.intent.Ciphertext[15] ^= 1 }, false},
		{"missing old key", func(s *Service, _ *continuationStore, _ *Admission) { delete(s.Keys.Proof, "v1") }, false},
		{"rotation", func(s *Service, _ *continuationStore, _ *Admission) {
			s.Keys.Active = "v2"
			s.Keys.Encryption["v2"] = []byte("44444444444444444444444444444444")
			s.Keys.Proof["v2"] = []byte("55555555555555555555555555555555")
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, p, a := continuation(t)
			tc.change(s, st, &a)
			err := s.Resume(context.Background(), a.IntentID, a.ResumeSecret)
			if (err == nil) != tc.wantSuccess {
				t.Fatalf("resume=%v", err)
			}
			if !tc.wantSuccess && p.creates != 0 {
				t.Fatal("invalid recovery dispatched")
			}
			if tc.wantSuccess && p.user.Proof[:3] != "v1:" {
				t.Fatal("rotation changed original proof key")
			}
		})
	}
}

func TestSelfCommandsUseCurrentPersonWithoutOrganization(t *testing.T) {
	s, st, _, _ := continuation(t)
	ctx := authenticated(s, "current-person")
	if _, err := s.ReadSelf(ctx); err != nil || st.selfSubject != "current-person" {
		t.Fatalf("self read=%v subject=%s", err, st.selfSubject)
	}
	if _, err := s.CreateSelfCode(ctx); err != nil || st.selfSubject != "current-person" {
		t.Fatalf("self code=%v", err)
	}
	if _, err := s.ReadSelf(context.Background()); !errors.Is(err, referral.ErrUnauthenticated) {
		t.Fatalf("anonymous read=%v", err)
	}
	if _, err := s.CreateSelfCode(context.Background()); !errors.Is(err, referral.ErrUnauthenticated) {
		t.Fatalf("anonymous code=%v", err)
	}
}
