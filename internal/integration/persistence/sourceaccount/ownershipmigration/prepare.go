package ownershipmigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	PreparedContractVersion       = 1
	PreparedStage                 = "prepared_only"
	preparedSourceSchema          = "public"
	maxPrepareDeadline            = 10 * time.Minute
	maxPrepareTextFieldBytes      = 64 * 1024
	maxPrepareSourceSnapshotBytes = 64 * 1024 * 1024
	maxPrepareRequestBytes        = 64 * 1024 * 1024
	maxPrepareSnapshotJSONBytes   = 128 * 1024 * 1024
	maxPreparedReceiptBytes       = 128 * 1024 * 1024
)

var (
	ErrIdempotencyConflict = errors.New("source account ownership migration idempotency conflict")
	ErrSourceDrift         = errors.New("source account ownership migration source drift")
	ErrTargetConflict      = errors.New("source account ownership migration target conflict")
	ErrTargetDrift         = errors.New("source account ownership migration target drift")
	ErrResourceLimit       = errors.New("source account ownership migration resource limit exceeded")
)

//go:embed prepared_schema.sql
var preparedSchemaSQL string

// PrepareRequest binds B1 to one complete A receipt and an operator-selected
// source identity. It is an offline migration input, not a runtime API contract.
type PrepareRequest struct {
	ContractVersion int     `json:"contract_version"`
	IdempotencyKey  string  `json:"idempotency_key"`
	SourceID        string  `json:"source_id"`
	Preflight       Receipt `json:"preflight"`
}

// PreparedAccountEvidence is the immutable B1 result. LegacyTenantID is retained
// only in the migration receipt; it is deliberately absent from the target table.
type PreparedAccountEvidence struct {
	ID               int64      `json:"id"`
	LegacyTenantID   int64      `json:"legacy_tenant_id"`
	OrganizationID   string     `json:"organization_id"`
	Platform         string     `json:"platform"`
	Label            *string    `json:"label"`
	ProfileRef       string     `json:"profile_ref"`
	ProfileDirectory string     `json:"profile_directory"`
	ProxyRef         *string    `json:"proxy_ref"`
	LoginURL         *string    `json:"login_url"`
	Status           int16      `json:"status"`
	Deleted          int16      `json:"deleted"`
	LastVerifiedAt   *time.Time `json:"last_verified_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type PreparedReceipt struct {
	ContractVersion int                       `json:"contract_version"`
	IdempotencyKey  string                    `json:"idempotency_key"`
	Stage           string                    `json:"stage"`
	RequestSHA256   string                    `json:"request_sha256"`
	PreflightSHA256 string                    `json:"preflight_sha256"`
	SourceID        string                    `json:"source_id"`
	SourceDatabase  string                    `json:"source_database"`
	SourceSchema    string                    `json:"source_schema"`
	SourceSHA256    string                    `json:"source_sha256"`
	TargetSHA256    string                    `json:"target_sha256"`
	AccountCount    int                       `json:"account_count"`
	Accounts        []PreparedAccountEvidence `json:"accounts"`
	PreparedAt      time.Time                 `json:"prepared_at"`
}

type validatedPrepareRequest struct {
	request       PrepareRequest
	requestSHA256 string
	evidenceByID  map[int64]AccountEvidence
}

type prepareResourceLimits struct {
	maxFieldBytes        int64
	maxSourceTextBytes   int64
	maxRequestJSONBytes  int64
	maxSnapshotJSONBytes int64
	maxReceiptJSONBytes  int64
}

func defaultPrepareResourceLimits() prepareResourceLimits {
	return prepareResourceLimits{
		maxFieldBytes:        maxPrepareTextFieldBytes,
		maxSourceTextBytes:   maxPrepareSourceSnapshotBytes,
		maxRequestJSONBytes:  maxPrepareRequestBytes,
		maxSnapshotJSONBytes: maxPrepareSnapshotJSONBytes,
		maxReceiptJSONBytes:  maxPreparedReceiptBytes,
	}
}

// Preparer is intentionally not wired into startup, HTTP, worker or runtime
// repository construction. B2 owns the operator-facing operation gate.
type Preparer struct {
	db *sql.DB

	// Test-only fault and concurrency seams. Production construction leaves them nil.
	afterLocks        func() error
	afterTargetInsert func(context.Context, *sql.Tx, int) error
	beforeCommit      func() error
	afterCommit       func() error
	resourceLimits    prepareResourceLimits
}

func InstallPreparedSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("source account ownership migration database is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin source account ownership migration schema install: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, preparedSchemaSQL); err != nil {
		return fmt.Errorf("install source account ownership migration schema: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `LOCK TABLE public.organization_source_accounts, public.source_account_ownership_migration_receipts IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock source account ownership migration schema: %w", err)
	}
	if err = validatePreparedSchema(ctx, tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit source account ownership migration schema install: %w", err)
	}
	return nil
}

func NewPreparer(db *sql.DB) (*Preparer, error) {
	if db == nil {
		return nil, errors.New("source account ownership migration database is nil")
	}
	return &Preparer{
		db:             db,
		resourceLimits: defaultPrepareResourceLimits(),
	}, nil
}

