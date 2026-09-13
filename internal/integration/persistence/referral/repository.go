package referral

import (
	"context"
	"errors"
	d "task-processor/internal/referral"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, d.ErrInvalid
	}
	if err := VerifyPermissions(context.Background(), db); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}
func databaseError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d.ErrMissing
	}
	if err != nil {
		return d.ErrUnavailable
	}
	return nil
}
func (r *Repository) transaction(ctx context.Context, f func(*gorm.DB) error) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var callbackError error
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { callbackError = f(tx); return callbackError })
	if callbackError != nil {
		return callbackError
	}
	if err != nil {
		return d.ErrUnknown
	}
	return nil
}
func (r *Repository) Find(ctx context.Context, field, issuer, value string) (d.Intent, error) {
	if field != "id" && field != "key_hash" && field != "subject" {
		return d.Intent{}, d.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var i d.Intent
	err := r.db.WithContext(ctx).Table("public.registration_intents").Where("issuer = ? AND "+field+" = ?", issuer, value).Take(&i).Error
	return i, databaseError(err)
}
func (r *Repository) Admit(ctx context.Context, i d.Intent, code string) (out d.Intent, err error) {
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", i.Issuer+i.KeyHash).Error; e != nil {
			return d.ErrUnavailable
		}
		e := tx.Table("public.registration_intents").Where("issuer=? AND key_hash=?", i.Issuer, i.KeyHash).Take(&out).Error
		if e == nil {
			if out.Fingerprint != i.Fingerprint {
				return d.ErrConflict
			}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return d.ErrUnavailable
		}
		var owner struct{ Subject string }
		if e = tx.Table("public.referral_codes").Where("issuer=? AND code=?", i.Issuer, code).Take(&owner).Error; e != nil {
			return databaseError(e)
		}
		i.Referrer = owner.Subject
		insert := tx.Table("public.registration_intents").Clauses(clause.OnConflict{DoNothing: true}).Create(&i)
		if insert.Error != nil {
			return d.ErrUnavailable
		}
		if insert.RowsAffected != 1 {
			return d.ErrConflict
		}
		out = i
		return nil
	})
	return
}
func (r *Repository) Claim(ctx context.Context, id string) (lease time.Time, err error) {
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		i, now, e := lockedIntentClock(tx, id)
		if e != nil {
			return e
		}
		if i.State == "CONSUMED" || !now.Before(i.CompletionExpiresAt) {
			return d.ErrExpired
		}
		if now.Before(i.LeaseUntil) {
			return nil
		}
		lease = now.Add(15 * time.Second)
		return databaseError(tx.Table("public.registration_intents").Where("id=?", id).Update("lease_until", lease).Error)
	})
	return
}

// Both decisions read clock_timestamp AFTER acquiring the row lock. A statement
// timestamp captured before lock contention could authorize expired work.
func lockedIntentClock(tx *gorm.DB, id string) (i d.Intent, now time.Time, err error) {
	if e := tx.Table("public.registration_intents").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&i).Error; e != nil {
		return i, now, databaseError(e)
	}
	err = databaseError(tx.Raw("SELECT clock_timestamp()").Scan(&now).Error)
	return
}

