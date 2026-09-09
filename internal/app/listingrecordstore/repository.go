package recordpersistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	contract "task-processor/internal/marketplace/validator"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db   *gorm.DB
	auth record.Authorizer
}

func NewRepository(db *gorm.DB, auth record.Authorizer) (*Repository, error) {
	if db == nil || auth == nil || db.Dialector.Name() != "postgres" {
		return nil, record.ErrUnavailable
	}
	return &Repository{db, auth}, nil
}

type row struct {
	ID                    string
	OrganizationID        string
	OwnerUserID           string
	OperationID           string
	ProductKey            string
	SnapshotVersion       uint64
	StoreID               string
	Country               string
	Language              string
	Action                string
	InputHash             string
	ProductHash           string
	AssetInventoryVersion uint64
	AssetInventoryHash    string
	RuleRevision          string
	PolicyRevision        string
	PackageHash           string
	DiagnosticHash        string
	DiagnosticStatus      string
	Payload               []byte
	Diagnostic            []byte
	CreatedAt             time.Time
	PayloadSize           int
	DiagnosticSize        int
}

type operationRow struct {
	OrganizationID string
	OperationID    string
	OwnerUserID    string
	InputHash      string
	RecordID       string
}

type collectionRow struct {
	ID              string
	ProductKey      string
	SnapshotVersion uint64
	StoreID         string
	Country         string
	Language        string
	Action          string
	CreatedAt       time.Time
}

const columns = `id, organization_id, owner_user_id, operation_id, product_key, snapshot_version, store_id, country, language, action, input_hash, product_hash, asset_inventory_version, asset_inventory_hash, rule_revision, policy_revision, package_hash, diagnostic_hash, diagnostic_status, created_at, octet_length(payload) AS payload_size, CASE WHEN octet_length(payload) <= 2097152 THEN payload ELSE NULL END AS payload, octet_length(diagnostic) AS diagnostic_size, CASE WHEN octet_length(diagnostic) <= 2097152 THEN diagnostic ELSE NULL END AS diagnostic`

func load(tx *gorm.DB, where string, args ...any) (record.Record, error) {
	var stored row
	result := tx.Raw("SELECT "+columns+" FROM listing_shein_records WHERE "+where, args...).Scan(&stored)
	if result.Error != nil {
		return record.Record{}, result.Error
	}
	if result.RowsAffected != 1 {
		return record.Record{}, record.ErrNotFound
	}
	if stored.PayloadSize > record.MaxPayloadBytes || stored.DiagnosticSize > record.MaxPayloadBytes {
		return record.Record{}, record.ErrTooLarge
	}
	if stored.PayloadSize == 0 || len(stored.Payload) != stored.PayloadSize || stored.DiagnosticSize == 0 || len(stored.Diagnostic) != stored.DiagnosticSize {
		return record.Record{}, record.ErrUnavailable
	}
	input := record.Input{ProductKey: stored.ProductKey, SnapshotVersion: stored.SnapshotVersion, StoreID: stored.StoreID, Country: stored.Country, Language: stored.Language, Action: contract.Action(stored.Action)}
	if input.Validate() != nil || stored.AssetInventoryVersion != stored.SnapshotVersion ||
		!validDigest(stored.InputHash) || !validDigest(stored.ProductHash) || !validDigest(stored.AssetInventoryHash) ||
		!validDigest(stored.PackageHash) || !validDigest(stored.DiagnosticHash) ||
		contentDigest(stored.Payload) != stored.PackageHash || contentDigest(stored.Diagnostic) != stored.DiagnosticHash ||
		stored.RuleRevision == "" || stored.PolicyRevision == "" || stored.DiagnosticStatus == "" {
		return record.Record{}, record.ErrUnavailable
	}
	return record.Record{
		ID: stored.ID, OrganizationID: stored.OrganizationID, OwnerUserID: stored.OwnerUserID, OperationID: stored.OperationID,
		Input: input, InputHash: stored.InputHash, ProductHash: stored.ProductHash,
		AssetInventoryVersion: stored.AssetInventoryVersion, AssetInventoryHash: stored.AssetInventoryHash,
		RuleRevision: stored.RuleRevision, PolicyRevision: stored.PolicyRevision,
		PackageHash: stored.PackageHash, DiagnosticHash: stored.DiagnosticHash, DiagnosticStatus: contract.Status(stored.DiagnosticStatus),
		Payload: append([]byte(nil), stored.Payload...), Diagnostic: append([]byte(nil), stored.Diagnostic...),
		CreatedAt: stored.CreatedAt.UTC(), ReadAt: time.Now().UTC(),
	}, nil
}