func (p *Preparer) Prepare(ctx context.Context, request PrepareRequest) (receipt PreparedReceipt, replayed bool, err error) {
	validated, err := validatePrepareRequestWithLimits(ctx, request, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("begin source account ownership migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = setPrepareTimeouts(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = lockPrepareTables(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if p.afterLocks != nil {
		if err = p.afterLocks(); err != nil {
			return PreparedReceipt{}, false, err
		}
	}
	if err = ctx.Err(); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = validatePreparedSchema(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = verifyPrepareDatabase(ctx, tx, request.Preflight.AccountObservation.Database); err != nil {
		return PreparedReceipt{}, false, err
	}
	stored, found, err := readPreparedReceiptRow(ctx, tx, request.ContractVersion, request.IdempotencyKey, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if found && stored.RequestSHA256 != validated.requestSHA256 {
		return PreparedReceipt{}, false, ErrIdempotencyConflict
	}
	if found {
		if err = validateStoredReceiptIdentity(stored, request); err != nil {
			return PreparedReceipt{}, false, err
		}
	}
	var target []PreparedAccountEvidence
	if !found {
		target, err = readPrepareTarget(ctx, tx, p.resourceLimits)
		if err != nil {
			return PreparedReceipt{}, false, err
		}
		if len(target) != 0 {
			return PreparedReceipt{}, false, ErrTargetConflict
		}
		otherReceipts, countErr := countPreparedReceipts(ctx, tx)
		if countErr != nil {
			return PreparedReceipt{}, false, countErr
		}
		if otherReceipts != 0 {
			return PreparedReceipt{}, false, ErrTargetConflict
		}
	}
	source, err := readPrepareSource(ctx, tx, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = compareSourceToPreflight(source, validated); err != nil {
		return PreparedReceipt{}, false, err
	}
	sourceSHA256, err := digestJSON(source)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	expectedTarget := buildPreparedAccounts(source, validated.evidenceByID)

	if found {
		if stored.SourceSHA256 != sourceSHA256 {
			return PreparedReceipt{}, false, ErrSourceDrift
		}
		if !preparedAccountsEqual(stored.Accounts, expectedTarget) {
			return PreparedReceipt{}, false, ErrTargetDrift
		}
		target, readErr := readPrepareTarget(ctx, tx, p.resourceLimits)
		if readErr != nil {
			return PreparedReceipt{}, false, readErr
		}
		if err = validateStoredTarget(stored, target, p.resourceLimits.maxSnapshotJSONBytes); err != nil {
			return PreparedReceipt{}, false, err
		}
		if err = tx.Commit(); err != nil {
			return PreparedReceipt{}, false, fmt.Errorf("commit source account ownership migration replay: %w", err)
		}
		return stored, true, nil
	}

	target = expectedTarget
	targetSHA256, err := digestPreparedTarget(target, p.resourceLimits.maxSnapshotJSONBytes)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	for index, account := range target {
		if err = insertPrepareTarget(ctx, tx, account); err != nil {
			return PreparedReceipt{}, false, err
		}
		if p.afterTargetInsert != nil {
			if err = p.afterTargetInsert(ctx, tx, index+1); err != nil {
				return PreparedReceipt{}, false, err
			}
		}
		if err = ctx.Err(); err != nil {
			return PreparedReceipt{}, false, err
		}
	}
	preparedAt, err := readPrepareTransactionTimestamp(ctx, tx)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	receipt = PreparedReceipt{
		ContractVersion: request.ContractVersion,
		IdempotencyKey:  request.IdempotencyKey,
		Stage:           PreparedStage,
		RequestSHA256:   validated.requestSHA256,
		PreflightSHA256: request.Preflight.Digest,
		SourceID:        request.SourceID,
		SourceDatabase:  request.Preflight.AccountObservation.Database,
		SourceSchema:    preparedSourceSchema,
		SourceSHA256:    sourceSHA256,
		TargetSHA256:    targetSHA256,
		AccountCount:    len(target),
		Accounts:        target,
		PreparedAt:      preparedAt,
	}
	if err = insertPreparedReceipt(ctx, tx, receipt, p.resourceLimits); err != nil {
		return PreparedReceipt{}, false, err
	}
	if p.beforeCommit != nil {
		if err = p.beforeCommit(); err != nil {
			return PreparedReceipt{}, false, err
		}
	}
	if err = ctx.Err(); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("commit source account ownership migration: %w", err)
	}
	// COMMIT is the state boundary. Do not reinterpret a durable result from a
	// context cancellation observed after this point.
	if p.afterCommit != nil {
		if err = p.afterCommit(); err != nil {
			return receipt, false, err
		}
	}
	return receipt, false, nil
}

func (p *Preparer) ReadPreparedReceipt(ctx context.Context, request PrepareRequest) (PreparedReceipt, bool, error) {
	validated, err := validatePrepareRequestWithLimits(ctx, request, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("begin prepared receipt read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = setPrepareTimeouts(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = lockPreparedSchemaReadTables(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = validatePreparedSchema(ctx, tx); err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = verifyPrepareDatabase(ctx, tx, request.Preflight.AccountObservation.Database); err != nil {
		return PreparedReceipt{}, false, err
	}
	stored, found, err := readPreparedReceiptRow(ctx, tx, request.ContractVersion, request.IdempotencyKey, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if !found {
		target, readErr := readPrepareTarget(ctx, tx, p.resourceLimits)
		if readErr != nil {
			return PreparedReceipt{}, false, readErr
		}
		if len(target) != 0 {
			return PreparedReceipt{}, false, ErrTargetConflict
		}
		return PreparedReceipt{}, false, nil
	}
	if stored.RequestSHA256 != validated.requestSHA256 {
		return PreparedReceipt{}, false, ErrIdempotencyConflict
	}
	if err = validateStoredReceiptIdentity(stored, request); err != nil {
		return PreparedReceipt{}, false, err
	}
	source, err := readPrepareSource(ctx, tx, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = compareSourceToPreflight(source, validated); err != nil {
		return PreparedReceipt{}, false, err
	}
	sourceSHA256, err := digestJSON(source)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if stored.SourceSHA256 != sourceSHA256 {
		return PreparedReceipt{}, false, ErrSourceDrift
	}
	expectedTarget := buildPreparedAccounts(source, validated.evidenceByID)
	if !preparedAccountsEqual(stored.Accounts, expectedTarget) {
		return PreparedReceipt{}, false, ErrTargetDrift
	}
	target, err := readPrepareTarget(ctx, tx, p.resourceLimits)
	if err != nil {
		return PreparedReceipt{}, false, err
	}
	if err = validateStoredTarget(stored, target, p.resourceLimits.maxSnapshotJSONBytes); err != nil {
		return PreparedReceipt{}, false, err
	}
	return stored, true, nil
}

func validatePrepareRequest(ctx context.Context, request PrepareRequest) (validatedPrepareRequest, error) {
	return validatePrepareRequestWithLimits(ctx, request, defaultPrepareResourceLimits())
}

func validatePrepareRequestWithLimits(ctx context.Context, request PrepareRequest, limits prepareResourceLimits) (validatedPrepareRequest, error) {
	if err := ctx.Err(); err != nil {
		return validatedPrepareRequest{}, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return validatedPrepareRequest{}, errors.New("source account ownership migration deadline required")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > maxPrepareDeadline {
		return validatedPrepareRequest{}, errors.New("source account ownership migration deadline must be within ten minutes")
	}
	if request.ContractVersion != PreparedContractVersion {
		return validatedPrepareRequest{}, errors.New("unsupported source account ownership migration contract version")
	}
	if request.IdempotencyKey == "" || len([]byte(request.IdempotencyKey)) > 128 || strings.TrimSpace(request.IdempotencyKey) != request.IdempotencyKey {
		return validatedPrepareRequest{}, errors.New("invalid source account ownership migration idempotency key")
	}
	if request.SourceID == "" || strings.TrimSpace(request.SourceID) != request.SourceID || utf8.RuneCountInString(request.SourceID) > 256 || request.SourceID != request.Preflight.AccountObservation.SourceID {
		return validatedPrepareRequest{}, errors.New("source account ownership migration source identity mismatch")
	}
	if request.Preflight.Version != 1 || request.Preflight.Stage != "preflight_only" || request.Preflight.SnapshotConsistency != "separate_non_atomic_snapshots" {
		return validatedPrepareRequest{}, errors.New("invalid source account ownership preflight contract")
	}
	if request.Preflight.AccountObservation.Database == "" || strings.TrimSpace(request.Preflight.AccountObservation.Database) != request.Preflight.AccountObservation.Database || utf8.RuneCountInString(request.Preflight.AccountObservation.Database) > 128 ||
		request.Preflight.MetadataObservation.SourceID == "" || strings.TrimSpace(request.Preflight.MetadataObservation.SourceID) != request.Preflight.MetadataObservation.SourceID || utf8.RuneCountInString(request.Preflight.MetadataObservation.SourceID) > 256 ||
		request.Preflight.MetadataObservation.Database == "" || strings.TrimSpace(request.Preflight.MetadataObservation.Database) != request.Preflight.MetadataObservation.Database || utf8.RuneCountInString(request.Preflight.MetadataObservation.Database) > 128 ||
		request.Preflight.AccountObservation.At.IsZero() || request.Preflight.MetadataObservation.At.IsZero() {
		return validatedPrepareRequest{}, errors.New("incomplete source account ownership preflight observation")
	}
	if len(request.Preflight.Accounts) == 0 || len(request.Preflight.Accounts) > MaxRows || len(request.Preflight.Metadata) > MaxRows {
		return validatedPrepareRequest{}, errors.New("invalid source account ownership preflight row count")
	}
	owners := make(map[int64]string, len(request.Preflight.Metadata))
	seenOrganizations := make(map[string]struct{}, len(request.Preflight.Metadata))
	previousOrganization := ""
	for _, metadata := range request.Preflight.Metadata {
		if err := ctx.Err(); err != nil {
			return validatedPrepareRequest{}, err
		}
		if metadata.OrganizationID == "" || strings.TrimSpace(metadata.OrganizationID) != metadata.OrganizationID || utf8.RuneCountInString(metadata.OrganizationID) > 128 || metadata.OrganizationID <= previousOrganization {
			return validatedPrepareRequest{}, errors.New("invalid or unordered Organization metadata")
		}
		if len(metadata.Value) > maxPrepareTextFieldBytes {
			return validatedPrepareRequest{}, fmt.Errorf("%w: Organization mapping value exceeds field byte budget", ErrResourceLimit)
		}
		previousOrganization = metadata.OrganizationID
		if _, exists := seenOrganizations[metadata.OrganizationID]; exists {
			return validatedPrepareRequest{}, errors.New("ambiguous Organization metadata")
		}
		seenOrganizations[metadata.OrganizationID] = struct{}{}
		legacyTenantID, parseErr := strconv.ParseInt(string(metadata.Value), 10, 64)
		if parseErr != nil || legacyTenantID <= 0 || strconv.FormatInt(legacyTenantID, 10) != string(metadata.Value) {
			return validatedPrepareRequest{}, errors.New("invalid Organization mapping value")
		}
		if metadata.OwnerRemoved {
			continue
		}
		if _, exists := owners[legacyTenantID]; exists {
			return validatedPrepareRequest{}, errors.New("ambiguous Organization mapping")
		}
		owners[legacyTenantID] = metadata.OrganizationID
	}

	evidenceByID := make(map[int64]AccountEvidence, len(request.Preflight.Accounts))
	var previousID int64
	for _, evidence := range request.Preflight.Accounts {
		if err := ctx.Err(); err != nil {
			return validatedPrepareRequest{}, err
		}
		account := evidence.Previous
		if account.ID <= previousID || account.TenantID <= 0 || account.Platform != "1688" || strings.TrimSpace(account.ProfileRef) == "" || utf8.RuneCountInString(account.ProfileRef) > 256 {
			return validatedPrepareRequest{}, fmt.Errorf("invalid or unordered source account %d", account.ID)
		}
		previousID = account.ID
		if evidence.OrganizationID == "" || evidence.OrganizationID != owners[account.TenantID] {
			return validatedPrepareRequest{}, fmt.Errorf("invalid Organization owner for source account %d", account.ID)
		}
		if strings.TrimSpace(evidence.ProfileDirectory) == "" || utf8.RuneCountInString(evidence.ProfileDirectory) > 1024 || !filepath.IsAbs(evidence.ProfileDirectory) {
			return validatedPrepareRequest{}, fmt.Errorf("invalid profile directory for source account %d", account.ID)
		}
		evidenceByID[account.ID] = evidence
	}
	if _, err := measurePreflightReceiptJSON(request.Preflight, limits.maxRequestJSONBytes); err != nil {
		return validatedPrepareRequest{}, err
	}
	if request.Preflight.Digest == "" || request.Preflight.Digest != receiptDigest(request.Preflight) {
		return validatedPrepareRequest{}, errors.New("invalid source account ownership preflight digest")
	}
	fingerprint := struct {
		ContractVersion int               `json:"contract_version"`
		SourceID        string            `json:"source_id"`
		PreflightSHA256 string            `json:"preflight_sha256"`
		Accounts        []AccountEvidence `json:"accounts"`
	}{request.ContractVersion, request.SourceID, request.Preflight.Digest, request.Preflight.Accounts}
	if _, err := measureJSONValue(fingerprint, limits.maxRequestJSONBytes, "source account ownership migration request fingerprint"); err != nil {
		return validatedPrepareRequest{}, err
	}
	requestSHA256, err := digestJSON(fingerprint)
	if err != nil {
		return validatedPrepareRequest{}, err
	}
	return validatedPrepareRequest{request: request, requestSHA256: requestSHA256, evidenceByID: evidenceByID}, nil
}

func measurePreflightReceiptJSON(receipt Receipt, limit int64) (int64, error) {
	canonical := receipt
	canonical.Digest = ""
	canonical.AccountObservation.At = time.Time{}
	canonical.MetadataObservation.At = time.Time{}
	accountBytes, err := measureJSONArray(canonical.Accounts, limit, "source account ownership preflight accounts")
	if err != nil {
		return 0, err
	}
	metadataBytes, err := measureJSONArray(canonical.Metadata, limit, "source account ownership preflight metadata")
	if err != nil {
		return 0, err
	}
	canonical.Accounts = nil
	canonical.Metadata = nil
	baseBytes, err := measureJSONValue(canonical, limit, "source account ownership preflight receipt")
	if err != nil {
		return 0, err
	}
	return replaceNullJSONArrays(baseBytes, limit, "source account ownership preflight receipt", accountBytes, metadataBytes)
}

func measurePreparedReceiptJSON(receipt PreparedReceipt, limit int64) (int64, error) {
	accountBytes, err := measureJSONArray(receipt.Accounts, limit, "prepared source account receipt accounts")
	if err != nil {
		return 0, err
	}
	withoutAccounts := receipt
	withoutAccounts.Accounts = nil
	baseBytes, err := measureJSONValue(withoutAccounts, limit, "prepared source account receipt")
	if err != nil {
		return 0, err
	}
	return replaceNullJSONArrays(baseBytes, limit, "prepared source account receipt", accountBytes)
}

func measureJSONArray[T any](values []T, limit int64, owner string) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("%w: invalid %s JSON byte limit", ErrResourceLimit, owner)
	}
	if values == nil {
		return 4, nil
	}
	total := int64(2)
	for index, value := range values {
		payload, err := json.Marshal(value)
		if err != nil {
			return 0, fmt.Errorf("measure %s JSON: %w", owner, err)
		}
		addition := int64(len(payload))
		if index != 0 {
			addition++
		}
		if addition > limit-total {
			return 0, fmt.Errorf("%w: %s JSON exceeds %d bytes", ErrResourceLimit, owner, limit)
		}
		total += addition
	}
	return total, nil
}

func measureJSONValue(value any, limit int64, owner string) (int64, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("%w: invalid %s JSON byte limit", ErrResourceLimit, owner)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("measure %s JSON: %w", owner, err)
	}
	if int64(len(payload)) > limit {
		return 0, fmt.Errorf("%w: %s JSON exceeds %d bytes", ErrResourceLimit, owner, limit)
	}
	return int64(len(payload)), nil
}

func replaceNullJSONArrays(baseBytes, limit int64, owner string, arrayBytes ...int64) (int64, error) {
	nullBytes := int64(4 * len(arrayBytes))
	if baseBytes < nullBytes {
		return 0, fmt.Errorf("measure %s JSON: invalid null-array base", owner)
	}
	total := baseBytes - nullBytes
	for _, size := range arrayBytes {
		if size > limit-total {
			return 0, fmt.Errorf("%w: %s JSON exceeds %d bytes", ErrResourceLimit, owner, limit)
		}
		total += size
	}
	return total, nil
}

type prepareSourceAccount struct {
	ID             int64      `json:"id"`
	LegacyTenantID int64      `json:"legacy_tenant_id"`
	Platform       string     `json:"platform"`
	Label          *string    `json:"label"`
	ProfileRef     string     `json:"profile_ref"`
	ProxyRef       *string    `json:"proxy_ref"`
	LoginURL       *string    `json:"login_url"`
	Status         int16      `json:"status"`
	Deleted        int16      `json:"deleted"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func readPrepareSource(ctx context.Context, tx *sql.Tx, limits prepareResourceLimits) ([]prepareSourceAccount, error) {
	if err := validatePrepareTextResourceBounds(ctx, tx, `SELECT count(*), COALESCE(MAX(GREATEST(COALESCE(octet_length(platform), 0), COALESCE(octet_length(label), 0), COALESCE(octet_length(profile_ref), 0), COALESCE(octet_length(proxy_ref), 0), COALESCE(octet_length(login_url), 0))), 0), COALESCE(SUM(COALESCE(octet_length(platform), 0) + COALESCE(octet_length(label), 0) + COALESCE(octet_length(profile_ref), 0) + COALESCE(octet_length(proxy_ref), 0) + COALESCE(octet_length(login_url), 0)), 0) FROM public.source_account WHERE LOWER(platform) = '1688'`, limits, "legacy source account"); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, platform, label, profile_ref, proxy_ref, login_url, status, deleted, last_verified_at, created_at, updated_at FROM public.source_account WHERE LOWER(platform) = '1688' ORDER BY id LIMIT $1`, MaxRows+1)
	if err != nil {
		return nil, fmt.Errorf("read legacy source accounts: %w", err)
	}
	defer rows.Close()
	accounts := make([]prepareSourceAccount, 0)
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var account prepareSourceAccount
		var label, proxyRef, loginURL sql.NullString
		var lastVerifiedAt sql.NullTime
		if err = rows.Scan(&account.ID, &account.LegacyTenantID, &account.Platform, &label, &account.ProfileRef, &proxyRef, &loginURL, &account.Status, &account.Deleted, &lastVerifiedAt, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan legacy source account: %w", err)
		}
		account.Label = nullableString(label)
		account.ProxyRef = nullableString(proxyRef)
		account.LoginURL = nullableString(loginURL)
		account.LastVerifiedAt = nullableTime(lastVerifiedAt)
		normalizePreparedTimes(&account.CreatedAt, &account.UpdatedAt, account.LastVerifiedAt)
		accounts = append(accounts, account)
		if len(accounts) > MaxRows {
			return nil, errors.New("legacy source account inventory exceeds row limit")
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy source accounts: %w", err)
	}
	if _, err = measureJSONArray(accounts, limits.maxSnapshotJSONBytes, "legacy source account snapshot"); err != nil {
		return nil, err
	}
	return accounts, nil
}

func validatePrepareTextResourceBounds(ctx context.Context, tx *sql.Tx, query string, limits prepareResourceLimits, owner string) error {
	if limits.maxFieldBytes <= 0 || limits.maxSourceTextBytes <= 0 {
		return fmt.Errorf("%w: invalid %s byte limits", ErrResourceLimit, owner)
	}
	var rowCount, largestFieldBytes, snapshotBytes int64
	if err := tx.QueryRowContext(ctx, query).Scan(&rowCount, &largestFieldBytes, &snapshotBytes); err != nil {
		return fmt.Errorf("measure %s text: %w", owner, err)
	}
	if rowCount > MaxRows {
		return fmt.Errorf("%w: %s row count %d exceeds %d", ErrResourceLimit, owner, rowCount, MaxRows)
	}
	if largestFieldBytes > limits.maxFieldBytes {
		return fmt.Errorf("%w: %s field bytes %d exceeds %d", ErrResourceLimit, owner, largestFieldBytes, limits.maxFieldBytes)
	}
	if snapshotBytes > limits.maxSourceTextBytes {
		return fmt.Errorf("%w: %s text bytes %d exceeds %d", ErrResourceLimit, owner, snapshotBytes, limits.maxSourceTextBytes)
	}
	return nil
}

func compareSourceToPreflight(source []prepareSourceAccount, validated validatedPrepareRequest) error {
	if len(source) != len(validated.request.Preflight.Accounts) {
		return ErrSourceDrift
	}
	for index, account := range source {
		evidence := validated.request.Preflight.Accounts[index]
		previous := evidence.Previous
		if account.ID != previous.ID || account.LegacyTenantID != previous.TenantID || account.Platform != previous.Platform || account.ProfileRef != previous.ProfileRef || account.Status != previous.Status || account.Deleted != previous.Deleted {
			return ErrSourceDrift
		}
	}
	return nil
}

func buildPreparedAccounts(source []prepareSourceAccount, evidenceByID map[int64]AccountEvidence) []PreparedAccountEvidence {
	target := make([]PreparedAccountEvidence, 0, len(source))
	for _, account := range source {
		evidence := evidenceByID[account.ID]
		target = append(target, PreparedAccountEvidence{
			ID: account.ID, LegacyTenantID: account.LegacyTenantID,
			OrganizationID: evidence.OrganizationID, Platform: "1688", Label: account.Label,
			ProfileRef: account.ProfileRef, ProfileDirectory: evidence.ProfileDirectory,
			ProxyRef: account.ProxyRef, LoginURL: account.LoginURL, Status: account.Status, Deleted: account.Deleted,
			LastVerifiedAt: account.LastVerifiedAt, CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
		})
	}
	return target
}

func readPrepareTarget(ctx context.Context, tx *sql.Tx, limits prepareResourceLimits) ([]PreparedAccountEvidence, error) {
	if err := validatePrepareTextResourceBounds(ctx, tx, `SELECT count(*), COALESCE(MAX(GREATEST(COALESCE(octet_length(organization_id), 0), COALESCE(octet_length(platform), 0), COALESCE(octet_length(label), 0), COALESCE(octet_length(profile_ref), 0), COALESCE(octet_length(profile_directory), 0), COALESCE(octet_length(proxy_ref), 0), COALESCE(octet_length(login_url), 0))), 0), COALESCE(SUM(COALESCE(octet_length(organization_id), 0) + COALESCE(octet_length(platform), 0) + COALESCE(octet_length(label), 0) + COALESCE(octet_length(profile_ref), 0) + COALESCE(octet_length(profile_directory), 0) + COALESCE(octet_length(proxy_ref), 0) + COALESCE(octet_length(login_url), 0)), 0) FROM public.organization_source_accounts`, limits, "Organization source account target"); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, organization_id, platform, label, profile_ref, profile_directory, proxy_ref, login_url, status, deleted, last_verified_at, created_at, updated_at FROM public.organization_source_accounts ORDER BY id LIMIT $1`, MaxRows+1)
	if err != nil {
		return nil, fmt.Errorf("read Organization source account target: %w", err)
	}
	defer rows.Close()
	target := make([]PreparedAccountEvidence, 0)
	for rows.Next() {
		var account PreparedAccountEvidence
		var label, proxyRef, loginURL sql.NullString
		var lastVerifiedAt sql.NullTime
		if err = rows.Scan(&account.ID, &account.OrganizationID, &account.Platform, &label, &account.ProfileRef, &account.ProfileDirectory, &proxyRef, &loginURL, &account.Status, &account.Deleted, &lastVerifiedAt, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan Organization source account target: %w", err)
		}
		account.Label = nullableString(label)
		account.ProxyRef = nullableString(proxyRef)
		account.LoginURL = nullableString(loginURL)
		account.LastVerifiedAt = nullableTime(lastVerifiedAt)
		normalizePreparedTimes(&account.CreatedAt, &account.UpdatedAt, account.LastVerifiedAt)
		target = append(target, account)
		if len(target) > MaxRows {
			return nil, errors.New("Organization source account target exceeds row limit")
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Organization source account target: %w", err)
	}
	if _, err = measureJSONArray(target, limits.maxSnapshotJSONBytes, "Organization source account target snapshot"); err != nil {
		return nil, err
	}
	return target, nil
}

func insertPrepareTarget(ctx context.Context, tx *sql.Tx, account PreparedAccountEvidence) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO public.organization_source_accounts (id, organization_id, platform, label, profile_ref, profile_directory, proxy_ref, login_url, status, deleted, last_verified_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		account.ID, account.OrganizationID, account.Platform, account.Label, account.ProfileRef, account.ProfileDirectory, account.ProxyRef, account.LoginURL, account.Status, account.Deleted, account.LastVerifiedAt, account.CreatedAt, account.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert Organization source account target %d: %w", account.ID, err)
	}
	return nil
}

func insertPreparedReceipt(ctx context.Context, tx *sql.Tx, receipt PreparedReceipt, limits prepareResourceLimits) error {
	measuredBytes, err := measurePreparedReceiptJSON(receipt, limits.maxReceiptJSONBytes)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("marshal prepared receipt: %w", err)
	}
	if int64(len(resultJSON)) != measuredBytes {
		return errors.New("prepared receipt JSON size measurement mismatch")
	}
	var persistedJSONBytes int64
	err = tx.QueryRowContext(ctx, `INSERT INTO public.source_account_ownership_migration_receipts (contract_version, idempotency_key, stage, request_sha256, preflight_sha256, source_id, source_database, source_schema, source_sha256, target_sha256, account_count, result_json, prepared_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING octet_length(result_json::text)`,
		receipt.ContractVersion, receipt.IdempotencyKey, receipt.Stage, receipt.RequestSHA256, receipt.PreflightSHA256, receipt.SourceID, receipt.SourceDatabase, receipt.SourceSchema, receipt.SourceSHA256, receipt.TargetSHA256, receipt.AccountCount, resultJSON, receipt.PreparedAt).Scan(&persistedJSONBytes)
	if err != nil {
		return fmt.Errorf("insert source account ownership migration receipt: %w", err)
	}
	if persistedJSONBytes > limits.maxReceiptJSONBytes {
		return fmt.Errorf("%w: persisted prepared receipt bytes %d exceeds %d", ErrResourceLimit, persistedJSONBytes, limits.maxReceiptJSONBytes)
	}
	return nil
}

func readPrepareTransactionTimestamp(ctx context.Context, tx *sql.Tx) (time.Time, error) {
	var preparedAtUnknownZone time.Time
	if err := tx.QueryRowContext(ctx, "SELECT transaction_timestamp()").Scan(&preparedAtUnknownZone); err != nil {
		return time.Time{}, fmt.Errorf("read source account ownership migration transaction timestamp: %w", err)
	}
	return preparedAtUnknownZone.UTC(), nil
}

func readPreparedReceiptRow(ctx context.Context, tx *sql.Tx, version int, key string, limits prepareResourceLimits) (PreparedReceipt, bool, error) {
	var resultBytes int64
	err := tx.QueryRowContext(ctx, `SELECT octet_length(result_json::text) FROM public.source_account_ownership_migration_receipts WHERE contract_version = $1 AND idempotency_key = $2`, version, key).Scan(&resultBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return PreparedReceipt{}, false, nil
	}
	if err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("measure source account ownership migration receipt: %w", err)
	}
	if resultBytes > limits.maxReceiptJSONBytes {
		return PreparedReceipt{}, false, fmt.Errorf("%w: persisted prepared receipt bytes %d exceeds %d", ErrResourceLimit, resultBytes, limits.maxReceiptJSONBytes)
	}
	var resultJSON []byte
	var row PreparedReceipt
	err = tx.QueryRowContext(ctx, `SELECT contract_version, idempotency_key, stage, request_sha256, preflight_sha256, source_id, source_database, source_schema, source_sha256, target_sha256, account_count, result_json, prepared_at FROM public.source_account_ownership_migration_receipts WHERE contract_version = $1 AND idempotency_key = $2`, version, key).
		Scan(&row.ContractVersion, &row.IdempotencyKey, &row.Stage, &row.RequestSHA256, &row.PreflightSHA256, &row.SourceID, &row.SourceDatabase, &row.SourceSchema, &row.SourceSHA256, &row.TargetSHA256, &row.AccountCount, &resultJSON, &row.PreparedAt)
	if err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("read source account ownership migration receipt: %w", err)
	}
	row.PreparedAt = row.PreparedAt.UTC()
	var result PreparedReceipt
	if err = json.Unmarshal(resultJSON, &result); err != nil {
		return PreparedReceipt{}, false, fmt.Errorf("decode source account ownership migration receipt: %w", err)
	}
	if _, err = measurePreparedReceiptJSON(result, limits.maxReceiptJSONBytes); err != nil {
		return PreparedReceipt{}, false, err
	}
	if !preparedReceiptsEqual(rowWithoutAccounts(row), rowWithoutAccounts(result)) || result.AccountCount != len(result.Accounts) {
		return PreparedReceipt{}, false, fmt.Errorf("source account ownership migration receipt row mismatch: row=%#v result=%#v", rowWithoutAccounts(row), rowWithoutAccounts(result))
	}
	return result, true, nil
}

func rowWithoutAccounts(receipt PreparedReceipt) PreparedReceipt {
	receipt.Accounts = nil
	return receipt
}

func validateStoredReceiptIdentity(receipt PreparedReceipt, request PrepareRequest) error {
	if receipt.ContractVersion != request.ContractVersion ||
		receipt.IdempotencyKey != request.IdempotencyKey ||
		receipt.Stage != PreparedStage ||
		receipt.PreflightSHA256 != request.Preflight.Digest ||
		receipt.SourceID != request.SourceID ||
		receipt.SourceDatabase != request.Preflight.AccountObservation.Database ||
		receipt.SourceSchema != preparedSourceSchema {
		return errors.New("source account ownership migration receipt identity mismatch")
	}
	return nil
}

func countPreparedReceipts(ctx context.Context, tx *sql.Tx) (int, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM public.source_account_ownership_migration_receipts`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count source account ownership migration receipts: %w", err)
	}
	return count, nil
}

func validateStoredTarget(receipt PreparedReceipt, target []PreparedAccountEvidence, maxJSONBytes int64) error {
	if len(target) != receipt.AccountCount || len(receipt.Accounts) != receipt.AccountCount {
		return ErrTargetDrift
	}
	for index := range target {
		// The numeric owner exists only in receipt evidence. Populate it solely
		// for receipt equality; the target digest below deliberately excludes it.
		target[index].LegacyTenantID = receipt.Accounts[index].LegacyTenantID
	}
	targetSHA256, err := digestPreparedTarget(target, maxJSONBytes)
	if err != nil {
		return err
	}
	if receipt.TargetSHA256 != targetSHA256 || !preparedAccountsEqual(receipt.Accounts, target) {
		return ErrTargetDrift
	}
	return nil
}

func setPrepareTimeouts(ctx context.Context, tx *sql.Tx) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("source account ownership migration deadline required")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.DeadlineExceeded
	}
	milliseconds := remaining.Milliseconds()
	if milliseconds < 1 {
		milliseconds = 1
	}
	timeout := strconv.FormatInt(milliseconds, 10) + "ms"
	for _, setting := range []string{"lock_timeout", "statement_timeout", "idle_in_transaction_session_timeout"} {
		if _, err := tx.ExecContext(ctx, `SELECT set_config($1, $2, true)`, setting, timeout); err != nil {
			return fmt.Errorf("set PostgreSQL %s: %w", setting, err)
		}
	}
	return nil
}

func lockPrepareTables(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`LOCK TABLE public.source_account IN SHARE MODE`,
		`LOCK TABLE public.organization_source_accounts IN SHARE ROW EXCLUSIVE MODE`,
		`LOCK TABLE public.source_account_ownership_migration_receipts IN SHARE ROW EXCLUSIVE MODE`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("lock source account ownership migration table: %w", err)
		}
	}
	return nil
}

