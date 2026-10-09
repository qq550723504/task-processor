package dataservicepersistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/dataservice"
	"task-processor/internal/product/collection"
)

type CredentialRepository struct{ db *gorm.DB }

func NewCredentialRepository(ctx context.Context, db *gorm.DB) (*CredentialRepository, error) {
	if err := VerifySchema(ctx, db); err != nil {
		return nil, err
	}
	return &CredentialRepository{db}, nil
}

type keyRow struct {
	ID, OrganizationID, ActorID, MemberID, Digest, Suffix, State string
	Revision                                                     int64
	ConfigJSON                                                   []byte
	ExpiresAt, CreatedAt                                         time.Time
}

func (row keyRow) credential() (dataservice.Credential, error) {
	var input dataservice.KeyInput
	if json.Unmarshal(row.ConfigJSON, &input) != nil || !input.ExpiresAt.Truncate(time.Microsecond).Equal(row.ExpiresAt) {
		return dataservice.Credential{}, dataservice.ErrUnavailable
	}
	return dataservice.Credential{ID: row.ID, Scope: collection.Scope{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID}, Digest: row.Digest, Suffix: row.Suffix, State: row.State, Revision: row.Revision, Input: input, CreatedAt: row.CreatedAt}, nil
}

const keyColumns = "id,organization_id,actor_id,member_id,digest,suffix,state,revision,config_json,expires_at,created_at"

func readKey(ctx context.Context, db *gorm.DB, id string, lock bool) (dataservice.Credential, error) {
	query := "SELECT " + keyColumns + " FROM data_service_credentials WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	var row keyRow
	result := db.WithContext(ctx).Raw(query, id).Scan(&row)
	if result.Error != nil {
		return dataservice.Credential{}, result.Error
	}
	if result.RowsAffected != 1 {
		return dataservice.Credential{}, dataservice.ErrNotFound
	}
	return row.credential()
}
func (r *CredentialRepository) Read(ctx context.Context, id string) (dataservice.Credential, error) {
	if !collection.ValidID(id) {
		return dataservice.Credential{}, dataservice.ErrNotFound
	}
	return readKey(ctx, r.db, id, false)
}

func (r *CredentialRepository) Creation(ctx context.Context, s collection.Scope, command string) (dataservice.Credential, error) {
	if s.Validate() != nil || !collection.ValidID(command) {
		return dataservice.Credential{}, dataservice.ErrInvalid
	}
	var target string
	row := r.db.WithContext(ctx).Raw("SELECT target_id FROM data_service_commands WHERE organization_id=? AND actor_id=? AND command_key=? AND action='create_key'", s.OrganizationID, s.ActorID, command).Scan(&target)
	if row.Error != nil {
		return dataservice.Credential{}, row.Error
	}
	if row.RowsAffected != 1 {
		return dataservice.Credential{}, dataservice.ErrNotFound
	}
	key, err := r.Read(ctx, target)
	if err != nil {
		return dataservice.Credential{}, err
	}
	if key.Scope.OrganizationID != s.OrganizationID || key.Scope.ActorID != s.ActorID {
		return dataservice.Credential{}, dataservice.ErrNotFound
	}
	return key, nil
}
func (r *CredentialRepository) List(ctx context.Context, s collection.Scope) ([]dataservice.Credential, error) {
	if s.Validate() != nil {
		return nil, dataservice.ErrForbidden
	}
	var rows []keyRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+keyColumns+" FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND state<>'REVOKED' AND expires_at>now() ORDER BY created_at DESC,id", s.OrganizationID, s.ActorID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	keys := []dataservice.Credential{}
	for _, row := range rows {
		k, err := row.credential()
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *CredentialRepository) History(ctx context.Context, s collection.Scope, cursor string, limit int) (dataservice.CredentialHistoryPage, error) {
	if s.Validate() != nil {
		return dataservice.CredentialHistoryPage{}, dataservice.ErrForbidden
	}
	if (cursor != "" && !collection.ValidID(cursor)) || limit < 1 || limit > 100 {
		return dataservice.CredentialHistoryPage{}, dataservice.ErrInvalid
	}
	query := "SELECT " + keyColumns + " FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND (state='REVOKED' OR expires_at<=now())"
	args := []any{s.OrganizationID, s.ActorID}
	if cursor != "" {
		query += " AND id>?"
		args = append(args, cursor)
	}
	query += " ORDER BY id LIMIT ?"
	args = append(args, limit+1)
	var rows []keyRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return dataservice.CredentialHistoryPage{}, err
	}
	page := dataservice.CredentialHistoryPage{Items: []dataservice.Credential{}}
	if len(rows) > limit {
		page.NextCursor = rows[limit-1].ID
		rows = rows[:limit]
	}
	for _, row := range rows {
		key, err := row.credential()
		if err != nil {
			return dataservice.CredentialHistoryPage{}, err
		}
		page.Items = append(page.Items, key)
	}
	return page, nil
}

