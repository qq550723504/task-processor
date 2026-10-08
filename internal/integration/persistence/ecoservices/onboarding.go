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
	AcceptanceFingerprint                   string
	UpdatedAt                               time.Time
}

func (merchantProgressRow) TableName() string { return "ecoservices_merchant_progress" }
func (r *Repository) CreateMerchantAttempt(ctx context.Context, in e.MerchantIntent) (e.MerchantAttempt, error) {
	var a e.MerchantAttempt
	if !e.ValidID(in.ID) || !e.ValidID(in.ApplicationID) || !e.ValidID(in.Key) || in.OrganizationID == "" || in.ActorID == "" || in.Fingerprint == "" || len(in.SealedDetails) == 0 || len(in.SealedDetails) > 65580 || len(in.FileIDs) < 2 || len(in.FileIDs) > 11 || in.Profile.Version == "" || in.Profile.PlatformMerchantID == "" || in.OutRequestNo == "" || in.ExpectedRevisionVersion < 0 {
		return a, e.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		command := e.Command{Scope: e.Scope{OrganizationID: in.OrganizationID, ActorID: in.ActorID}, Kind: "merchant_submit", Key: in.Key, Fingerprint: in.Fingerprint}
		if err := lockOperation(tx, command); err != nil {
			return err
		}
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", in.ApplicationID, in.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		var receipt operationRow
		if err = tx.Where("organization_id=? AND kind=? AND key=?", in.OrganizationID, "merchant_submit", in.Key).Take(&receipt).Error; err == nil {
			if receipt.Fingerprint != in.Fingerprint {
				return e.ErrConflict
			}
			var ref struct {
				IntentID, RevisionID string
				Version              int64
			}
			if json.Unmarshal(receipt.Result, &ref) != nil || ref.IntentID == "" || ref.RevisionID == "" || ref.Version < 1 {
				return e.ErrConflict
			}
			currentID := app.CurrentMerchantRevisionID
			app.CurrentMerchantRevisionID = ref.RevisionID
			app.CurrentMerchantRevisionVersion = ref.Version
			a, err = loadMerchantAttempt(tx, app)
			if err == nil && a.Intent.ID != ref.IntentID {
				return e.ErrConflict
			}
			a.Current = ref.RevisionID == currentID
			return err
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if app.Version != in.ApplicationVersion || app.MerchantID != "" {
			return e.ErrConflict
		}
		if app.State != "APPROVED" || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion {
			return e.ErrNotQualified
		}
		if err = merchantFiles(tx, in); err != nil {
			return err
		}
		now := time.Now().UTC()
		var root merchantIntentRow
		var previous e.MerchantAttempt
		var detailVersion int64 = 1
		if err = tx.Where("application_id=?", app.ID).Take(&root).Error; err == nil {
			previous, err = loadMerchantAttempt(tx, app)
			if err != nil {
				return err
			}
			// A lease expiry is not dispatch evidence. Only the saved, trusted terminal
			// observation for the current revision permits a different sealed payload.
			var p merchantProgressRow
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", previous.Revision.ID).Take(&p).Error; err != nil {
				return err
			}
			if in.ExpectedRevisionVersion != previous.Revision.Version || previous.State != "REJECTED" || !previous.Dispatched || len(previous.SealedObservation) == 0 || p.ChannelApplicationID == "" || p.ClaimUntil.After(now) || !previous.CanObserve() || in.Profile != previous.Intent.Profile {
				return e.ErrConflict
			}
			in.OutRequestNo = previous.Intent.OutRequestNo
			detailVersion = previous.Revision.Version + 1
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if in.ExpectedRevisionVersion != 0 || app.CurrentMerchantRevisionID != "" {
				return e.ErrConflict
			}
			if !merchantReviewedLicense(app, in.LicenseFileID) {
				return e.ErrConflict
			}
			raw, _ := json.Marshal(in)
			root = merchantIntentRow{ID: in.ID, ApplicationID: in.ApplicationID, OrganizationID: in.OrganizationID, OutRequestNo: in.OutRequestNo, Fingerprint: in.Fingerprint, CompanyName: app.CompanyName, RegistrationNumber: app.RegistrationNumber, Payload: raw, CreatedAt: now}
			if err = tx.Create(&root).Error; err != nil {
				return err
			}
		} else {
			return err
		}
		revision := e.MerchantDetailsRevision{ID: in.ID, Version: detailVersion, Input: in, ReviewedApplicationVersion: app.Version, AgreementVersion: app.AgreementVersion}
		if detailVersion > 1 && in.LicenseFileID != previous.Revision.Input.LicenseFileID {
			for n, id := range app.FileIDs {
				if id == previous.Revision.Input.LicenseFileID {
					app.FileIDs[n] = in.LicenseFileID
				}
			}
			if !merchantReviewedLicense(app, in.LicenseFileID) {
				return e.ErrConflict
			}
			app.State = "SUBMITTED"
			app.AgreementAccepted = false
			app.ReviewReason = ""
			revision.ReviewedApplicationVersion = 0
		}
		revision.ApprovalFingerprint = merchantApprovalFingerprint(app)
		payload, _ := json.Marshal(revision)
		if err = tx.Create(&versionRow{ID: root.ID, Kind: "MERCHANT_DETAILS", Version: detailVersion, ActorID: in.ActorID, Payload: payload, CreatedAt: now}).Error; err != nil {
			return err
		}
		if err = tx.Create(&merchantProgressRow{ID: revision.ID, State: "PREPARING", MediaIDs: []byte("{}"), UpdatedAt: now}).Error; err != nil {
			return err
		}
		app.CurrentMerchantRevisionID = revision.ID
		app.CurrentMerchantRevisionVersion = detailVersion
		app.OnboardingState = "PREPARING"
		app.Version++
		app.UpdatedAt = now
		if err = saveMerchantApplication(tx, app, in.ActorID, now); err != nil {
			return err
		}
		result, _ := json.Marshal(struct {
			IntentID, RevisionID string
			Version              int64
		}{root.ID, revision.ID, detailVersion})
		if err = tx.Create(&operationRow{OrganizationID: in.OrganizationID, Kind: "merchant_submit", Key: in.Key, Fingerprint: in.Fingerprint, Result: result}).Error; err != nil {
			return err
		}
		a, err = loadMerchantAttempt(tx, app)
		return err
	})
	return a, err
}
func (r *Repository) ReadMerchantAttempt(ctx context.Context, scope e.Scope, appID string) (e.MerchantAttempt, error) {
	var a e.MerchantAttempt
	if scope.Platform {
		return a, e.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id=? AND organization_id=?", appID, scope.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		a, err = loadMerchantAttempt(tx, app)
		return err
	})
	return a, err
}
func (r *Repository) ClaimMerchantAttempt(ctx context.Context, scope e.Scope, appID string) (e.MerchantAttempt, bool, error) {
	var a e.MerchantAttempt
	var claimed bool
	if scope.Platform {
		return a, false, e.ErrForbidden
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", appID, scope.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		app, err := applicationFact(row)
		if err != nil {
			return err
		}
		a, err = loadMerchantAttempt(tx, app)
		if err != nil {
			return err
		}
		var p merchantProgressRow
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", a.Revision.ID).Take(&p).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if !p.ClaimUntil.After(now) {
			a.ClaimToken = uuid.NewString()
			a.ClaimUntil = now.Add(90 * time.Second)
			if err = tx.Model(&p).Updates(map[string]any{"claim_token": a.ClaimToken, "claim_until": a.ClaimUntil}).Error; err != nil {
				return err
			}
			claimed = true
		}
		return nil
	})
	return a, claimed, err
}
func merchantClaim(tx *gorm.DB, a e.MerchantAttempt) (merchantProgressRow, error) {
	var p merchantProgressRow
	var row applicationRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", a.Intent.ApplicationID, a.Intent.OrganizationID).Take(&row).Error; err != nil {
		return p, e.ErrNotFound
	}
	app, err := applicationFact(row)
	if err != nil {
		return p, err
	}
	if app.CurrentMerchantRevisionID != a.Revision.ID || app.CurrentMerchantRevisionVersion != a.Revision.Version {
		return p, e.ErrConflict
	}
	current, err := loadMerchantAttempt(tx, app)
	if err != nil || current.Intent.ID != a.Intent.ID || current.Revision.Input.Fingerprint != a.Revision.Input.Fingerprint {
		return p, e.ErrConflict
	}
	if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", a.Revision.ID).Take(&p).Error; err != nil {
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
		for _, v := range a.Revision.Input.FileIDs {
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
		if app.State != "APPROVED" || app.MerchantID != "" || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion || app.CompanyName != a.CompanyName || app.RegistrationNumber != a.RegistrationNumber || a.DispatchActorID == "" || !merchantReviewedLicense(app, a.Revision.Input.LicenseFileID) || merchantApprovalFingerprint(app) != a.Revision.ApprovalFingerprint {
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
		for _, id := range a.Revision.Input.FileIDs {
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
		if (a.Revision.Version > 1 || p.AcceptanceFingerprint != "") && (p.AcceptanceFingerprint == "" || a.Acceptance == nil || p.AcceptanceFingerprint != e.Fingerprint(*a.Acceptance)) {
			return e.ErrConflict
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
			if (app.State != "APPROVED" && app.State != "ACTIVE") || !app.AgreementAccepted || app.AgreementVersion != e.PolicyVersion || !merchantReviewedLicense(app, a.Revision.Input.LicenseFileID) || merchantApprovalFingerprint(app) != a.Revision.ApprovalFingerprint {
				return e.ErrNotQualified
			}
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := merchantClaim(tx, a)
		if err != nil {
			return err
		}
		return tx.Model(&p).Updates(map[string]any{"claim_token": "", "claim_until": time.Time{}}).Error
	})
}