func verifyPrepareDatabase(ctx context.Context, tx *sql.Tx, want string) error {
	var got string
	if err := tx.QueryRowContext(ctx, `SELECT current_database()`).Scan(&got); err != nil {
		return fmt.Errorf("read source account database identity: %w", err)
	}
	if got != want {
		return fmt.Errorf("source account database identity mismatch: got %q", got)
	}
	return nil
}

func digestJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal source account ownership migration digest: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func preparedAccountsEqual(left, right []PreparedAccountEvidence) bool {
	return reflect.DeepEqual(left, right)
}

func preparedReceiptsEqual(left, right PreparedReceipt) bool {
	return reflect.DeepEqual(left, right)
}

func digestPreparedTarget(accounts []PreparedAccountEvidence, maxJSONBytes int64) (string, error) {
	target := make([]struct {
		ID               int64      `json:"id"`
		OrganizationID   string     `json:"organization_id"`
		Platform         string     `json:"platform"`
		Label            *string    `json:"label"`
		ProfileRef       string     `json:"profile_ref"`
		ProfileDirectory string     `json:"profile_directory"`
		ProxyRef         *string    `json:"proxy_ref"`
		LoginURL         *string    `json:"login_url"`
		Status           int16      `json:"status"`
		Deleted          int16      `json:"deleted"`
		LastVerifiedAt   *time.Time `json:"last_verified_at"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
	}, 0, len(accounts))
	for _, account := range accounts {
		target = append(target, struct {
			ID               int64      `json:"id"`
			OrganizationID   string     `json:"organization_id"`
			Platform         string     `json:"platform"`
			Label            *string    `json:"label"`
			ProfileRef       string     `json:"profile_ref"`
			ProfileDirectory string     `json:"profile_directory"`
			ProxyRef         *string    `json:"proxy_ref"`
			LoginURL         *string    `json:"login_url"`
			Status           int16      `json:"status"`
			Deleted          int16      `json:"deleted"`
			LastVerifiedAt   *time.Time `json:"last_verified_at"`
			CreatedAt        time.Time  `json:"created_at"`
			UpdatedAt        time.Time  `json:"updated_at"`
		}{
			ID: account.ID, OrganizationID: account.OrganizationID, Platform: account.Platform,
			Label: account.Label, ProfileRef: account.ProfileRef, ProfileDirectory: account.ProfileDirectory,
			ProxyRef: account.ProxyRef, LoginURL: account.LoginURL, Status: account.Status, Deleted: account.Deleted,
			LastVerifiedAt: account.LastVerifiedAt, CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
		})
	}
	if _, err := measureJSONArray(target, maxJSONBytes, "Organization source account target digest"); err != nil {
		return "", err
	}
	return digestJSON(target)
}

func normalizePreparedTimes(createdAt, updatedAt *time.Time, lastVerifiedAt *time.Time) {
	*createdAt = createdAt.UTC()
	*updatedAt = updatedAt.UTC()
	if lastVerifiedAt != nil {
		*lastVerifiedAt = lastVerifiedAt.UTC()
	}
}
