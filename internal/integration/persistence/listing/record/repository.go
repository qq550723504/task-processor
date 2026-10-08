package recordpersistence

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/listing/record"
	"task-processor/internal/product/collection"
)

type Repository struct{ db *gorm.DB }

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if VerifySchema(ctx, db) != nil {
		return nil, record.ErrUnavailable
	}
	return &Repository{db}, nil
}

type recordRow struct {
	OrganizationID, ActorID, MemberID     string
	ID, TargetID, SourceID, StoreID, Site string
	Revision                              int64
	InputHash, BodyHash                   string
	BodyJSON                              []byte
	CreatedAt                             time.Time
}

func (recordRow) TableName() string { return "listing_target_records" }

type headRow struct {
	OrganizationID, ActorID, MemberID            string
	ID, SourceID, StoreID, Site, CurrentRecordID string
	Revision                                     int64
}

func (headRow) TableName() string { return "listing_preparation_targets" }

type commandRow struct {
	OrganizationID, ActorID, MemberID string
	CommandKey, RecordID, InputHash   string
}

func (commandRow) TableName() string { return "listing_target_record_commands" }
func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
func validateRecord(scope collection.Scope, value record.TargetRecord) bool {
	return scope.Validate() == nil && collection.ValidID(value.ID) && collection.ValidID(value.Source.ID) && collection.ValidID(value.Input.StoreID) &&
		value.TargetID == record.TargetIdentity(scope, value.Source.ID, value.Input.StoreID) && value.Revision > 0 && value.Revision == value.Input.ExpectedRevision+1 &&
		value.Input.SourceID == value.Source.ID && value.EffectiveVersion == value.Input.EffectiveVersion && value.ApplyReceiptID == value.Input.ApplyReceiptID &&
		value.Merchant.OrganizationID == scope.OrganizationID && value.Merchant.StoreID == value.Input.StoreID && value.Merchant.Site == "shein-us" &&
		validHash(value.ProductHash) && validHash(value.InventoryHash) && validHash(value.RulesHash) && !value.CreatedAt.IsZero()
}
func decodeRecord(scope collection.Scope, row recordRow) (record.TargetRecord, error) {
	if row.OrganizationID != scope.OrganizationID || row.ActorID != scope.ActorID || row.MemberID != scope.MemberID {
		return record.TargetRecord{}, record.ErrNotFound
	}
	var value record.TargetRecord
	if len(row.BodyJSON) > record.MaxPayloadBytes || json.Unmarshal(row.BodyJSON, &value) != nil || !validateRecord(scope, value) ||
		collection.Digest(value) != row.BodyHash || collection.Digest(value.Input) != row.InputHash || value.ID != row.ID || value.TargetID != row.TargetID ||
		value.Source.ID != row.SourceID || value.Input.StoreID != row.StoreID || value.Merchant.Site != row.Site || value.Revision != row.Revision || !value.CreatedAt.Equal(row.CreatedAt) {
		return record.TargetRecord{}, record.ErrUnavailable
	}
	return value, nil
}
func readRecord(db *gorm.DB, scope collection.Scope, id string) (recordRow, record.TargetRecord, error) {
	var row recordRow
	result := db.Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return row, record.TargetRecord{}, record.ErrNotFound
	}
	if result.Error != nil {
		return row, record.TargetRecord{}, record.ErrUnavailable
	}
	value, err := decodeRecord(scope, row)
	return row, value, err
}
func readCommand(db *gorm.DB, scope collection.Scope, key string) (record.TargetReceipt, string, error) {
	if scope.Validate() != nil || !collection.ValidID(key) {
		return record.TargetReceipt{}, "", record.ErrInvalid
	}
	var command commandRow
	result := db.Where("organization_id=? AND actor_id=? AND member_id=? AND command_key=?", scope.OrganizationID, scope.ActorID, scope.MemberID, key).Take(&command)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return record.TargetReceipt{}, "", record.ErrNotFound
	}
	if result.Error != nil {
		return record.TargetReceipt{}, "", record.ErrUnavailable
	}
	row, value, err := readRecord(db, scope, command.RecordID)
	if err != nil {
		return record.TargetReceipt{}, "", err
	}
	if row.InputHash != command.InputHash || value.ID != collection.StableID(scope.OrganizationID, scope.ActorID, "supply-record", key) {
		return record.TargetReceipt{}, "", record.ErrUnavailable
	}
	return record.TargetReceipt{Record: value, Replayed: true}, command.InputHash, nil
}
func (r *Repository) FindTargetCommand(ctx context.Context, scope collection.Scope, key, hash string) (record.TargetReceipt, error) {
	if ctx == nil || !validHash(hash) {
		return record.TargetReceipt{}, record.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, record.Timeout)
	defer cancel()
	value, savedHash, err := readCommand(r.db.WithContext(ctx), scope, key)
	if err == nil && savedHash != hash {
		return record.TargetReceipt{}, record.ErrConflict
	}
	return value, err
}
func (r *Repository) ReadTargetCommand(ctx context.Context, scope collection.Scope, key string) (record.TargetReceipt, error) {
	if ctx == nil {
		return record.TargetReceipt{}, record.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, record.Timeout)
	defer cancel()
	value, _, err := readCommand(r.db.WithContext(ctx), scope, key)
	return value, err
}
func (r *Repository) ReadTargetHead(ctx context.Context, scope collection.Scope, id string) (record.TargetRecord, error) {
	if ctx == nil || scope.Validate() != nil || !collection.ValidID(id) {
		return record.TargetRecord{}, record.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, record.Timeout)
	defer cancel()
	var head headRow
	result := r.db.WithContext(ctx).Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&head)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return record.TargetRecord{}, record.ErrNotFound
	}
	if result.Error != nil {
		return record.TargetRecord{}, record.ErrUnavailable
	}
	_, value, err := readRecord(r.db.WithContext(ctx), scope, head.CurrentRecordID)
	if err != nil {
		return record.TargetRecord{}, err
	}
	if value.TargetID != head.ID || value.Revision != head.Revision || value.Source.ID != head.SourceID || value.Input.StoreID != head.StoreID || value.Merchant.Site != head.Site {
		return record.TargetRecord{}, record.ErrUnavailable
	}
	return value, nil
}
func (r *Repository) SaveTarget(ctx context.Context, proof record.TargetPrepared) (record.TargetReceipt, error) {
	scope, key, hash, value, err := proof.Read(ctx)
	if err != nil {
		return record.TargetReceipt{}, err
	}
	if !validateRecord(scope, value) || value.ID != collection.StableID(scope.OrganizationID, scope.ActorID, "supply-record", key) {
		return record.TargetReceipt{}, record.ErrInvalid
	}
	value.CreatedAt = value.CreatedAt.UTC().Truncate(time.Microsecond)
	ctx, cancel := context.WithTimeout(ctx, record.Timeout)
	defer cancel()
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return record.TargetReceipt{}, record.ErrUnavailable
	}
	defer tx.Rollback()
	for _, lock := range []string{collection.Digest([]string{scope.OrganizationID, scope.ActorID, "supply-record-command", key}), collection.Digest([]string{scope.OrganizationID, scope.ActorID, "supply-target", value.TargetID})} {
		if tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", lock).Error != nil {
			return record.TargetReceipt{}, record.ErrUnavailable
		}
	}
	if prior, savedHash, lookupErr := readCommand(tx, scope, key); lookupErr == nil {
		if savedHash != hash {
			return record.TargetReceipt{}, record.ErrConflict
		}
		return prior, nil
	} else if !errors.Is(lookupErr, record.ErrNotFound) {
		return record.TargetReceipt{}, lookupErr
	}
	var head headRow
	result := tx.Where("organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, value.TargetID).Take(&head)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return record.TargetReceipt{}, record.ErrUnavailable
	}
	existing := result.Error == nil
	if existing && (head.MemberID != scope.MemberID || head.Revision != value.Input.ExpectedRevision) || !existing && value.Input.ExpectedRevision != 0 {
		return record.TargetReceipt{}, record.ErrConflict
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > record.MaxPayloadBytes {
		return record.TargetReceipt{}, record.ErrTooLarge
	}
	row := recordRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ID: value.ID, TargetID: value.TargetID, SourceID: value.Source.ID, StoreID: value.Input.StoreID, Site: value.Merchant.Site, Revision: value.Revision, InputHash: hash, BodyHash: collection.Digest(value), BodyJSON: raw, CreatedAt: value.CreatedAt}
	if tx.Create(&row).Error != nil {
		return record.TargetReceipt{}, record.ErrUnavailable
	}
	if existing {
		changed := tx.Model(&headRow{}).Where("organization_id=? AND actor_id=? AND member_id=? AND id=? AND revision=?", scope.OrganizationID, scope.ActorID, scope.MemberID, value.TargetID, value.Input.ExpectedRevision).Updates(map[string]any{"current_record_id": value.ID, "revision": value.Revision})
		if changed.Error != nil {
			return record.TargetReceipt{}, record.ErrUnavailable
		}
		if changed.RowsAffected != 1 {
			return record.TargetReceipt{}, record.ErrConflict
		}
	} else {
		head = headRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ID: value.TargetID, SourceID: value.Source.ID, StoreID: value.Input.StoreID, Site: value.Merchant.Site, CurrentRecordID: value.ID, Revision: value.Revision}
		if tx.Create(&head).Error != nil {
			return record.TargetReceipt{}, record.ErrUnavailable
		}
	}
	command := commandRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, CommandKey: key, RecordID: value.ID, InputHash: hash}
	if tx.Create(&command).Error != nil {
		return record.TargetReceipt{}, record.ErrUnavailable
	}
	if _, _, _, _, err = proof.Read(ctx); err != nil {
		return record.TargetReceipt{}, err
	}
	if tx.Commit().Error != nil {
		return record.TargetReceipt{}, record.ErrTargetUnknown
	}
	return record.TargetReceipt{Record: value}, nil
}

var _ record.TargetRepository = (*Repository)(nil)
