package ecoservices

import (
	"context"
	"database/sql"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	e "task-processor/internal/ecoservices"
	"time"
)

func fileFact(row fileRow) e.File {
	return e.File{ID: row.ID, OrganizationID: row.OrganizationID, ParentID: row.ParentID, ParentKind: row.ParentKind, Filename: row.Filename, ContentType: row.ContentType, SizeBytes: row.SizeBytes, SHA256: row.SHA256, ObjectKey: row.ObjectKey, State: row.State}
}
func authorizeFileParent(tx *gorm.DB, scope e.Scope, parentKind, parentID string) error {
	if parentID == "" {
		if scope.Platform {
			return e.ErrForbidden
		}
		return nil
	}
	switch parentKind {
	case "APPLICATION":
		var row applicationRow
		q := tx.Where("id=?", parentID)
		if !scope.Platform {
			q = q.Where("organization_id=?", scope.OrganizationID)
		}
		if err := q.Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
	case "REQUEST":
		var row requestRow
		q := tx.Where("id=?", parentID)
		if !scope.Platform {
			q = q.Where("buyer_organization_id=? OR provider_organization_id=?", scope.OrganizationID, scope.OrganizationID)
		}
		if err := q.Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
	default:
		return e.ErrInvalid
	}
	return nil
}
func (r *Repository) CreateFileIntent(ctx context.Context, in e.FileIntent) (e.File, error) {
	var out e.File
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.Scope.Platform || in.File.OrganizationID != in.Scope.OrganizationID || in.Fingerprint == "" {
			return e.ErrForbidden
		}
		if err := authorizeFileParent(tx, in.Scope, in.File.ParentKind, in.File.ParentID); err != nil {
			return err
		}
		var existing fileRow
		if err := tx.Where("id=?", in.File.ID).Take(&existing).Error; err == nil {
			if existing.Fingerprint != in.Fingerprint || existing.OrganizationID != in.Scope.OrganizationID {
				return e.ErrConflict
			}
			out = fileFact(existing)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		file := in.File
		row := fileRow{ID: file.ID, OrganizationID: in.Scope.OrganizationID, ParentID: file.ParentID, ParentKind: file.ParentKind, ActorID: in.Scope.ActorID, Fingerprint: in.Fingerprint, Filename: file.Filename, ContentType: file.ContentType, SizeBytes: file.SizeBytes, SHA256: file.SHA256, ObjectKey: file.ObjectKey, State: "PENDING", CreatedAt: time.Now().UTC()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("id=?", in.File.ID).Take(&existing).Error; err != nil {
			return err
		}
		if existing.Fingerprint != in.Fingerprint {
			return e.ErrConflict
		}
		out = fileFact(existing)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) ConfirmFile(ctx context.Context, in e.FileIntent) (e.File, error) {
	var out e.File
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeFileParent(tx, in.Scope, in.File.ParentKind, in.File.ParentID); err != nil {
			return err
		}
		var row fileRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=?", in.File.ID, in.Scope.OrganizationID).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		if row.Fingerprint != in.Fingerprint {
			return e.ErrConflict
		}
		if err := tx.Model(&row).Update("state", "CONFIRMED").Error; err != nil {
			return err
		}
		row.State = "CONFIRMED"
		out = fileFact(row)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) ReadFile(ctx context.Context, scope e.Scope, id string) (e.File, error) {
	var out e.File
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row fileRow
		if err := tx.Where("id=?", id).Take(&row).Error; err != nil {
			return e.ErrNotFound
		}
		if row.ParentID == "" {
			if scope.Platform || row.OrganizationID != scope.OrganizationID {
				return e.ErrNotFound
			}
		} else if err := authorizeFileParent(tx, scope, row.ParentKind, row.ParentID); err != nil {
			return err
		}
		out = fileFact(row)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
