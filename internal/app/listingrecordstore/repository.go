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

type collectionRow struct {
	ID              string
	ProductKey      string
	SnapshotVersion uint64
	Country         string
	Language        string
	CreatedAt       time.Time
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

	query := "SELECT id, product_key, snapshot_version, country, language, created_at FROM listing_shein_records WHERE " + where + " ORDER BY created_at DESC, id DESC LIMIT ?"
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
		input := record.Input{ProductKey: item.ProductKey, SnapshotVersion: item.SnapshotVersion, Country: item.Country, Language: item.Language}
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

var _ record.Store = (*Repository)(nil)
var _ record.Reader = (*Repository)(nil)
var _ record.CollectionReader = (*Repository)(nil)