func loadOperation(tx *gorm.DB, organizationID, operationID string) (operationRow, error) {
	var stored operationRow
	result := tx.Raw(`SELECT organization_id, operation_id, owner_user_id, input_hash, record_id FROM listing_shein_record_operations WHERE organization_id = ? AND operation_id = ?`, organizationID, operationID).Scan(&stored)
	if result.Error != nil {
		return operationRow{}, result.Error
	}
	if result.RowsAffected != 1 || stored.OrganizationID != organizationID || stored.OperationID != operationID {
		return operationRow{}, record.ErrNotFound
	}
	if stored.OwnerUserID == "" || !validDigest(stored.InputHash) {
		return operationRow{}, record.ErrUnavailable
	}
	parsed, err := uuid.Parse(stored.RecordID)
	if err != nil || parsed.String() != stored.RecordID {
		return operationRow{}, record.ErrUnavailable
	}
	return stored, nil
}

func (r *Repository) FindOperation(ctx context.Context, actor listingtask.Actor, operationID string) (record.Record, error) {
	if err := ctx.Err(); err != nil {
		return record.Record{}, err
	}
	if listingtask.ValidateActor(actor) != nil || listingtask.ValidateTaskID(operationID) != nil {
		return record.Record{}, record.ErrInvalid
	}
	operation, err := loadOperation(r.db.WithContext(ctx), actor.TenantID, operationID)
	if err != nil {
		return record.Record{}, err
	}
	stored, err := load(r.db.WithContext(ctx), "id = ? AND organization_id = ? AND operation_id = ?", operation.RecordID, actor.TenantID, operationID)
	if err != nil {
		return record.Record{}, err
	}
	if stored.InputHash != operation.InputHash || stored.OwnerUserID != operation.OwnerUserID {
		return record.Record{}, record.ErrUnavailable
	}
	return stored, nil
}

