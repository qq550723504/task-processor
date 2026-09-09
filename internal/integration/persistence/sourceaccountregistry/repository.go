package sourceaccountregistry

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/sourceaccountregistry"
)

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, sourceaccountregistry.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), sourceaccountregistry.Timeout)
	defer cancel()
	if err := VerifySchema(ctx, db); err != nil {
		return nil, fmt.Errorf("%w: %v", sourceaccountregistry.ErrUnavailable, err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Run(ctx context.Context, operation sourceaccountregistry.Operation, fn func(sourceaccountregistry.Transaction) (sourceaccountregistry.Account, error)) (sourceaccountregistry.MutationResult, error) {
	if r == nil || r.db == nil || fn == nil || !validOperation(operation) {
		return sourceaccountregistry.MutationResult{}, sourceaccountregistry.ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return sourceaccountregistry.MutationResult{}, err
	}
	db := r.db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if db.Error != nil {
		return sourceaccountregistry.MutationResult{}, mapError(ctx, db.Error)
	}
	committed := false
	defer func() {
		if !committed {
			_ = db.Rollback().Error
		}
	}()
	if err := db.Exec(`SELECT pg_advisory_xact_lock(?)`, advisoryLock("operation", operation.Scope.OrganizationID, operation.Scope.ActorSubject, operation.Key)).Error; err != nil {
		return sourceaccountregistry.MutationResult{}, mapError(ctx, err)
	}
	tx := &transaction{db: db, operation: operation}
	account, err := fn(tx)
	if err != nil {
		return sourceaccountregistry.MutationResult{}, mapError(ctx, err)
	}
	if !tx.replayed && (!tx.completed || !reflect.DeepEqual(tx.completedAccount, account)) {
		return sourceaccountregistry.MutationResult{}, sourceaccountregistry.ErrUnavailable
	}
	if tx.replayed && tx.completed {
		return sourceaccountregistry.MutationResult{}, sourceaccountregistry.ErrUnavailable
	}
	if err := db.Commit().Error; err != nil {
		return sourceaccountregistry.MutationResult{}, sourceaccountregistry.ErrOutcomeUnknown
	}
	committed = true
	return sourceaccountregistry.MutationResult{Account: account, Replayed: tx.replayed}, nil
}

func (r *Repository) Read(ctx context.Context, scope sourceaccountregistry.Scope, id string) (sourceaccountregistry.Account, error) {
	if r == nil || r.db == nil {
		return sourceaccountregistry.Account{}, sourceaccountregistry.ErrUnavailable
	}
	var row accountRow
	err := r.db.WithContext(ctx).Table(resourceTable).Where("organization_id = ? AND id = ?", scope.OrganizationID, id).Take(&row).Error
	if err != nil {
		return sourceaccountregistry.Account{}, mapError(ctx, err)
	}
	return row.account()
}

func (r *Repository) List(ctx context.Context, scope sourceaccountregistry.Scope, request sourceaccountregistry.PageRequest) (sourceaccountregistry.Page, error) {
	if r == nil || r.db == nil || request.Limit < 1 || request.Limit > sourceaccountregistry.MaxPageLimit {
		return sourceaccountregistry.Page{}, sourceaccountregistry.ErrUnavailable
	}
	query := r.db.WithContext(ctx).Table(resourceTable).Where("organization_id = ?", scope.OrganizationID)
	if request.After != nil {
		query = query.Where("(created_at > ?) OR (created_at = ? AND id > ?)", request.After.CreatedAt.UTC(), request.After.CreatedAt.UTC(), request.After.ID)
	}
	var rows []accountRow
	if err := query.Order("created_at ASC, id ASC").Limit(request.Limit + 1).Find(&rows).Error; err != nil {
		return sourceaccountregistry.Page{}, mapError(ctx, err)
	}
	hasNext := len(rows) > request.Limit
	if hasNext {
		rows = rows[:request.Limit]
	}
	page := sourceaccountregistry.Page{Items: make([]sourceaccountregistry.Account, 0, len(rows))}
	for _, row := range rows {
		account, err := row.account()
		if err != nil {
			return sourceaccountregistry.Page{}, err
		}
		page.Items = append(page.Items, account)
	}
	if hasNext && len(page.Items) != 0 {
		last := page.Items[len(page.Items)-1]
		page.Next = &sourceaccountregistry.PagePosition{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

type transaction struct {
	db               *gorm.DB
	operation        sourceaccountregistry.Operation
	replayChecked    bool
	replayed         bool
	capacityChecked  bool
	completed        bool
	completedAccount sourceaccountregistry.Account
}

func (t *transaction) Replay() (sourceaccountregistry.Account, bool, error) {
	if t.replayChecked {
		return sourceaccountregistry.Account{}, false, sourceaccountregistry.ErrUnavailable
	}
	t.replayChecked = true
	var operation operationRow
	err := t.db.Table(operationTable).Where("organization_id = ? AND actor_subject = ? AND idempotency_key = ?", t.operation.Scope.OrganizationID, t.operation.Scope.ActorSubject, t.operation.Key).Take(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sourceaccountregistry.Account{}, false, nil
	}
	if err != nil {
		return sourceaccountregistry.Account{}, false, mapError(t.db.Statement.Context, err)
	}
	if operation.Kind != string(t.operation.Kind) || operation.RequestFingerprint != t.operation.Fingerprint {
		return sourceaccountregistry.Account{}, false, sourceaccountregistry.ErrIdempotencyConflict
	}
	var account accountRow
	if err := t.db.Table(resourceTable).Where("organization_id = ? AND id = ?", t.operation.Scope.OrganizationID, operation.AccountID).Take(&account).Error; err != nil {
		return sourceaccountregistry.Account{}, false, mapError(t.db.Statement.Context, err)
	}
	result, err := account.account()
	if err != nil {
		return sourceaccountregistry.Account{}, false, err
	}
	t.replayed = true
	return result, true, nil
}

func (t *transaction) CountForCreate() (int, error) {
	if !t.replayChecked || t.replayed || t.capacityChecked {
		return 0, sourceaccountregistry.ErrUnavailable
	}
	t.capacityChecked = true
	if err := t.db.Exec(`SELECT pg_advisory_xact_lock(?)`, advisoryLock("capacity", t.operation.Scope.OrganizationID)).Error; err != nil {
		return 0, mapError(t.db.Statement.Context, err)
	}
	var count int64
	if err := t.db.Table(resourceTable).Where("organization_id = ?", t.operation.Scope.OrganizationID).Count(&count).Error; err != nil {
		return 0, mapError(t.db.Statement.Context, err)
	}
	if count < 0 || count > int64(^uint(0)>>1) {
		return 0, sourceaccountregistry.ErrUnavailable
	}
	return int(count), nil
}

func (t *transaction) Insert(account sourceaccountregistry.Account) error {
	if !t.capacityChecked || t.replayed || t.completed || account.OrganizationID != t.operation.Scope.OrganizationID || account.CreatedBy != t.operation.Scope.ActorSubject || account.UpdatedBy != t.operation.Scope.ActorSubject {
		return sourceaccountregistry.ErrUnavailable
	}
	if err := account.Validate(); err != nil {
		return err
	}
	return mapError(t.db.Statement.Context, t.db.Table(resourceTable).Create(accountRowFrom(account)).Error)
}

func (t *transaction) LoadForUpdate(id string) (sourceaccountregistry.Account, error) {
	if !t.replayChecked || t.replayed || t.completed {
		return sourceaccountregistry.Account{}, sourceaccountregistry.ErrUnavailable
	}
	var row accountRow
	err := t.db.Raw(`SELECT organization_id, id, platform, display_name, management_status, connection_status, version, created_by, updated_by, created_at, updated_at FROM public.source_account_resources WHERE organization_id = ? AND id = ? FOR UPDATE`, t.operation.Scope.OrganizationID, id).Scan(&row).Error
	if err != nil {
		return sourceaccountregistry.Account{}, mapError(t.db.Statement.Context, err)
	}
	if row.ID == "" {
		return sourceaccountregistry.Account{}, sourceaccountregistry.ErrNotFound
	}
	return row.account()
}

func (t *transaction) Save(account sourceaccountregistry.Account, expectedVersion int64) error {
	if t.replayed || t.completed || account.OrganizationID != t.operation.Scope.OrganizationID || account.UpdatedBy != t.operation.Scope.ActorSubject || account.Version != expectedVersion+1 {
		return sourceaccountregistry.ErrUnavailable
	}
	if err := account.Validate(); err != nil {
		return err
	}
	result := t.db.Table(resourceTable).Where("organization_id = ? AND id = ? AND version = ?", account.OrganizationID, account.ID, expectedVersion).Updates(map[string]any{
		"management_status": account.ManagementStatus, "version": account.Version, "updated_by": account.UpdatedBy, "updated_at": account.UpdatedAt,
	})
	if result.Error != nil {
		return mapError(t.db.Statement.Context, result.Error)
	}
	if result.RowsAffected != 1 {
		return sourceaccountregistry.ErrVersionConflict
	}
	return nil
}

func (t *transaction) Complete(account sourceaccountregistry.Account) error {
	if t.replayed || t.completed || account.OrganizationID != t.operation.Scope.OrganizationID {
		return sourceaccountregistry.ErrUnavailable
	}
	row := operationRow{
		OrganizationID: account.OrganizationID, ActorSubject: t.operation.Scope.ActorSubject,
		IdempotencyKey: t.operation.Key, Kind: string(t.operation.Kind), RequestFingerprint: t.operation.Fingerprint,
		AccountID: account.ID, ResultingVersion: account.Version, ResultingManagementStatus: string(account.ManagementStatus), CreatedAt: account.UpdatedAt.UTC(),
	}
	if err := t.db.Table(operationTable).Create(&row).Error; err != nil {
		return mapError(t.db.Statement.Context, err)
	}
	t.completed = true
	t.completedAccount = account
	return nil
}

type accountRow struct {
	OrganizationID   string    `gorm:"column:organization_id"`
	ID               string    `gorm:"column:id"`
	Platform         string    `gorm:"column:platform"`
	DisplayName      string    `gorm:"column:display_name"`
	ManagementStatus string    `gorm:"column:management_status"`
	ConnectionStatus string    `gorm:"column:connection_status"`
	Version          int64     `gorm:"column:version"`
	CreatedBy        string    `gorm:"column:created_by"`
	UpdatedBy        string    `gorm:"column:updated_by"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func accountRowFrom(account sourceaccountregistry.Account) *accountRow {
	return &accountRow{
		OrganizationID: account.OrganizationID, ID: account.ID, Platform: string(account.Platform), DisplayName: account.DisplayName,
		ManagementStatus: string(account.ManagementStatus), ConnectionStatus: string(account.ConnectionStatus), Version: account.Version,
		CreatedBy: account.CreatedBy, UpdatedBy: account.UpdatedBy, CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
	}
}

func (row accountRow) account() (sourceaccountregistry.Account, error) {
	account := sourceaccountregistry.Account{
		ID: row.ID, OrganizationID: row.OrganizationID, Platform: sourceaccountregistry.Platform(row.Platform), DisplayName: row.DisplayName,
		ManagementStatus: sourceaccountregistry.ManagementStatus(row.ManagementStatus), ConnectionStatus: sourceaccountregistry.ConnectionStatus(row.ConnectionStatus),
		Version: row.Version, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
	if err := account.Validate(); err != nil {
		return sourceaccountregistry.Account{}, sourceaccountregistry.ErrUnavailable
	}
	return account, nil
}

type operationRow struct {
	OrganizationID            string    `gorm:"column:organization_id"`
	ActorSubject              string    `gorm:"column:actor_subject"`
	IdempotencyKey            string    `gorm:"column:idempotency_key"`
	Kind                      string    `gorm:"column:kind"`
	RequestFingerprint        string    `gorm:"column:request_fingerprint"`
	AccountID                 string    `gorm:"column:account_id"`
	ResultingVersion          int64     `gorm:"column:resulting_version"`
	ResultingManagementStatus string    `gorm:"column:resulting_management_status"`
	CreatedAt                 time.Time `gorm:"column:created_at"`
}

func validOperation(operation sourceaccountregistry.Operation) bool {
	return operation.Scope.OrganizationID != "" && operation.Scope.ActorSubject != "" && operation.Key != "" && operation.Kind != "" && fingerprintPattern.MatchString(operation.Fingerprint)
}

func advisoryLock(parts ...string) int64 {
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(length[:], uint64(len(part)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(part))
	}
	return int64(binary.BigEndian.Uint64(hash.Sum(nil)[:8]))
}

func mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sourceaccountregistry.ErrInvalid) || errors.Is(err, sourceaccountregistry.ErrForbidden) || errors.Is(err, sourceaccountregistry.ErrNotFound) || errors.Is(err, sourceaccountregistry.ErrIdempotencyConflict) || errors.Is(err, sourceaccountregistry.ErrVersionConflict) || errors.Is(err, sourceaccountregistry.ErrInvalidTransition) || errors.Is(err, sourceaccountregistry.ErrResourceLimitReached) || errors.Is(err, sourceaccountregistry.ErrUnavailable) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sourceaccountregistry.ErrNotFound
	}
	return sourceaccountregistry.ErrUnavailable
}
