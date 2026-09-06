package recordpersistence

import (
	"context"
	"task-processor/internal/authz"
	"task-processor/internal/listing/record"
	listingtask "task-processor/internal/listing/task"
	"time"

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
	ID              string
	OrganizationID  string
	OwnerUserID     string
	OperationID     string
	ProductKey      string
	SnapshotVersion uint64
	Country         string
	Language        string
	Payload         []byte
	CreatedAt       time.Time
	PayloadSize     int
}

const columns = `id, organization_id, owner_user_id, operation_id, product_key, snapshot_version, country, language, created_at, octet_length(payload) AS payload_size, CASE WHEN octet_length(payload) <= 2097152 THEN payload ELSE NULL END AS payload`

func load(tx *gorm.DB, where string, args ...any) (record.Record, error) {
	var r row
	result := tx.Raw("SELECT "+columns+" FROM listing_shein_records WHERE "+where, args...).Scan(&r)
	if result.Error != nil {
		return record.Record{}, result.Error
	}
	if result.RowsAffected != 1 {
		return record.Record{}, record.ErrNotFound
	}
	if r.PayloadSize > record.MaxPayloadBytes {
		return record.Record{}, record.ErrTooLarge
	}
	if r.PayloadSize == 0 || len(r.Payload) != r.PayloadSize {
		return record.Record{}, record.ErrUnavailable
	}
	return record.Record{ID: r.ID, OrganizationID: r.OrganizationID, OwnerUserID: r.OwnerUserID, OperationID: r.OperationID, Input: record.Input{ProductKey: r.ProductKey, SnapshotVersion: r.SnapshotVersion, Country: r.Country, Language: r.Language}, Payload: append([]byte(nil), r.Payload...), CreatedAt: r.CreatedAt, ReadAt: time.Now().UTC()}, nil
}
func (r *Repository) FindOperation(ctx context.Context, actor listingtask.Actor, operation string) (record.Record, error) {
	if err := ctx.Err(); err != nil {
		return record.Record{}, err
	}
	if listingtask.ValidateActor(actor) != nil || listingtask.ValidateTaskID(operation) != nil {
		return record.Record{}, record.ErrInvalid
	}
	got, err := load(r.db.WithContext(ctx), "organization_id = ? AND owner_user_id = ? AND operation_id = ?", actor.TenantID, actor.UserID, operation)
	if err != nil {
		return record.Record{}, err
	}
	if got.OrganizationID != actor.TenantID || got.OwnerUserID != actor.UserID || got.OperationID != operation {
		return record.Record{}, record.ErrNotFound
	}
	return got, nil
}
func (r *Repository) Insert(ctx context.Context, prepared record.Prepared) (record.Record, error) {
	proposed := prepared.Record()
	if _, err := uuid.Parse(proposed.ID); err != nil {
		return record.Record{}, record.ErrInvalid
	}
	if proposed.Input.Validate() != nil || proposed.OrganizationID == "" || proposed.OwnerUserID == "" || len(proposed.Payload) == 0 || len(proposed.Payload) > record.MaxPayloadBytes {
		return record.Record{}, record.ErrInvalid
	}
	var result record.Record
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := tx.Exec(`INSERT INTO listing_shein_records(id,organization_id,owner_user_id,operation_id,product_key,snapshot_version,country,language,payload) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT (organization_id,owner_user_id,operation_id) DO NOTHING`, proposed.ID, proposed.OrganizationID, proposed.OwnerUserID, proposed.OperationID, proposed.Input.ProductKey, proposed.Input.SnapshotVersion, proposed.Input.Country, proposed.Input.Language, proposed.Payload).Error
		if err != nil {
			return err
		}
		result, err = load(tx, "organization_id = ? AND owner_user_id = ? AND operation_id = ?", proposed.OrganizationID, proposed.OwnerUserID, proposed.OperationID)
		if err != nil {
			return err
		}
		if result.Input != proposed.Input {
			return record.ErrConflict
		}
		if result.OrganizationID != proposed.OrganizationID || result.OwnerUserID != proposed.OwnerUserID || result.OperationID != proposed.OperationID {
			return record.ErrNotFound
		}
		return ctx.Err()
	})
	// An error at COMMIT may mean outcome unknown: do not compensate/delete.
	if err != nil {
		return record.Record{}, err
	}
	return result.Clone(), nil
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
	if _, err := uuid.Parse(id); err != nil {
		return record.Record{}, record.ErrNotFound
	}
	where := "id = ? AND organization_id = ? AND owner_user_id <> ''"
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

var _ record.Store = (*Repository)(nil)
var _ record.Reader = (*Repository)(nil)
