package subjectverification

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	domain "task-processor/internal/subjectverification"
)

//go:embed schema.sql
var schema string

const applications = "public.subject_verification_applications"
const messages = "public.subject_verification_messages"

type Repository struct{ db *gorm.DB }
type message struct{ Scope, MessageID, Digest, ApplicationID, Outcome string }

func InstallSchemaTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, schema)
	return err
}

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil, domain.ErrUnavailable
	}
	// Runtime verifies the current shape without performing DDL or repairing data.
	if err := db.WithContext(ctx).Table(applications).Select("id, scope, organization_id, actor_id, idempotency_key, input_digest, company_name, credit_code, phone_digest, masked_phone, correlation, state, encrypted_url, provider_organization_id, provider_admin_id, created_at, expires_at, provider_verified_at, observed_at").Limit(0).Find(&[]domain.Application{}).Error; err != nil {
		return nil, fmt.Errorf("verification schema unavailable: %w", err)
	}
	if err := db.WithContext(ctx).Table(messages).Select("scope, message_id, digest, application_id, outcome").Limit(0).Find(&[]message{}).Error; err != nil {
		return nil, fmt.Errorf("verification receipt schema unavailable: %w", err)
	}
	var keys int64
	err := db.WithContext(ctx).Raw(`WITH required(table_name,kind,columns) AS (VALUES
 ('subject_verification_applications','p','id'),
 ('subject_verification_applications','u','organization_id'),
 ('subject_verification_applications','u','scope,correlation'),
 ('subject_verification_messages','p','scope,message_id'))
 SELECT count(*) FROM required r WHERE EXISTS (
 SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid
 JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_index i ON i.indexrelid=c.conindid
 WHERE n.nspname='public' AND t.relname=r.table_name AND c.contype::text=r.kind
 AND c.convalidated AND NOT c.condeferrable AND i.indisvalid AND i.indisready
 AND (SELECT string_agg(a.attname,',' ORDER BY k.ordinality)
 FROM unnest(c.conkey) WITH ORDINALITY k(attnum,ordinality)
 JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum)=r.columns)`).Scan(&keys).Error
	if err != nil || keys != 4 {
		return nil, errors.New("verification durable keys unavailable")
	}
	return &Repository{db: db}, nil
}
func (r *Repository) Reserve(ctx context.Context, a domain.Application) (domain.Application, bool, error) {
	result := r.db.WithContext(ctx).Table(applications).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "organization_id"}}, DoNothing: true}).Create(&a)
	if result.Error != nil {
		return domain.Application{}, false, result.Error
	}
	if result.RowsAffected == 1 {
		return a, true, nil
	}
	existing, err := r.Read(ctx, a.OrganizationID)
	return existing, false, err
}
func (r *Repository) Read(ctx context.Context, org string) (domain.Application, error) {
	var a domain.Application
	err := r.db.WithContext(ctx).Table(applications).Where("organization_id = ?", org).Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = domain.ErrNotFound
	}
	return a, err
}
func (r *Repository) SaveLink(ctx context.Context, id string, ciphertext []byte, expires time.Time) error {
	return r.db.WithContext(ctx).Table(applications).Where("id = ? AND state = ?", id, domain.Unknown).Updates(map[string]any{"state": domain.Pending, "encrypted_url": ciphertext, "expires_at": expires}).Error
}
func (r *Repository) Apply(ctx context.Context, receipt domain.Receipt, apply func(*domain.Application) string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var a domain.Application
		if err := tx.Table(applications).Clauses(clause.Locking{Strength: "UPDATE"}).Where("scope = ? AND correlation = ?", receipt.Scope, receipt.Correlation).Take(&a).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}
		m := message{Scope: receipt.Scope, MessageID: receipt.MessageID, Digest: receipt.Digest, ApplicationID: a.ID, Outcome: "RECEIVED"}
		inserted := tx.Table(messages).Clauses(clause.OnConflict{DoNothing: true}).Create(&m)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 0 {
			var prior message
			if err := tx.Table(messages).Where("scope = ? AND message_id = ?", m.Scope, m.MessageID).Take(&prior).Error; err != nil {
				return err
			}
			if prior.Digest != m.Digest || prior.ApplicationID != a.ID {
				return domain.ErrConflict
			}
			return nil
		}
		outcome := apply(&a)
		if err := tx.Table(applications).Where("id = ?", a.ID).Updates(map[string]any{"state": a.State, "encrypted_url": a.EncryptedURL, "provider_organization_id": a.ProviderOrganizationID, "provider_admin_id": a.ProviderAdminID, "provider_verified_at": a.ProviderVerifiedAt, "observed_at": a.ObservedAt}).Error; err != nil {
			return err
		}
		return tx.Table(messages).Where("scope = ? AND message_id = ?", m.Scope, m.MessageID).Update("outcome", outcome).Error
	})
}
