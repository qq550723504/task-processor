package supplymarketpersistence

import (
	"context"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"
	"time"

	"gorm.io/gorm"
)

type fileRow struct {
	ID, OrganizationID, ActorID, MemberID, ObjectKey, ContentType, Digest string
	Size                                                                  int64
	CreatedAt                                                             time.Time
}

func (row fileRow) file() (supplymarket.PrivateFile, error) {
	file := supplymarket.PrivateFile{ID: row.ID, Owner: collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}, ObjectKey: row.ObjectKey, ContentType: row.ContentType, SHA256: row.Digest, Size: row.Size, CreatedAt: row.CreatedAt}
	if file.Validate() != nil {
		return supplymarket.PrivateFile{}, supplymarket.ErrUnavailable
	}
	return file, nil
}
func readUpload(db *gorm.DB, scope collection.Scope, id string) (supplymarket.PrivateFile, error) {
	if scope.Validate() != nil || !collection.ValidID(id) {
		return supplymarket.PrivateFile{}, supplymarket.ErrInvalid
	}
	var row fileRow
	result := db.Table("supply_market_private_uploads").Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&row)
	if result.Error == gorm.ErrRecordNotFound {
		return supplymarket.PrivateFile{}, supplymarket.ErrNotFound
	}
	if result.Error != nil {
		return supplymarket.PrivateFile{}, result.Error
	}
	return row.file()
}
func (r *Repository) ReadUpload(ctx context.Context, scope collection.Scope, id string) (supplymarket.PrivateFile, error) {
	return readUpload(r.db.WithContext(ctx), scope, id)
}
func (r *Repository) SaveUpload(ctx context.Context, file supplymarket.PrivateFile, guard supplymarket.Guard) (saved supplymarket.PrivateFile, err error) {
	if ctx == nil || file.Validate() != nil || guard == nil {
		return saved, supplymarket.ErrInvalid
	}
	err = r.write(ctx, func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "supply-file:"+file.ID).Error; err != nil {
			return err
		}
		var existing fileRow
		found := tx.Table("supply_market_private_uploads").Where("id=?", file.ID).Find(&existing)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected != 0 {
			var e error
			saved, e = existing.file()
			if e != nil {
				return e
			}
			if saved.Owner != file.Owner || saved.ObjectKey != file.ObjectKey || saved.SHA256 != file.SHA256 || saved.Size != file.Size || saved.ContentType != file.ContentType {
				return supplymarket.ErrConflict
			}
		} else {
			if err := tx.Exec("INSERT INTO supply_market_private_uploads(id,organization_id,actor_id,member_id,object_key,content_type,digest,size,created_at) VALUES(?,?,?,?,?,?,?,?,?)", file.ID, file.Owner.OrganizationID, file.Owner.ActorID, file.Owner.MemberID, file.ObjectKey, file.ContentType, file.SHA256, file.Size, file.CreatedAt).Error; err != nil {
				return err
			}
			saved = file
		}
		return guard(ctx)
	})
	if err != nil {
		return supplymarket.PrivateFile{}, err
	}
	return saved, nil
}
func (r *Repository) ReadAttachedFile(ctx context.Context, scope collection.Scope, platform, recordID, fileID string) (supplymarket.PrivateFile, error) {
	if !collection.ValidID(fileID) {
		return supplymarket.PrivateFile{}, supplymarket.ErrInvalid
	}
	record, err := r.ReadRecord(ctx, scope, platform, recordID)
	if err != nil {
		return supplymarket.PrivateFile{}, err
	}
	var attached bool
	if err := r.db.WithContext(ctx).Raw(`SELECT jsonb_exists(record_json->'fileIds',?) OR EXISTS(SELECT 1 FROM supply_market_events e WHERE e.record_id=r.id AND jsonb_exists(e.event_json->'fileIds',?)) FROM supply_market_records r WHERE r.id=?`, fileID, fileID, recordID).Scan(&attached).Error; err != nil {
		return supplymarket.PrivateFile{}, err
	}
	if !attached {
		return supplymarket.PrivateFile{}, supplymarket.ErrNotFound
	}
	return r.ReadUpload(ctx, record.Owner, fileID)
}

type fileVerifier struct {
	tx      *gorm.DB
	storage supplymarket.PrivateFileStorage
}

func NewFileVerifier(tx *gorm.DB, storage supplymarket.PrivateFileStorage) (FileVerifier, error) {
	if tx == nil || storage == nil {
		return nil, supplymarket.ErrUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, supplymarket.ErrUnavailable
	}
	return fileVerifier{tx, storage}, nil
}
func (v fileVerifier) Verify(ctx context.Context, scope collection.Scope, ids []string) error {
	if scope.Validate() != nil || len(ids) > 6 {
		return supplymarket.ErrInvalid
	}
	var total int64
	seen := map[string]bool{}
	for _, id := range ids {
		if !collection.ValidID(id) || seen[id] {
			return supplymarket.ErrInvalid
		}
		seen[id] = true
		file, err := readUpload(v.tx.WithContext(ctx), scope, id)
		if err != nil {
			return err
		}
		total += file.Size
		if total > 60<<20 {
			return supplymarket.ErrInvalid
		}
		object, err := v.storage.Inspect(ctx, file)
		if err != nil {
			return supplymarket.ErrUnavailable
		}
		if !object.Exists || object.SHA256 != file.SHA256 || object.ContentType != file.ContentType || object.Size != file.Size {
			return supplymarket.ErrConflict
		}
	}
	return ctx.Err()
}

var _ supplymarket.FileRepository = (*Repository)(nil)
