//go:build integration

package subjectverification

import (
	"context"
	"errors"
	"github.com/google/uuid"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
	"sync/atomic"
	domain "task-processor/internal/subjectverification"
	"testing"
	"time"
)

func TestPersonalPostgresLifetimeAndConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("personal"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, _ := c.ConnectionString(ctx, "sslmode=disable")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx, _ := sqlDB.BeginTx(ctx, nil)
	if err = InstallPersonalSchemaTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	r, err := NewPersonalRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	limits := domain.DefaultPersonalLimits()
	application := func(user string) domain.PersonalApplication {
		return domain.PersonalApplication{ID: uuid.NewString(), UserID: user, Scope: "scope", ProviderSceneID: 123, IdempotencyKey: uuid.NewString(), InputDigest: "input", IdentityDigest: "identity", PhoneDigest: "phone", MaskedPhone: "138****0001", State: domain.Unknown}
	}
	a := application("same-key")
	var n atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, e := r.ReservePersonal(ctx, a, limits)
			if e != nil {
				t.Error(e)
			}
			if created {
				n.Add(1)
			}
		}()
	}
	wg.Wait()
	if n.Load() != 1 {
		t.Fatal(n.Load())
	}
	a.InputDigest = "other"
	if _, _, e := r.ReservePersonal(ctx, a, limits); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("different payload accepted")
	}
	n.Store(0)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, e := r.ReservePersonal(ctx, application("different-key"), limits)
			if e != nil && !errors.Is(e, domain.ErrConflict) {
				t.Error(e)
			}
			if created {
				n.Add(1)
			}
		}()
	}
	wg.Wait()
	if n.Load() != 1 {
		t.Fatal("parallel application count", n.Load())
	}
	// Historical attempts persist across restarts, days, and provider scopes.
	for i := 0; i < 5; i++ {
		row := application("lifetime")
		row.State = domain.Rejected
		row.CreatedAt = time.Now().Add(time.Duration(-48+i) * time.Hour)
		row.ExpiresAt = row.CreatedAt.Add(time.Minute)
		if err := db.Table(personalApplications).Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	r, err = NewPersonalRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	a = application("lifetime")
	a.Scope = "rotated"
	_, _, err = r.ReservePersonal(ctx, a, limits)
	var limit *domain.PersonalLimitError
	if !errors.As(err, &limit) || limit.Code != "VERIFICATION_TOTAL_LIMIT" {
		t.Fatalf("lifetime not enforced: %v", err)
	}
	snap, err := r.ReadPersonal(ctx, "lifetime", limits)
	if err != nil || snap.Quota.TotalUsed != 5 || snap.Quota.DailyUsed != 0 {
		t.Fatalf("restart count: %+v %v", snap.Quota, err)
	}
	// A terminal failure still consumes today's attempt and enforces the interval.
	a = application("cooldown")
	a, _, err = r.ReservePersonal(ctx, a, limits)
	if err != nil {
		t.Fatal(err)
	}
	err = r.UpdatePersonal(ctx, a.UserID, a.ID, func(x *domain.PersonalApplication, now time.Time) error { x.State = domain.Rejected; return nil })
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = r.ReservePersonal(ctx, application(a.UserID), limits)
	if !errors.As(err, &limit) || limit.Code != "VERIFICATION_COOLDOWN" {
		t.Fatalf("cooldown missing: %v", err)
	}
	// Once replaced, a delayed result cannot mutate the former application.
	if err = db.Table(personalApplications).Where("id = ?", a.ID).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	b, created, err := r.ReservePersonal(ctx, application(a.UserID), limits)
	if err != nil || !created {
		t.Fatal(err)
	}
	if err = r.UpdatePersonal(ctx, a.UserID, a.ID, func(x *domain.PersonalApplication, _ time.Time) error { x.State = domain.Verified; return nil }); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale result accepted", b.ID, err)
	}
	// The database uses Beijing midnight, including its boundary exactly once.
	day, err := r.ReadPersonal(ctx, "day-split", limits)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		row := application("day-split")
		row.State = domain.Rejected
		row.CreatedAt = day.Quota.ResetAt.Add(-24 * time.Hour).Add(time.Duration(i-1) * time.Second)
		row.ExpiresAt = row.CreatedAt.Add(time.Minute)
		if err = db.Table(personalApplications).Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	day, err = r.ReadPersonal(ctx, "day-split", limits)
	if err != nil || day.Quota.TotalUsed != 4 || day.Quota.DailyUsed != 3 {
		t.Fatalf("Beijing day boundary: %+v %v", day.Quota, err)
	}
	_, _, err = r.ReservePersonal(ctx, application("day-split"), limits)
	if !errors.As(err, &limit) || limit.Code != "VERIFICATION_DAILY_LIMIT" {
		t.Fatalf("daily ceiling: %v", err)
	}

	// A real concurrent query cannot be duplicated or confirm a replaced attempt.
	protection, _ := NewProtection(make([]byte, 32))
	p := &personalBlockingProvider{started: make(chan struct{}), finish: make(chan struct{})}
	s := &domain.PersonalService{Store: r, Provider: p, Protection: protection, Scope: "scope", SceneID: 123, Limits: limits}
	a = application("lease")
	a.PhoneDigest = protection.Digest("personal-phone", "13800000001")
	a, _, err = r.ReservePersonal(ctx, a, limits)
	if err != nil {
		t.Fatal(err)
	}
	err = r.UpdatePersonal(ctx, a.UserID, a.ID, func(x *domain.PersonalApplication, now time.Time) error {
		x.State = domain.Pending
		x.ProviderCertifyID = "original-cert"
		x.PhoneVerifiedAt = now
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	actor := domain.Actor{UserID: a.UserID, VerifiedPhone: "+8613800000001"}
	done := make(chan error, 1)
	go func() { done <- s.Refresh(ctx, actor, a.ID) }()
	<-p.started
	err = s.Refresh(ctx, actor, a.ID)
	if !errors.As(err, &limit) || limit.Code != "VERIFICATION_REFRESH_BUSY" {
		t.Fatalf("parallel query accepted: %v", err)
	}
	if err = db.Table(personalApplications).Where("id = ?", a.ID).Updates(map[string]any{"created_at": time.Now().Add(-time.Hour), "expires_at": time.Now().Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	b, created, err = r.ReservePersonal(ctx, application(a.UserID), limits)
	if err != nil || !created {
		t.Fatal(err)
	}
	close(p.finish)
	if err = <-done; !errors.Is(err, domain.ErrConflict) {
		t.Fatal("late query confirmed superseded identity", err)
	}
	latest, err := r.ReadPersonal(ctx, a.UserID, limits)
	if err != nil || latest.Application.ID != b.ID || latest.Application.State != domain.Unknown || p.calls.Load() != 1 {
		t.Fatal("query race changed current identity", err)
	}
}

type personalBlockingProvider struct {
	started, finish chan struct{}
	calls           atomic.Int32
}

func (*personalBlockingProvider) VerifyPhone(context.Context, domain.PersonalIdentity) (bool, error) {
	panic("not used")
}
func (*personalBlockingProvider) CreateFace(context.Context, string, int64, domain.PersonalIdentity, string) (domain.PersonalLink, error) {
	panic("not used")
}
func (p *personalBlockingProvider) QueryFace(ctx context.Context, scene int64, id string) (domain.PersonalResult, error) {
	p.calls.Add(1)
	close(p.started)
	select {
	case <-p.finish:
		return domain.PersonalResult{Passed: "T"}, nil
	case <-ctx.Done():
		return domain.PersonalResult{}, ctx.Err()
	}
}
