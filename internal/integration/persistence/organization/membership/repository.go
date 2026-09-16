package membership

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	domain "task-processor/internal/organization/membership"
)

const table = "public.organization_member_operations"

var fingerprintPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Repository struct{ db *sql.DB }

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if ctx == nil || db == nil {
		return nil, domain.ErrUnavailable
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	// Read-only schema admission; explicit initialization owns all DDL.
	rows, err := sqlDB.QueryContext(ctx, `SELECT project_id,organization_id,actor_id,operation_key,target_user_id,invite_email,fingerprint,revision,active,payload FROM `+table+` LIMIT 0`)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	_ = rows.Close()
	if err := verifySchema(ctx, sqlDB); err != nil {
		return nil, domain.ErrUnavailable
	}
	return &Repository{db: sqlDB}, nil
}

func validScope(scope domain.OperationScope) bool {
	return authidentity.IsBoundedIdentifier(scope.ProjectID) && authidentity.IsBoundedIdentifier(scope.OrganizationID) && authidentity.IsBoundedIdentifier(scope.ActorID)
}
func validKey(key string) bool {
	id, err := uuid.Parse(key)
	return err == nil && id != uuid.Nil && id.String() == key
}

func (r *Repository) Begin(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	if !validScope(op.Scope) || !validKey(op.Key) || !authidentity.IsBoundedIdentifier(op.TargetUserID) || !fingerprintPattern.MatchString(op.Fingerprint) || op.Phase != domain.PhaseReady || op.Revision != 1 || op.DispatchID != "" || op.Acknowledgment != nil {
		return domain.Operation{}, domain.ErrInvalidRequest
	}
	payload, err := json.Marshal(op)
	if err != nil || len(payload) > 16384 {
		return domain.Operation{}, domain.ErrInvalidRequest
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	defer tx.Rollback()
	// Serializes only short admission transactions, including the bounded count.
	digest := sha256.Sum256([]byte(op.Scope.ProjectID + "\x00" + op.Scope.OrganizationID))
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(digest[:8]))); err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	existing, err := read(ctx, tx, op.Scope, op.Key, false)
	if err == nil {
		if existing.Fingerprint != op.Fingerprint {
			return domain.Operation{}, domain.ErrConflict
		}
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Operation{}, err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE project_id=$1 AND organization_id=$2`, op.Scope.ProjectID, op.Scope.OrganizationID).Scan(&count); err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	if count >= 10000 {
		return domain.Operation{}, domain.ErrUnavailable
	}
	var email any
	if op.Kind == domain.CommandInvite {
		if op.Invitation == nil || op.Invitation.Email == "" || len(op.Invitation.Email) > 320 {
			return domain.Operation{}, domain.ErrInvalidRequest
		}
		email = op.Invitation.Email
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (project_id,organization_id,actor_id,operation_key,target_user_id,invite_email,fingerprint,revision,active,payload) VALUES ($1,$2,$3,$4,$5,$6,$7,1,true,$8) ON CONFLICT DO NOTHING`, op.Scope.ProjectID, op.Scope.OrganizationID, op.Scope.ActorID, op.Key, op.TargetUserID, email, op.Fingerprint, payload)
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	if affected != 1 {
		return domain.Operation{}, domain.ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	return op, nil
}

func (r *Repository) Read(ctx context.Context, scope domain.OperationScope, key string) (domain.Operation, error) {
	if !validScope(scope) || !validKey(key) {
		return domain.Operation{}, domain.ErrInvalidRequest
	}
	return read(ctx, r.db, scope, key, false)
}

type rowReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func read(ctx context.Context, db rowReader, scope domain.OperationScope, key string, lock bool) (domain.Operation, error) {
	query := `SELECT payload,revision,target_user_id,fingerprint,active,invite_email FROM ` + table + ` WHERE project_id=$1 AND organization_id=$2 AND actor_id=$3 AND operation_key=$4`
	if lock {
		query += " FOR UPDATE"
	}
	var payload []byte
	var revision int64
	var target, fingerprint string
	var active bool
	var email sql.NullString
	err := db.QueryRowContext(ctx, query, scope.ProjectID, scope.OrganizationID, scope.ActorID, key).Scan(&payload, &revision, &target, &fingerprint, &active, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operation{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	var op domain.Operation
	if len(payload) > 16384 || json.Unmarshal(payload, &op) != nil || op.Scope != scope || op.Key != key || op.Revision != revision {
		return domain.Operation{}, domain.ErrUnavailable
	}
	if op.TargetUserID != target || op.Fingerprint != fingerprint || active != (op.Phase != domain.PhaseCompleted && op.Phase != domain.PhaseRejected) || email.Valid != (op.Kind == domain.CommandInvite) || (email.Valid && (op.Invitation == nil || op.Invitation.Email != email.String)) {
		return domain.Operation{}, domain.ErrUnavailable
	}
	return op, nil
}

func (r *Repository) Apply(ctx context.Context, scope domain.OperationScope, key string, revision int64, change domain.OperationChange) (domain.Operation, error) {
	if !validScope(scope) || !validKey(key) {
		return domain.Operation{}, domain.ErrInvalidRequest
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	defer tx.Rollback()
	op, err := read(ctx, tx, scope, key, true)
	if err != nil {
		return domain.Operation{}, err
	}
	if op.Revision != revision {
		return domain.Operation{}, domain.ErrConflict
	}
	next, err := domain.Transition(op, change)
	if err != nil {
		return domain.Operation{}, err
	}
	payload, err := json.Marshal(next)
	if err != nil || len(payload) > 16384 {
		return domain.Operation{}, domain.ErrInvalidRequest
	}
	active := next.Phase != domain.PhaseCompleted && next.Phase != domain.PhaseRejected
	_, err = tx.ExecContext(ctx, `UPDATE `+table+` SET payload=$1,revision=$2,active=$3 WHERE project_id=$4 AND organization_id=$5 AND actor_id=$6 AND operation_key=$7`, payload, next.Revision, active, scope.ProjectID, scope.OrganizationID, scope.ActorID, key)
	if err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	if err := tx.Commit(); err != nil {
		return domain.Operation{}, domain.ErrUnavailable
	}
	return next, nil
}
