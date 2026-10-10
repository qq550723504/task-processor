package podpersistence

import (
	"context"
	"encoding/json"
	"errors"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"time"

	"gorm.io/gorm"
)

type Guard func(context.Context, *gorm.DB, pod.Plan) error
type Finalize func(context.Context, *gorm.DB, pod.Operation) error
type Repository struct{ db *gorm.DB }

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if VerifySchema(ctx, db) != nil || submissionstore.VerifySchema(ctx, db) != nil {
		return nil, pod.ErrUnavailable
	}
	return &Repository{db}, nil
}
func (r *Repository) write(ctx context.Context, fn func(*gorm.DB) error) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return pod.ErrUnavailable
	}
	if e := fn(tx); e != nil {
		_ = tx.Rollback().Error
		return e
	}
	if tx.Commit().Error != nil {
		return pod.ErrUnknown
	}
	return nil
}

type operationRow struct {
	ID, OrganizationID, ActorID, MemberID string
	OperationJSON                         []byte
	CreatedAt, NextObservationAt          time.Time
}

func read(db *gorm.DB, scope collection.Scope, id string, lock bool) (pod.Operation, error) {
	var row operationRow
	q := db.Table("product_pod_operations").Where("id=? AND organization_id=? AND actor_id=? AND member_id=?", id, scope.OrganizationID, scope.ActorID, scope.MemberID)
	if lock {
		result := db.Raw("SELECT id,organization_id,actor_id,member_id,operation_json,created_at,next_observation_at FROM product_pod_operations WHERE id=? AND organization_id=? AND actor_id=? AND member_id=? FOR UPDATE", id, scope.OrganizationID, scope.ActorID, scope.MemberID).Scan(&row)
		if result.Error != nil {
			return pod.Operation{}, result.Error
		}
		if result.RowsAffected != 1 {
			return pod.Operation{}, pod.ErrNotFound
		}
	} else {
		if e := q.Take(&row).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return pod.Operation{}, pod.ErrNotFound
			}
			return pod.Operation{}, e
		}
	}
	var o pod.Operation
	if len(row.OperationJSON) > 2<<20 || json.Unmarshal(row.OperationJSON, &o) != nil || o.Plan.Scope != scope || o.Plan.OperationID != id || o.Plan.Validate() != nil {
		return pod.Operation{}, pod.ErrUnavailable
	}
	o.CreatedAt = row.CreatedAt
	o.NextObservationAt = row.NextObservationAt
	return o, nil
}
func (r *Repository) Read(ctx context.Context, scope collection.Scope, id string) (pod.Operation, error) {
	if ctx == nil || scope.Validate() != nil || !collection.ValidID(id) {
		return pod.Operation{}, pod.ErrInvalid
	}
	return read(r.db.WithContext(ctx), scope, id, false)
}
func update(db *gorm.DB, o pod.Operation) error {
	raw, e := json.Marshal(o)
	if e != nil || len(raw) > 2<<20 {
		return pod.ErrInvalid
	}
	result := db.Exec("UPDATE product_pod_operations SET operation_json=?::jsonb WHERE id=? AND organization_id=? AND actor_id=? AND member_id=?", string(raw), o.Plan.OperationID, o.Plan.Scope.OrganizationID, o.Plan.Scope.ActorID, o.Plan.Scope.MemberID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return pod.ErrNotFound
	}
	return nil
}
func held(db *gorm.DB, p pod.Plan) error {
	var id string
	result := db.Raw("SELECT operation_id FROM product_pod_fences WHERE fence_key=? FOR UPDATE", p.FenceKey()).Scan(&id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 || id != p.OperationID {
		return pod.ErrConflict
	}
	return nil
}

type commandRow struct {
	MemberID, InputHash, Kind, OperationID string
	ReceiptJSON                            []byte
}

func command(db *gorm.DB, scope collection.Scope, key string) (commandRow, bool, error) {
	var row commandRow
	result := db.Raw("SELECT member_id,input_hash,kind,operation_id,receipt_json FROM product_pod_commands WHERE organization_id=? AND actor_id=? AND command_key=?", scope.OrganizationID, scope.ActorID, key).Scan(&row)
	return row, result.RowsAffected == 1, result.Error
}
func commandLock(db *gorm.DB, scope collection.Scope, key string) error {
	return db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", collection.Digest([]string{"pod-command", scope.OrganizationID, scope.ActorID, key})).Error
}
func (r *Repository) Begin(ctx context.Context, key, hash string, p pod.Plan, guard Guard) (out pod.Operation, err error) {
	if ctx == nil || !collection.ValidID(key) || len(hash) != 64 || p.Validate() != nil || guard == nil {
		return out, pod.ErrInvalid
	}
	err = r.write(ctx, func(tx *gorm.DB) error {
		if e := commandLock(tx, p.Scope, key); e != nil {
			return e
		}
		row, found, e := command(tx, p.Scope, key)
		if e != nil {
			return e
		}
		if found {
			if row.InputHash != hash || row.Kind != "design" || row.MemberID != p.Scope.MemberID || row.OperationID != p.OperationID {
				return pod.ErrConflict
			}
			out, e = read(tx, p.Scope, p.OperationID, false)
			if e != nil {
				return e
			}
			return guard(ctx, tx, out.Plan)
		}
		if e = tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", p.FenceKey()).Error; e != nil {
			return e
		}
		var n int64
		if e = tx.Table("product_pod_fences").Where("fence_key=?", p.FenceKey()).Count(&n).Error; e != nil {
			return e
		}
		if n != 0 {
			return pod.ErrConflict
		}
		if e = guard(ctx, tx, p); e != nil {
			return e
		}
		out = pod.Operation{Plan: p, CreatedAt: time.Now().UTC()}
		raw, _ := json.Marshal(out)
		if e = tx.Exec("INSERT INTO product_pod_operations(id,organization_id,actor_id,member_id,operation_json,created_at,next_observation_at) VALUES(?,?,?,?,?::jsonb,?,?)", p.OperationID, p.Scope.OrganizationID, p.Scope.ActorID, p.Scope.MemberID, string(raw), out.CreatedAt, out.CreatedAt).Error; e != nil {
			return e
		}
		if e = tx.Exec("INSERT INTO product_pod_fences(fence_key,operation_id) VALUES(?,?)", p.FenceKey(), p.OperationID).Error; e != nil {
			return e
		}
		return tx.Exec("INSERT INTO product_pod_commands(organization_id,actor_id,member_id,command_key,input_hash,kind,operation_id,receipt_json) VALUES(?,?,?,?,?,'design',?,'{}'::jsonb)", p.Scope.OrganizationID, p.Scope.ActorID, p.Scope.MemberID, key, hash, p.OperationID).Error
	})
	return
}
func (r *Repository) ByKey(ctx context.Context, scope collection.Scope, key string) (pod.Operation, error) {
	if scope.Validate() != nil || !collection.ValidID(key) {
		return pod.Operation{}, pod.ErrInvalid
	}
	row, found, e := command(r.db.WithContext(ctx), scope, key)
	if e != nil {
		return pod.Operation{}, e
	}
	if !found || row.MemberID != scope.MemberID || row.Kind != "design" {
		return pod.Operation{}, pod.ErrNotFound
	}
	return r.Read(ctx, scope, row.OperationID)
}
func (r *Repository) Check(ctx context.Context, o pod.Operation, guard Guard) error {
	return r.write(ctx, func(tx *gorm.DB) error {
		current, e := read(tx, o.Plan.Scope, o.Plan.OperationID, true)
		if e != nil {
			return e
		}
		if collection.Digest(current.Plan) != collection.Digest(o.Plan) {
			return pod.ErrConflict
		}
		if e = held(tx, current.Plan); e != nil {
			return e
		}
		return guard(ctx, tx, current.Plan)
	})
}