func (r *Repository) PermitCreate(ctx context.Context, id string, lease time.Time) (remaining time.Duration, err error) {
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		i, now, e := lockedIntentClock(tx, id)
		if e != nil {
			return e
		}
		if i.State == "CONSUMED" || !now.Before(i.CreateExpiresAt) || !now.Before(i.CompletionExpiresAt) {
			return d.ErrExpired
		}
		if lease.IsZero() || !i.LeaseUntil.Equal(lease) || !now.Before(lease) {
			return d.ErrPending
		}
		remaining = min(i.CreateExpiresAt.Sub(now), lease.Sub(now))
		return nil
	})
	return
}
func (r *Repository) Created(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	res := r.db.WithContext(ctx).Table("public.registration_intents").Where("id=? AND state<>'CONSUMED'", id).Updates(map[string]any{"state": "CREATED", "lease_until": time.Time{}})
	return databaseError(res.Error)
}
func (r *Repository) Receipt(ctx context.Context, issuer, subject string) (d.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var receipt d.Receipt
	err := r.db.WithContext(ctx).Table("public.referral_receipts").Where("issuer=? AND subject=?", issuer, subject).Take(&receipt).Error
	return receipt, databaseError(err)
}
func (r *Repository) Consume(ctx context.Context, input d.Intent, now time.Time) (out d.Receipt, err error) {
	err = r.transaction(ctx, func(tx *gorm.DB) error {
		var i d.Intent
		if e := tx.Table("public.registration_intents").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", input.ID).Take(&i).Error; e != nil {
			return databaseError(e)
		}
		if i.Issuer != input.Issuer || i.Subject != input.Subject || i.Fingerprint != input.Fingerprint || i.Referrer != input.Referrer {
			return d.ErrConflict
		}
		e := tx.Table("public.referral_receipts").Where("issuer=? AND subject=?", i.Issuer, i.Subject).Take(&out).Error
		if e == nil {
			if out.Referrer != i.Referrer || out.Fingerprint != i.Fingerprint {
				return d.ErrConflict
			}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return d.ErrUnavailable
		}
		var decisionTime time.Time
		if e = tx.Raw("SELECT clock_timestamp()").Scan(&decisionTime).Error; e != nil {
			return d.ErrUnavailable
		}
		if decisionTime.After(now) {
			now = decisionTime
		}
		if !now.Before(i.CompletionExpiresAt) {
			return d.ErrExpired
		}
		if i.State != "CREATED" {
			return d.ErrPending
		}
		out = d.Receipt{IntentID: i.ID, Issuer: i.Issuer, Subject: i.Subject, Referrer: i.Referrer, Fingerprint: i.Fingerprint, BoundAt: now}
		relation := map[string]any{"issuer": i.Issuer, "subject": i.Subject, "referrer": i.Referrer, "intent_id": i.ID, "bound_at": now}
		if e = tx.Table("public.referral_relations").Clauses(clause.OnConflict{DoNothing: true}).Create(relation).Error; e != nil {
			return d.ErrUnavailable
		}
		var existing d.Receipt
		if e = tx.Table("public.referral_relations").Where("issuer=? AND subject=?", i.Issuer, i.Subject).Take(&existing).Error; e != nil {
			return databaseError(e)
		}
		if existing.Referrer != i.Referrer {
			return d.ErrConflict
		}
		out.BoundAt = existing.BoundAt
		if e = tx.Table("public.referral_receipts").Create(&out).Error; e != nil {
			return d.ErrUnavailable
		}
		return databaseError(tx.Table("public.registration_intents").Where("id=?", i.ID).Updates(map[string]any{"state": "CONSUMED", "ciphertext": nil, "lease_until": time.Time{}}).Error)
	})
	return
}
func (r *Repository) Read(ctx context.Context, issuer, subject string) (d.Projection, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var p d.Projection
	err := r.db.WithContext(ctx).Raw(`SELECT COALESCE((SELECT code FROM public.referral_codes WHERE issuer=? AND subject=?),'') AS code, (SELECT COUNT(*) FROM public.referral_relations WHERE issuer=? AND referrer=?) AS count`, issuer, subject, issuer, subject).Scan(&p).Error
	p.EarningsAvailability = "unavailable"
	return p, databaseError(err)
}
func (r *Repository) CreateCode(ctx context.Context, issuer, subject, code string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	row := map[string]any{"issuer": issuer, "subject": subject, "code": code}
	if err := r.db.WithContext(ctx).Table("public.referral_codes").Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return "", d.ErrUnavailable
	}
	p, err := r.Read(ctx, issuer, subject)
	if err == nil && p.Code == "" {
		return "", d.ErrConflict
	}
	return p.Code, err
}
func (r *Repository) Cleanup(ctx context.Context, now time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	return databaseError(r.db.WithContext(ctx).Exec(`UPDATE public.registration_intents SET ciphertext=NULL WHERE id IN (SELECT id FROM public.registration_intents WHERE completion_expires_at<=? AND ciphertext IS NOT NULL ORDER BY completion_expires_at LIMIT 20 FOR UPDATE SKIP LOCKED)`, now).Error)
}
