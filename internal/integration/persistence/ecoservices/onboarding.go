package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	e "task-processor/internal/ecoservices"
	"time"
)

type merchantIntentRow struct {
	ID                                           string `gorm:"primaryKey"`
	ApplicationID                                string `gorm:"uniqueIndex;not null"`
	OrganizationID                               string `gorm:"index;not null"`
	OutRequestNo                                 string `gorm:"uniqueIndex;not null"`
	Fingerprint, CompanyName, RegistrationNumber string
	Payload                                      []byte
	CreatedAt                                    time.Time
}

func (r *Repository) CheckMerchantApplicationVersion(ctx context.Context, scope e.Scope, appID string, version int64) error {
	if scope.Platform {
		return e.ErrForbidden
	}
	var row applicationRow
	if err := r.db.WithContext(ctx).Where("id=? AND organization_id=?", appID, scope.OrganizationID).Take(&row).Error; err != nil {
		return e.ErrNotFound
	}
	app, err := applicationFact(row)
	if err != nil {
		return err
	}
	if app.Version != version {
		return e.ErrConflict
	}
	if (app.State != "APPROVED" && app.State != "ACTIVE") || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion {
		return e.ErrNotQualified
	}
	return nil
}

func (r *Repository) VerifyProviderQualification(ctx context.Context, org, merchant string) error {
	var row applicationRow
	if err := r.db.WithContext(ctx).Where("organization_id=? AND state=? AND merchant_id=?", org, "ACTIVE", merchant).Take(&row).Error; err != nil {
		return e.ErrNotQualified
	}
	app, err := applicationFact(row)
	if err != nil {
		return err
	}
	if merchant == "" || !app.AgreementAccepted || app.OnboardingState != "FINISH" {
		return e.ErrNotQualified
	}
	return nil
}

func (merchantIntentRow) TableName() string { return "ecoservices_merchant_intents" }

type merchantProgressRow struct {
	ID                                      string `gorm:"primaryKey"`
	State, ClaimToken, ChannelApplicationID string
	ClaimUntil                              time.Time
	Dispatched                              bool
	DispatchActorID                         string
	AdmittedAt                              *time.Time
	MediaIDs, SealedObservation             []byte
	UpdatedAt                               time.Time
}