// Provider evidence and retained step reference are committed in one Product UoW.
func (r *Repository) SaveStep(ctx context.Context, o pod.Operation, step string, value any, finalize Finalize, guard Guard) error {
	if finalize == nil || guard == nil || step != pod.StepOSS && step != pod.StepMaterial {
		return pod.ErrInvalid
	}
	return r.write(ctx, func(tx *gorm.DB) error {
		current, e := read(tx, o.Plan.Scope, o.Plan.OperationID, true)
		if e != nil {
			return e
		}
		if e = held(tx, current.Plan); e != nil {
			return e
		}
		if e = guard(ctx, tx, current.Plan); e != nil {
			return e
		}
		switch step {
		case pod.StepOSS:
			v, ok := value.(pod.ObjectReceipt)
			if !ok || v.Hash != current.Plan.Artwork.Hash || v.FileCode == "" || current.Object != nil {
				return pod.ErrConflict
			}
			current.Object = &v
		case pod.StepMaterial:
			v, ok := value.(pod.MaterialReceipt)
			if !ok || current.Object == nil || v.Hash != current.Plan.Artwork.Hash || v.FileCode != current.Object.FileCode || v.Name != pod.MaterialName(current.Plan.OperationID) || current.Material != nil {
				return pod.ErrConflict
			}
			current.Material = &v
		}
		if e = finalize(ctx, tx, current); e != nil {
			return e
		}
		return update(tx, current)
	})
}
func (r *Repository) FreezeSync(ctx context.Context, o pod.Operation, guard Guard) (out pod.Operation, err error) {
	err = r.write(ctx, func(tx *gorm.DB) error {
		current, e := read(tx, o.Plan.Scope, o.Plan.OperationID, true)
		if e != nil {
			return e
		}
		if e = held(tx, current.Plan); e != nil {
			return e
		}
		if e = guard(ctx, tx, current.Plan); e != nil {
			return e
		}
		if current.Material == nil {
			return pod.ErrConflict
		}
		intent, payload, e := pod.BuildSync(current.Plan, *current.Material)
		if e != nil {
			return e
		}
		if current.Intent != nil {
			if collection.Digest(*current.Intent) != collection.Digest(intent) || string(current.Payload) != string(payload) {
				return pod.ErrConflict
			}
		} else {
			current.Intent = &intent
			current.Payload = payload
			if e = update(tx, current); e != nil {
				return e
			}
		}
		out = current
		return nil
	})
	return
}
func (r *Repository) ObservePermit(ctx context.Context, scope collection.Scope, id string) (bool, error) {
	var allowed bool
	err := r.write(ctx, func(tx *gorm.DB) error {
		o, e := read(tx, scope, id, true)
		if e != nil {
			return e
		}
		if o.Finished != nil {
			return nil
		}
		if e = held(tx, o.Plan); e != nil {
			return e
		}
		now := time.Now().UTC()
		if now.Before(o.NextObservationAt) {
			return nil
		}
		result := tx.Exec("UPDATE product_pod_operations SET next_observation_at=? WHERE id=?", now.Add(5*time.Second), id)
		allowed = result.Error == nil
		return result.Error
	})
	return allowed, err
}

