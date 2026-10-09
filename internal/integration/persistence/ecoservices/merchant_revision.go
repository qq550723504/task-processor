package ecoservices

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	e "task-processor/internal/ecoservices"
	"time"
)

func merchantReviewedLicense(app e.Application, id string) bool {
	for _, v := range app.FileIDs {
		if id == v {
			return true
		}
	}
	return false
}
func merchantApprovalFingerprint(app e.Application) string {
	return e.Fingerprint([]any{app.CompanyName, app.RegistrationNumber, app.Categories, app.Regions, app.FileIDs, app.AgreementVersion})
}
func saveMerchantApplication(tx *gorm.DB, app e.Application, actor string, now time.Time) error {
	if err := tx.Save(applicationRecord(app)).Error; err != nil {
		return err
	}
	raw, _ := json.Marshal(app)
	return tx.Create(&versionRow{ID: app.ID, Kind: "APPLICATION", Version: app.Version, ActorID: actor, Payload: raw, CreatedAt: now}).Error
}
func merchantFiles(tx *gorm.DB, in e.MerchantIntent) error {
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
	return nil
}

type merchantAcceptanceRecord struct {
	Proof       e.MerchantSubmissionAcceptance
	Fingerprint string
	Sealed      []byte
}

func loadMerchantAttempt(tx *gorm.DB, app e.Application) (e.MerchantAttempt, error) {
	var a e.MerchantAttempt
	var root merchantIntentRow
	if err := tx.Where("application_id=? AND organization_id=?", app.ID, app.OrganizationID).Take(&root).Error; err != nil {
		return a, e.ErrNotFound
	}
	if json.Unmarshal(root.Payload, &a.Intent) != nil || a.Intent.ID != root.ID || a.Intent.OutRequestNo != root.OutRequestNo || a.Intent.Fingerprint != root.Fingerprint || a.Intent.ApplicationID != app.ID || a.Intent.OrganizationID != app.OrganizationID {
		return a, e.ErrConflict
	}
	var row versionRow
	if err := tx.Where("id=? AND kind=? AND version=?", root.ID, "MERCHANT_DETAILS", app.CurrentMerchantRevisionVersion).Take(&row).Error; err != nil {
		return a, e.ErrConflict
	}
	if json.Unmarshal(row.Payload, &a.Revision) != nil || a.Revision.ID != app.CurrentMerchantRevisionID || a.Revision.Version != app.CurrentMerchantRevisionVersion || a.Revision.Version < 1 || a.Revision.Input.ID != a.Revision.ID || a.Revision.Input.ApplicationID != app.ID || a.Revision.Input.OrganizationID != app.OrganizationID || a.Revision.Input.Profile != a.Intent.Profile || a.Revision.Input.OutRequestNo != a.Intent.OutRequestNo {
		return a, e.ErrConflict
	}
	var p merchantProgressRow
	if err := tx.Where("id=?", a.Revision.ID).Take(&p).Error; err != nil {
		return a, err
	}
	a.CompanyName = root.CompanyName
	a.RegistrationNumber = root.RegistrationNumber
	a.State = p.State
	a.ClaimToken = p.ClaimToken
	a.ClaimUntil = p.ClaimUntil
	a.Dispatched = p.Dispatched
	a.DispatchActorID = p.DispatchActorID
	a.SealedObservation = p.SealedObservation
	a.UpdatedAt = p.UpdatedAt
	a.MediaIDs = map[string]string{}
	a.Current = true
	if json.Unmarshal(p.MediaIDs, &a.MediaIDs) != nil {
		return a, e.ErrConflict
	}
	if p.AcceptanceFingerprint != "" {
		var v versionRow
		var saved merchantAcceptanceRecord
		if err := tx.Where("id=? AND kind=? AND version=?", root.ID, "MERCHANT_SUBMISSION_ACCEPTANCE", a.Revision.Version).Take(&v).Error; err != nil {
			return a, e.ErrConflict
		}
		if json.Unmarshal(v.Payload, &saved) != nil || saved.Fingerprint != p.AcceptanceFingerprint || e.Fingerprint(saved.Proof) != saved.Fingerprint || !saved.Proof.Matches(a) || len(saved.Sealed) == 0 {
			return a, e.ErrConflict
		}
		a.Acceptance = &saved.Proof
		a.SealedAcceptance = saved.Sealed
	}
	return a, nil
}

func (r *Repository) SaveMerchantAcceptance(ctx context.Context, a e.MerchantAttempt, proof e.MerchantSubmissionAcceptance, sealed []byte) error {
	if !proof.Matches(a) || len(sealed) == 0 || len(sealed) > 65580 {
		return e.ErrConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := merchantClaim(tx, a)
		if err != nil {
			return err
		}
		if !p.Dispatched || p.ChannelApplicationID != "" && p.ChannelApplicationID != proof.ChannelApplicationID {
			return e.ErrConflict
		}
		fp := e.Fingerprint(proof)
		payload, _ := json.Marshal(merchantAcceptanceRecord{Proof: proof, Fingerprint: fp, Sealed: sealed})
		row := versionRow{ID: a.Intent.ID, Kind: "MERCHANT_SUBMISSION_ACCEPTANCE", Version: a.Revision.Version, ActorID: "verified-channel:" + a.Revision.ID, Payload: payload, CreatedAt: time.Now().UTC()}
		if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		var stored versionRow
		var record merchantAcceptanceRecord
		if err = tx.Where("id=? AND kind=? AND version=?", row.ID, row.Kind, row.Version).Take(&stored).Error; err != nil {
			return err
		}
		if json.Unmarshal(stored.Payload, &record) != nil || record.Fingerprint != fp || e.Fingerprint(record.Proof) != fp || p.AcceptanceFingerprint != "" && p.AcceptanceFingerprint != fp {
			return e.ErrConflict
		}
		return tx.Model(&p).Update("acceptance_fingerprint", fp).Error
	})
}
