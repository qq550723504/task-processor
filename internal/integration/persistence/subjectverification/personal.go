package subjectverification

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"gorm.io/gorm"
	domain "task-processor/internal/subjectverification"
	"time"
)

//go:embed personal_schema.sql
var personalSchema string

const personalApplications = "public.personal_verification_applications"

type PersonalRepository struct{ db *gorm.DB }

func InstallPersonalSchemaTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, personalSchema)
	return err
}
func NewPersonalRepository(ctx context.Context, db *gorm.DB) (*PersonalRepository, error) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil, domain.ErrUnavailable
	}
	if err := db.WithContext(ctx).Table(personalApplications).Select("id,user_id,scope,idempotency_key,input_digest,identity_digest,phone_digest,masked_phone,state,provider_scene_id,provider_certify_id,encrypted_url,created_at,expires_at,phone_verified_at,verified_at,refresh_after,refresh_token").Limit(0).Find(&[]domain.PersonalApplication{}).Error; err != nil {
		return nil, errors.New("personal verification schema unavailable")
	}
	var count int64
	err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_constraint c JOIN pg_index i ON i.indexrelid=c.conindid WHERE c.conrelid='public.personal_verification_applications'::regclass AND c.contype IN ('p','u') AND NOT c.condeferrable AND c.convalidated AND i.indisvalid AND i.indisready AND (SELECT string_agg(a.attname,',' ORDER BY k.ordinality) FROM unnest(c.conkey) WITH ORDINALITY k(attnum,ordinality) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.attnum) IN ('id','user_id,idempotency_key')`).Scan(&count).Error
	if err != nil || count != 2 {
		return nil, errors.New("personal verification durable keys unavailable")
	}
	return &PersonalRepository{db}, nil
}
func (r *PersonalRepository) locked(ctx context.Context, user string, fn func(*gorm.DB, time.Time) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "personal-verification:"+user).Error; err != nil {
			return err
		}
		var now time.Time
		if err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
			return err
		}
		return fn(tx, now.UTC())
	})
}
func personalSnapshot(tx *gorm.DB, user string, l domain.PersonalLimits, now time.Time) (domain.PersonalSnapshot, error) {
	var s domain.PersonalSnapshot
	err := tx.Table(personalApplications).Where("user_id = ?", user).Order("created_at DESC, id DESC").Take(&s.Application).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return s, err
	}
	q := domain.PersonalQuota{TotalLimit: l.Total, DailyLimit: l.Daily, ServerTime: now, NextAllowedAt: now}
	// UTC+08:00 is the fixed current Beijing business-day contract.
	local := now.In(time.FixedZone("Asia/Shanghai", 8*3600))
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UTC()
	q.ResetAt = start.Add(24 * time.Hour)
	var counts struct{ Total, Daily int }
	err = tx.Table(personalApplications).Select("count(*) AS total, count(*) FILTER (WHERE created_at >= ? AND created_at < ?) AS daily", start, q.ResetAt).Where("user_id = ?", user).Scan(&counts).Error
	if err != nil {
		return s, err
	}
	q.TotalUsed = counts.Total
	q.DailyUsed = counts.Daily
	if s.Application.ID != "" {
		next := s.Application.CreatedAt.Add(time.Duration(l.IntervalSeconds) * time.Second)
		if next.After(now) {
			q.NextAllowedAt = next
		}
	}
	q.TotalRemaining = max(0, l.Total-q.TotalUsed)
	q.DailyRemaining = max(0, l.Daily-q.DailyUsed)
	s.Quota = q
	return s, nil
}
func (r *PersonalRepository) ReadPersonal(ctx context.Context, user string, l domain.PersonalLimits) (domain.PersonalSnapshot, error) {
	var s domain.PersonalSnapshot
	err := r.locked(ctx, user, func(tx *gorm.DB, now time.Time) error {
		var err error
		s, err = personalSnapshot(tx, user, l, now)
		return err
	})
	return s, err
}
func (r *PersonalRepository) ReservePersonal(ctx context.Context, a domain.PersonalApplication, l domain.PersonalLimits) (domain.PersonalApplication, bool, error) {
	created := false
	err := r.locked(ctx, a.UserID, func(tx *gorm.DB, now time.Time) error {
		var prior domain.PersonalApplication
		err := tx.Table(personalApplications).Where("user_id = ? AND idempotency_key = ?", a.UserID, a.IdempotencyKey).Take(&prior).Error
		if err == nil {
			if prior.Scope != a.Scope || prior.InputDigest != a.InputDigest {
				return domain.ErrConflict
			}
			a = prior
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		s, err := personalSnapshot(tx, a.UserID, l, now)
		if err != nil {
			return err
		}
		if err = domain.CheckPersonalAdmission(s); err != nil {
			return err
		}
		a.CreatedAt = now
		a.ExpiresAt = now.Add(35 * time.Minute)
		if err = tx.Table(personalApplications).Create(&a).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return a, created, err
}
func (r *PersonalRepository) UpdatePersonal(ctx context.Context, user, id string, apply func(*domain.PersonalApplication, time.Time) error) error {
	return r.locked(ctx, user, func(tx *gorm.DB, now time.Time) error {
		var a domain.PersonalApplication
		err := tx.Table(personalApplications).Where("user_id = ?", user).Order("created_at DESC, id DESC").Take(&a).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || err == nil && a.ID != id {
			return domain.ErrConflict
		}
		if err != nil {
			return err
		}
		if err = apply(&a, now); err != nil {
			return err
		}
		return tx.Table(personalApplications).Where("id = ?", id).Updates(map[string]any{"state": a.State, "provider_certify_id": a.ProviderCertifyID, "encrypted_url": a.EncryptedURL, "expires_at": a.ExpiresAt, "phone_verified_at": a.PhoneVerifiedAt, "verified_at": a.VerifiedAt, "refresh_after": a.RefreshAfter, "refresh_token": a.RefreshToken}).Error
	})
}