func (merchantProgressRow) TableName() string { return "ecoservices_merchant_progress" }
func merchantFact(in merchantIntentRow, p merchantProgressRow) (e.MerchantAttempt, error) {
	var a e.MerchantAttempt
	if json.Unmarshal(in.Payload, &a.Intent) != nil || a.Intent.ID != in.ID || a.Intent.ApplicationID != in.ApplicationID || a.Intent.OrganizationID != in.OrganizationID || a.Intent.OutRequestNo != in.OutRequestNo || a.Intent.Fingerprint != in.Fingerprint {
		return a, e.ErrConflict
	}
	a.CompanyName = in.CompanyName
	a.RegistrationNumber = in.RegistrationNumber
	a.State = p.State
	a.ClaimToken = p.ClaimToken
	a.ClaimUntil = p.ClaimUntil
	a.Dispatched = p.Dispatched
	a.DispatchActorID = p.DispatchActorID
	a.SealedObservation = p.SealedObservation
	a.UpdatedAt = p.UpdatedAt
	a.MediaIDs = map[string]string{}
	if len(p.MediaIDs) > 0 && json.Unmarshal(p.MediaIDs, &a.MediaIDs) != nil {
		return a, e.ErrConflict
	}
	return a, nil
}
func (r *Repository) CreateMerchantAttempt(ctx context.Context, in e.MerchantIntent) (e.MerchantAttempt, error) {
	var a e.MerchantAttempt
	if !e.ValidID(in.ID) || !e.ValidID(in.ApplicationID) || !e.ValidID(in.Key) || in.OrganizationID == "" || in.ActorID == "" || in.Fingerprint == "" || len(in.SealedDetails) == 0 || len(in.SealedDetails) > 65580 || len(in.FileIDs) < 2 || len(in.FileIDs) > 11 || in.Profile.Version == "" || in.Profile.PlatformMerchantID == "" || in.OutRequestNo == "" {
		return a, e.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", in.ApplicationID, in.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		var old merchantIntentRow
		if err := tx.Where("application_id=?", in.ApplicationID).Take(&old).Error; err == nil {
			if old.ID != in.ID || old.Fingerprint != in.Fingerprint {
				return e.ErrConflict
			}
			var progress merchantProgressRow
			if err = tx.Where("id=?", old.ID).Take(&progress).Error; err != nil {
				return err
			}
			a, err = merchantFact(old, progress)
			return err
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		if app.State != "APPROVED" || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion {
			return e.ErrNotQualified
		}
		if app.Version != in.ApplicationVersion {
			return e.ErrConflict
		}
		reviewedLicense := false
		for _, id := range app.FileIDs {
			if id == in.LicenseFileID {
				reviewedLicense = true
			}
		}
		if !reviewedLicense {
			return e.ErrConflict
		}
		seen := map[string]bool{}
		for _, id := range in.FileIDs {
			if !e.ValidID(id) || seen[id] {
				return e.ErrInvalid
			}
			seen[id] = true
			var f fileRow
			if err := tx.Where("id=? AND organization_id=? AND parent_kind=? AND parent_id=? AND state=?", id, in.OrganizationID, "APPLICATION", in.ApplicationID, "CONFIRMED").Take(&f).Error; err != nil {
				return e.ErrNotFound
			}
			if f.SizeBytes < 1 || f.SizeBytes > 2<<20 || f.ContentType != "image/jpeg" && f.ContentType != "image/png" {
				return e.ErrInvalid
			}
		}
		if !seen[in.LicenseFileID] {
			return e.ErrInvalid
		}
		now := time.Now().UTC()
		raw, _ := json.Marshal(in)
		record := merchantIntentRow{ID: in.ID, ApplicationID: in.ApplicationID, OrganizationID: in.OrganizationID, OutRequestNo: in.OutRequestNo, Fingerprint: in.Fingerprint, CompanyName: app.CompanyName, RegistrationNumber: app.RegistrationNumber, Payload: raw, CreatedAt: now}
		progress := merchantProgressRow{ID: in.ID, State: "PREPARING", MediaIDs: []byte("{}"), UpdatedAt: now}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Create(&progress).Error; err != nil {
			return err
		}
		a, err = merchantFact(record, progress)
		return err
	})
	return a, err
}
func (r *Repository) ReadMerchantAttempt(ctx context.Context, scope e.Scope, appID string) (e.MerchantAttempt, error) {
	if scope.Platform {
		return e.MerchantAttempt{}, e.ErrForbidden
	}
	var in merchantIntentRow
	if err := r.db.WithContext(ctx).Where("application_id=? AND organization_id=?", appID, scope.OrganizationID).Take(&in).Error; err != nil {
		return e.MerchantAttempt{}, e.ErrNotFound
	}
	var p merchantProgressRow
	if err := r.db.WithContext(ctx).Where("id=?", in.ID).Take(&p).Error; err != nil {
		return e.MerchantAttempt{}, err
	}
	return merchantFact(in, p)
}
func (r *Repository) ClaimMerchantAttempt(ctx context.Context, scope e.Scope, appID string) (e.MerchantAttempt, bool, error) {
	var a e.MerchantAttempt
	var claimed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if scope.Platform {
			return e.ErrForbidden
		}
		var in merchantIntentRow
		if err := tx.Where("application_id=? AND organization_id=?", appID, scope.OrganizationID).Take(&in).Error; err != nil {
			return e.ErrNotFound
		}
		var p merchantProgressRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", in.ID).Take(&p).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if !p.ClaimUntil.After(now) {
			p.ClaimToken = uuid.NewString()
			p.ClaimUntil = now.Add(90 * time.Second)
			if err := tx.Model(&p).Updates(map[string]any{"claim_token": p.ClaimToken, "claim_until": p.ClaimUntil}).Error; err != nil {
				return err
			}
			claimed = true
		}
		var err error
		a, err = merchantFact(in, p)
		return err
	})
	return a, claimed, err
}
func merchantClaim(tx *gorm.DB, a e.MerchantAttempt) (merchantProgressRow, error) {
	var p merchantProgressRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", a.Intent.ID).Take(&p).Error; err != nil {
		return p, err
	}
	if a.ClaimToken == "" || p.ClaimToken != a.ClaimToken || !p.ClaimUntil.After(time.Now().UTC()) {
		return p, e.ErrConflict
	}
	return p, nil
}
func (r *Repository) SaveMerchantMedia(ctx context.Context, a e.MerchantAttempt, id, media string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := merchantClaim(tx, a)
		if err != nil {
			return err
		}
		valid := false
		for _, v := range a.Intent.FileIDs {
			if v == id {
				valid = true
			}
		}
		if !valid || media == "" || len(media) > 256 {
			return e.ErrInvalid
		}
		var ids map[string]string
		if json.Unmarshal(p.MediaIDs, &ids) != nil {
			return e.ErrConflict
		}
		if old := ids[id]; old != "" && old != media {
			return e.ErrConflict
		}
		ids[id] = media
		raw, _ := json.Marshal(ids)
		return tx.Model(&p).Update("media_ids", raw).Error
	})
}
func (r *Repository) MarkMerchantDispatched(ctx context.Context, a e.MerchantAttempt) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", a.Intent.ApplicationID, a.Intent.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		if app.State != "APPROVED" || app.MerchantID != "" || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion || app.CompanyName != a.CompanyName || app.RegistrationNumber != a.RegistrationNumber || a.DispatchActorID == "" {
			return e.ErrNotQualified
		}
		p, err := merchantClaim(tx, a)
		if err != nil {
			return err
		}
		var ids map[string]string
		if json.Unmarshal(p.MediaIDs, &ids) != nil {
			return e.ErrConflict
		}
		for _, id := range a.Intent.FileIDs {
			if ids[id] == "" {
				return e.ErrConflict
			}
		}
		if p.ChannelApplicationID != "" || len(p.SealedObservation) > 0 {
			return e.ErrConflict
		}
		return tx.Model(&p).Updates(map[string]any{"dispatched": true, "dispatch_actor_id": a.DispatchActorID, "admitted_at": time.Now().UTC()}).Error
	})
}
func (r *Repository) ObserveMerchant(ctx context.Context, a e.MerchantAttempt, o e.MerchantObservation, sealed []byte) error {
	if !o.Matches(a) || o.ChannelApplicationID == "" || len(o.ChannelApplicationID) > 32 || len(sealed) == 0 || len(sealed) > 65580 {
		return e.ErrConflict
	}
	switch o.State {
	case "CHECKING", "ACCOUNT_NEED_VERIFY", "AUDITING", "REJECTED", "NEED_SIGN", "FINISH", "FROZEN", "CANCELED":
	default:
		return e.ErrConflict
	}
	if o.State == "FINISH" && (o.SignState != "SIGNED" || o.MerchantID == "" || o.MerchantID == a.Intent.Profile.PlatformMerchantID) {
		return e.ErrConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", a.Intent.ApplicationID, a.Intent.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		p, err := merchantClaim(tx, a)
		if err != nil {
			return err
		}
		if !p.Dispatched || p.ChannelApplicationID != "" && p.ChannelApplicationID != o.ChannelApplicationID {
			return e.ErrConflict
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		if app.CompanyName != a.CompanyName || app.RegistrationNumber != a.RegistrationNumber {
			return e.ErrConflict
		}
		if o.State == "FINISH" {
			binding := merchantBindingRow{ApplicationID: app.ID, OrganizationID: app.OrganizationID, MerchantID: o.MerchantID, OriginalAttemptID: a.Intent.ID, Proof: sealed}
			if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error; err != nil {
				return err
			}
			var original merchantBindingRow
			if err = tx.Where("application_id=?", app.ID).Take(&original).Error; err != nil || original.OrganizationID != binding.OrganizationID || original.MerchantID != binding.MerchantID || original.OriginalAttemptID != binding.OriginalAttemptID {
				return e.ErrConflict
			}
			if app.MerchantID != "" && app.MerchantID != o.MerchantID {
				return e.ErrConflict
			}
			app.MerchantID = o.MerchantID
		}
		state := app.State
		if state == "ACTIVE" {
			state = "APPROVED"
		}
		if (state == "APPROVED") && app.AgreementAccepted && app.AgreementVersion == e.PolicyVersion && o.State == "FINISH" && app.MerchantID != "" {
			state = "ACTIVE"
		}
		now := time.Now().UTC()
		if app.State != state || app.OnboardingState != o.State {
			app.State = state
			app.OnboardingState = o.State
			app.Version++
			app.UpdatedAt = now
			if err = tx.Save(applicationRecord(app)).Error; err != nil {
				return err
			}
			raw, _ := json.Marshal(app)
			if err = tx.Create(&versionRow{ID: app.ID, Kind: "APPLICATION", Version: app.Version, ActorID: "verified-channel:" + a.Intent.ID, Payload: raw, CreatedAt: now}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&p).Updates(map[string]any{"state": o.State, "channel_application_id": o.ChannelApplicationID, "sealed_observation": sealed, "updated_at": now}).Error
	})
}
func (r *Repository) ReleaseMerchantClaim(ctx context.Context, a e.MerchantAttempt) error {
	return r.db.WithContext(ctx).Model(&merchantProgressRow{}).Where("id=? AND claim_token=?", a.Intent.ID, a.ClaimToken).Updates(map[string]any{"claim_token": "", "claim_until": time.Time{}}).Error
}