// All sent steps must be terminal. A late sync readback resolves only its
// original attempt; the global template fence is released in this transaction.
func (r *Repository) Finish(ctx context.Context, o pod.Operation, q pod.QualifiedFinished, finalize Finalize, guard Guard) error {
	f := q.Reference()
	if finalize == nil || guard == nil || f.OperationID != o.Plan.OperationID {
		return pod.ErrUnknown
	}
	return r.write(ctx, func(tx *gorm.DB) error {
		current, e := read(tx, o.Plan.Scope, o.Plan.OperationID, true)
		if e != nil {
			return e
		}
		if current.Finished != nil {
			if collection.Digest(*current.Finished) != collection.Digest(f) {
				return pod.ErrConflict
			}
			return nil
		}
		if e = held(tx, current.Plan); e != nil {
			return e
		}
		if e = guard(ctx, tx, current.Plan); e != nil {
			return e
		}
		if current.Object == nil || current.Material == nil || current.Intent == nil || !q.MatchesIntent(*current.Intent) || f.MerchantID != current.Plan.Binding.MerchantID {
			return pod.ErrUnknown
		}
		if e = finalize(ctx, tx, current); e != nil {
			return e
		}
		current.Finished = &f
		if e = update(tx, current); e != nil {
			return e
		}
		result := tx.Exec("DELETE FROM product_pod_fences WHERE fence_key=? AND operation_id=?", current.Plan.FenceKey(), current.Plan.OperationID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return pod.ErrUnknown
		}
		return nil
	})
}
