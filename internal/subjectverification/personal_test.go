package subjectverification

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
	"time"
)

type personalTestStore struct {
	a       PersonalApplication
	creates int
	now     time.Time
}

func (r *personalTestStore) ReservePersonal(_ context.Context, a PersonalApplication, _ PersonalLimits) (PersonalApplication, bool, error) {
	if r.a.ID != "" {
		if r.a.IdempotencyKey != a.IdempotencyKey || r.a.InputDigest != a.InputDigest {
			return r.a, false, ErrConflict
		}
		return r.a, false, nil
	}
	a.CreatedAt = r.now
	a.ExpiresAt = r.now.Add(35 * time.Minute)
	r.a = a
	r.creates++
	return a, true, nil
}
func (r *personalTestStore) ReadPersonal(_ context.Context, _ string, l PersonalLimits) (PersonalSnapshot, error) {
	return PersonalSnapshot{Application: r.a, Quota: PersonalQuota{TotalLimit: l.Total, DailyLimit: l.Daily, TotalUsed: r.creates, DailyUsed: r.creates, ServerTime: r.now, ResetAt: r.now.Add(time.Hour)}}, nil
}
func (r *personalTestStore) UpdatePersonal(_ context.Context, user, id string, fn func(*PersonalApplication, time.Time) error) error {
	if r.a.ID != id || r.a.UserID != user {
		return ErrConflict
	}
	return fn(&r.a, r.now)
}

type personalTestProvider struct {
	match        bool
	phoneErr     error
	createErr    error
	calls        []string
	input        PersonalIdentity
	passed       string
	queried      string
	queriedScene int64
}

func (p *personalTestProvider) VerifyPhone(_ context.Context, in PersonalIdentity) (bool, error) {
	p.calls = append(p.calls, "phone")
	p.input = in
	return p.match, p.phoneErr
}
func (p *personalTestProvider) CreateFace(_ context.Context, id string, scene int64, in PersonalIdentity, meta string) (PersonalLink, error) {
	p.calls = append(p.calls, "face")
	if in != p.input {
		panic("identity changed")
	}
	return PersonalLink{CertifyID: "provider-certify", URL: "https://t.aliyun.com/example"}, p.createErr
}
func (p *personalTestProvider) QueryFace(_ context.Context, scene int64, id string) (PersonalResult, error) {
	p.queried = id
	p.queriedScene = scene
	return PersonalResult{Passed: p.passed}, nil
}
func personalFixture() (*PersonalService, *personalTestStore, *personalTestProvider, Actor, PersonalInput) {
	r := &personalTestStore{now: time.Now().UTC()}
	p := &personalTestProvider{match: true, passed: "T"}
	s := &PersonalService{Store: r, Provider: p, Protection: testProtection{}, Scope: "ali-scene", SceneID: 123, Limits: DefaultPersonalLimits()}
	return s, r, p, Actor{UserID: "u1", VerifiedPhone: "+8613800000001"}, PersonalInput{Name: "测试姓名", IDNumber: "110101199001010010", MetaInfo: `{"deviceType":"pc"}`, Consent: true, IdempotencyKey: uuid.NewString()}
}
func TestPersonalUnknownAndReplayDoNotRepeatProvider(t *testing.T) {
	s, r, p, a, in := personalFixture()
	p.createErr = errors.New("response lost")
	if err := s.Start(context.Background(), a, in); err != nil {
		t.Fatal(err)
	}
	in.MetaInfo = `{"deviceType":"changed"}`
	if err := s.Start(context.Background(), a, in); err != nil {
		t.Fatal(err)
	}
	if r.creates != 1 || len(p.calls) != 2 || r.a.State != Unknown {
		t.Fatalf("repeated side effect: %+v %v", r, p.calls)
	}
	in.Name = "其他人"
	if !errors.Is(s.Start(context.Background(), a, in), ErrConflict) {
		t.Fatal("changed identity replay accepted")
	}
}
func TestPersonalPhoneFailureNeverStartsFace(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		s, r, p, a, in := personalFixture()
		p.match = false
		if uncertain {
			p.phoneErr = errors.New("timeout")
		}
		_ = s.Start(context.Background(), a, in)
		want := Rejected
		if uncertain {
			want = Unknown
		}
		if len(p.calls) != 1 || r.a.State != want || r.creates != 1 {
			t.Fatal("phone failure did not fail closed")
		}
	}
}
func TestPersonalOnlyStoredProviderIdentityCanBeConfirmed(t *testing.T) {
	s, r, p, a, in := personalFixture()
	ctx := context.Background()
	if err := s.Start(ctx, a, in); err != nil {
		t.Fatal(err)
	}
	if r.a.State != Pending {
		t.Fatal(r.a.State)
	}
	other := a
	other.UserID = "u2"
	if s.Refresh(ctx, other, r.a.ID) == nil {
		t.Fatal("cross user accepted")
	}
	other = a
	other.VerifiedPhone = "+8613900000001"
	if s.Refresh(ctx, other, r.a.ID) == nil {
		t.Fatal("changed phone accepted")
	}
	if s.Refresh(ctx, a, "arbitrary-provider-id") == nil {
		t.Fatal("client provider id accepted")
	}
	s.SceneID = 456 // Configuration changes must not replace the application's scene.
	if err := s.Refresh(ctx, a, r.a.ID); err != nil {
		t.Fatal(err)
	}
	if r.a.State != Verified || p.queried != "provider-certify" || p.queriedScene != 123 || len(r.a.EncryptedURL) != 0 {
		t.Fatal("binding not confirmed")
	}
}
func TestPersonalLifetimeLimitSurvivesDayReset(t *testing.T) {
	q := PersonalQuota{TotalLimit: 5, TotalUsed: 5, DailyLimit: 3, DailyUsed: 0, ServerTime: time.Now()}
	var limit *PersonalLimitError
	if !errors.As(CheckPersonalAdmission(PersonalSnapshot{Quota: q}), &limit) || limit.Code != "VERIFICATION_TOTAL_LIMIT" {
		t.Fatal("lifetime limit lost at midnight")
	}
	q.TotalUsed = 4
	q.DailyUsed = 3
	if !errors.As(CheckPersonalAdmission(PersonalSnapshot{Quota: q}), &limit) || limit.Code != "VERIFICATION_DAILY_LIMIT" {
		t.Fatal("daily limit missing")
	}
	q.DailyUsed = 0
	q.NextAllowedAt = q.ServerTime.Add(time.Second)
	if !errors.As(CheckPersonalAdmission(PersonalSnapshot{Quota: q}), &limit) || limit.Code != "VERIFICATION_COOLDOWN" {
		t.Fatal("cross-day cooldown missing")
	}
}