func (r *Repository) Insert(ctx context.Context, prepared record.Prepared) (record.Record, error) {
	proposed := prepared.Record()
	if err := validatePrepared(proposed); err != nil {
		return record.Record{}, err
	}
	var result record.Record
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		inserted := tx.Exec(`INSERT INTO listing_shein_record_operations(organization_id,operation_id,owner_user_id,input_hash,record_id) VALUES (?,?,?,?,?) ON CONFLICT (organization_id,operation_id) DO NOTHING`, proposed.OrganizationID, proposed.OperationID, proposed.OwnerUserID, proposed.InputHash, proposed.ID)
		if inserted.Error != nil {
			return inserted.Error
		}
		operation, err := loadOperation(tx, proposed.OrganizationID, proposed.OperationID)
		if err != nil {
			return err
		}
		if operation.OwnerUserID != proposed.OwnerUserID || operation.InputHash != proposed.InputHash {
			return record.ErrConflict
		}
		if inserted.RowsAffected == 0 {
			result, err = load(tx, "id = ? AND organization_id = ? AND owner_user_id = ? AND operation_id = ?", operation.RecordID, proposed.OrganizationID, proposed.OwnerUserID, proposed.OperationID)
			if err != nil {
				return err
			}
			if result.InputHash != proposed.InputHash {
				return record.ErrConflict
			}
			return ctx.Err()
		}
		if operation.RecordID != proposed.ID {
			return record.ErrUnavailable
		}
		created := tx.Exec(`INSERT INTO listing_shein_records(id,organization_id,owner_user_id,operation_id,product_key,snapshot_version,store_id,country,language,action,input_hash,product_hash,asset_inventory_version,asset_inventory_hash,rule_revision,policy_revision,package_hash,diagnostic_hash,diagnostic_status,payload,diagnostic) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			proposed.ID, proposed.OrganizationID, proposed.OwnerUserID, proposed.OperationID, proposed.Input.ProductKey, proposed.Input.SnapshotVersion,
			proposed.Input.StoreID, proposed.Input.Country, proposed.Input.Language, string(proposed.Input.Action), proposed.InputHash, proposed.ProductHash,
			proposed.AssetInventoryVersion, proposed.AssetInventoryHash, proposed.RuleRevision, proposed.PolicyRevision, proposed.PackageHash,
			proposed.DiagnosticHash, string(proposed.DiagnosticStatus), proposed.Payload, proposed.Diagnostic)
		if created.Error != nil {
			return created.Error
		}
		result, err = load(tx, "id = ? AND organization_id = ? AND owner_user_id = ? AND operation_id = ?", proposed.ID, proposed.OrganizationID, proposed.OwnerUserID, proposed.OperationID)
		if err != nil {
			return err
		}
		return ctx.Err()
	})
	// An error at COMMIT may mean outcome unknown: do not compensate/delete.
	if err != nil {
		return record.Record{}, err
	}
	return result.Clone(), nil
}

func validatePrepared(proposed record.Record) error {
	parsed, err := uuid.Parse(proposed.ID)
	if err != nil || parsed.String() != proposed.ID || proposed.Input.Validate() != nil || proposed.OrganizationID == "" || proposed.OwnerUserID == "" || proposed.OperationID == "" ||
		proposed.AssetInventoryVersion != proposed.Input.SnapshotVersion || len(proposed.Payload) == 0 || len(proposed.Payload) > record.MaxPayloadBytes ||
		len(proposed.Diagnostic) == 0 || len(proposed.Diagnostic) > record.MaxPayloadBytes || !validDigest(proposed.InputHash) ||
		!validDigest(proposed.ProductHash) || !validDigest(proposed.AssetInventoryHash) || !validDigest(proposed.PackageHash) ||
		!validDigest(proposed.DiagnosticHash) || proposed.PackageHash != contentDigest(proposed.Payload) || proposed.DiagnosticHash != contentDigest(proposed.Diagnostic) ||
		proposed.RuleRevision == "" || proposed.PolicyRevision == "" || proposed.DiagnosticStatus == "" {
		return record.ErrInvalid
	}
	return nil
}

func (r *Repository) ReadOfflinePackage(ctx context.Context, actor listingtask.Actor, id string) (record.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, record.Timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return record.Record{}, err
	}
	if listingtask.ValidateActor(actor) != nil || !r.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminRead) {
		return record.Record{}, record.ErrForbidden
	}
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return record.Record{}, record.ErrNotFound
	}
	where := "id = ? AND organization_id = ? AND owner_user_id <> '' AND store_id IS NOT NULL"
	args := []any{id, actor.TenantID}
	admin := r.auth.IsTenantAdmin(actor.UserID, actor.Roles)
	if !admin {
		where += " AND owner_user_id = ?"
		args = append(args, actor.UserID)
	}
	got, err := load(r.db.WithContext(ctx), where, args...)
	if err != nil {
		return record.Record{}, err
	}
	if got.ID != id || got.OrganizationID != actor.TenantID || got.OwnerUserID == "" || (got.OwnerUserID != actor.UserID && !admin) {
		return record.Record{}, record.ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return record.Record{}, err
	}
	return got.Clone(), nil
}

func (r *Repository) List(ctx context.Context, actor listingtask.Actor, request record.PageRequest) (record.Page, error) {
	if err := ctx.Err(); err != nil {
		return record.Page{}, err
	}
	if listingtask.ValidateActor(actor) != nil || !r.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminRead) {
		return record.Page{}, record.ErrForbidden
	}
	if err := request.Validate(); err != nil {
		return record.Page{}, err
	}
	where := "organization_id = ? AND owner_user_id <> ''"
	args := []any{actor.TenantID}
	admin := r.auth.IsTenantAdmin(actor.UserID, actor.Roles)
	if !admin {
		where += " AND owner_user_id = ?"
		args = append(args, actor.UserID)
	}
	if request.Cursor != nil {
		var visible bool
		anchorArgs := append(append([]any(nil), args...), request.Cursor.CreatedAt, request.Cursor.ID)
		anchorSQL := "SELECT EXISTS(SELECT 1 FROM listing_shein_records WHERE " + where + " AND created_at = ? AND id = ?::uuid)"
		if err := r.db.WithContext(ctx).Raw(anchorSQL, anchorArgs...).Row().Scan(&visible); err != nil {
			return record.Page{}, err
		}
		if !visible {
			return record.Page{}, record.ErrInvalid
		}
		where += " AND (created_at, id) < (?, ?::uuid)"
		args = append(args, request.Cursor.CreatedAt, request.Cursor.ID)
	}
	query := "SELECT id, product_key, snapshot_version, store_id, country, language, action, created_at FROM listing_shein_records WHERE " + where + " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, request.Limit+1)
	var rows []collectionRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return record.Page{}, err
	}
	if err := ctx.Err(); err != nil {
		return record.Page{}, err
	}
	more := len(rows) > request.Limit
	if more {
		rows = rows[:request.Limit]
	}
	items := make([]record.CollectionItem, 0, len(rows))
	for _, item := range rows {
		parsed, err := uuid.Parse(item.ID)
		input := record.Input{ProductKey: item.ProductKey, SnapshotVersion: item.SnapshotVersion, StoreID: item.StoreID, Country: item.Country, Language: item.Language, Action: contract.Action(item.Action)}
		if err != nil || parsed.String() != item.ID || input.Validate() != nil || item.CreatedAt.IsZero() {
			return record.Page{}, record.ErrUnavailable
		}
		items = append(items, record.CollectionItem{ID: item.ID, Input: input, CreatedAt: item.CreatedAt.UTC()})
	}
	page := record.Page{Items: items}
	if more {
		last := items[len(items)-1]
		page.NextCursor = &record.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt}
	}
	return page, nil
}

func contentDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validDigest(value string) bool { return contract.ValidContentDigest(value) }

var _ record.Store = (*Repository)(nil)
var _ record.Reader = (*Repository)(nil)
var _ record.CollectionReader = (*Repository)(nil)