type commandRow struct {
	MemberID, InputHash, Action, TargetID string
	ReceiptJSON                           []byte
}

func readCommand(tx *gorm.DB, s collection.Scope, command, hash, action string) (commandRow, bool, error) {
	var row commandRow
	result := tx.Raw("SELECT member_id,input_hash,action,target_id,receipt_json FROM data_service_commands WHERE organization_id=? AND actor_id=? AND command_key=?", s.OrganizationID, s.ActorID, command).Scan(&row)
	if result.Error != nil {
		return row, false, result.Error
	}
	if result.RowsAffected == 0 {
		return row, false, nil
	}
	if row.MemberID != s.MemberID || row.InputHash != hash || row.Action != action {
		return row, false, dataservice.ErrConflict
	}
	return row, true, nil
}
func writeCommand(tx *gorm.DB, s collection.Scope, command, hash, action, target string, receipt any) error {
	raw, err := json.Marshal(receipt)
	if err != nil || len(raw) > 8192 {
		return dataservice.ErrInvalid
	}
	return tx.Exec("INSERT INTO data_service_commands(organization_id,actor_id,member_id,command_key,input_hash,action,target_id,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)", s.OrganizationID, s.ActorID, s.MemberID, command, hash, action, target, string(raw), time.Now().UTC()).Error
}
func actorLock(tx *gorm.DB, s collection.Scope) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "data-actor:"+collection.Digest([]string{s.OrganizationID, s.ActorID})).Error
}
func (r *CredentialRepository) Create(ctx context.Context, key dataservice.Credential, command, hash string) (dataservice.Credential, bool, error) {
	if key.Scope.Validate() != nil || !collection.ValidID(key.ID) || !collection.ValidID(command) || len(hash) != 64 || len(key.Digest) != 64 || len(key.Suffix) != 4 || key.State != "ACTIVE" || key.Revision != 1 {
		return dataservice.Credential{}, false, dataservice.ErrInvalid
	}
	var out dataservice.Credential
	var replayed, finished bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := actorLock(tx, key.Scope); err != nil {
			return err
		}
		row, replay, err := readCommand(tx, key.Scope, command, hash, "create_key")
		if err != nil {
			return err
		}
		if replay {
			out, err = readKey(ctx, tx, row.TargetID, false)
			replayed = true
			finished = err == nil
			return err
		}
		var count int64
		if err := tx.Raw("SELECT count(*) FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND state<>'REVOKED' AND expires_at>now()", key.Scope.OrganizationID, key.Scope.ActorID).Scan(&count).Error; err != nil {
			return err
		}
		if count >= 20 {
			return dataservice.ErrConflict
		}
		raw, err := json.Marshal(key.Input)
		if err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO data_service_credentials(id,organization_id,actor_id,member_id,digest,suffix,state,revision,config_json,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", key.ID, key.Scope.OrganizationID, key.Scope.ActorID, key.Scope.MemberID, key.Digest, key.Suffix, key.State, key.Revision, string(raw), key.Input.ExpiresAt, key.CreatedAt).Error; err != nil {
			return err
		}
		if err := writeCommand(tx, key.Scope, command, hash, "create_key", key.ID, key); err != nil {
			return err
		}
		out = key
		finished = true
		return nil
	})
	if err != nil && finished {
		return dataservice.Credential{}, false, dataservice.ErrUnknown
	}
	return out, replayed, err
}
func (r *CredentialRepository) Change(ctx context.Context, s collection.Scope, id, command, hash string, revision int64, patch dataservice.KeyPatch) (dataservice.Credential, error) {
	if s.Validate() != nil || !collection.ValidID(id) || !collection.ValidID(command) || revision < 1 || len(hash) != 64 || (patch.State != "ACTIVE" && patch.State != "DISABLED" && patch.State != "REVOKED") {
		return dataservice.Credential{}, dataservice.ErrInvalid
	}
	var out dataservice.Credential
	finished := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := actorLock(tx, s); err != nil {
			return err
		}
		row, replay, err := readCommand(tx, s, command, hash, "change_key")
		if err != nil {
			return err
		}
		if replay {
			if row.TargetID != id || json.Unmarshal(row.ReceiptJSON, &out) != nil {
				return dataservice.ErrConflict
			}
			bound, err := readKey(ctx, tx, id, false)
			if err != nil {
				return err
			}
			out.Scope = bound.Scope
			finished = true
			return nil
		}
		key, err := readKey(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if key.Scope.OrganizationID != s.OrganizationID || key.Scope.ActorID != s.ActorID {
			return dataservice.ErrNotFound
		}
		if key.Scope.MemberID != s.MemberID && (patch.State != "REVOKED" || patch.Limits != nil) {
			return dataservice.ErrForbidden
		}
		if key.Revision != revision || key.State == "REVOKED" {
			return dataservice.ErrConflict
		}
		if patch.Limits != nil {
			// key is locked before any quota bucket. Job admission/publication use
			// the same order; reducing limits cannot erase pending reservations.
			var usage []struct {
				WindowKind string
				Rows, Fen  int64
			}
			if err := tx.Raw("SELECT window_kind,consumed_rows+reserved_rows AS rows,consumed_fen+reserved_fen AS fen FROM data_service_quota WHERE organization_id=? AND actor_id=? AND key_id=? AND (window_start>=date_trunc(window_kind,now() AT TIME ZONE 'UTC')::date OR reserved_rows>0 OR reserved_fen>0) ORDER BY window_kind,window_start FOR UPDATE", s.OrganizationID, s.ActorID, id).Scan(&usage).Error; err != nil {
				return err
			}
			for _, u := range usage {
				if u.WindowKind == "day" && u.Rows > patch.Limits.DailyRows || u.WindowKind == "month" && u.Fen > patch.Limits.MonthlyCostFen {
					return dataservice.ErrConflict
				}
			}
			key.Input = *patch.Limits
		}
		key.State = patch.State
		if key.State != "REVOKED" {
			// Creation and expiry extension use the same actor lock. Disabled
			// credentials also occupy a slot because they can be enabled again.
			var exceeds bool
			if err := tx.Raw("SELECT ?::timestamptz>now() AND (SELECT count(*) FROM data_service_credentials WHERE organization_id=? AND actor_id=? AND id<>? AND state<>'REVOKED' AND expires_at>now())>=20", key.Input.ExpiresAt, s.OrganizationID, s.ActorID, id).Scan(&exceeds).Error; err != nil {
				return err
			}
			if exceeds {
				return dataservice.ErrConflict
			}
		}
		key.Revision++
		raw, err := json.Marshal(key.Input)
		if err != nil {
			return err
		}
		if err := tx.Exec("UPDATE data_service_credentials SET config_json=?,expires_at=?,state=?,revision=? WHERE id=?", string(raw), key.Input.ExpiresAt, key.State, key.Revision, id).Error; err != nil {
			return err
		}
		if err := writeCommand(tx, s, command, hash, "change_key", id, key); err != nil {
			return err
		}
		out = key
		finished = true
		return nil
	})
	if err != nil && finished {
		return dataservice.Credential{}, dataservice.ErrUnknown
	}
	return out, err
}

var _ dataservice.CredentialRepository = (*CredentialRepository)(nil)
