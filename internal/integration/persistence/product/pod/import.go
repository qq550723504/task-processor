package podpersistence

import (
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
)

type Receiver func(context.Context, *gorm.DB, collection.Scope, string) (collection.Receipt, error)

func (r *Repository) ReceiptByKey(ctx context.Context, scope collection.Scope, key string) (collection.Receipt, error) {
	if scope.Validate() != nil || !collection.ValidID(key) {
		return collection.Receipt{}, pod.ErrInvalid
	}
	row, found, e := command(r.db.WithContext(ctx), scope, key)
	if e != nil {
		return collection.Receipt{}, e
	}
	if !found || row.MemberID != scope.MemberID || row.Kind == "design" {
		return collection.Receipt{}, pod.ErrNotFound
	}
	var receipt collection.Receipt
	if json.Unmarshal(row.ReceiptJSON, &receipt) != nil || !collection.ValidID(receipt.OperationID) {
		return receipt, pod.ErrUnavailable
	}
	return receipt, nil
}
func (r *Repository) Import(ctx context.Context, scope collection.Scope, key, hash, kind, designID string, receive Receiver, guard func(context.Context) error) (receipt collection.Receipt, err error) {
	if scope.Validate() != nil || !collection.ValidID(key) || len(hash) != 64 || kind != "template" && kind != "finished" || kind == "finished" && !collection.ValidID(designID) || receive == nil || guard == nil {
		return receipt, pod.ErrInvalid
	}
	err = r.write(ctx, func(tx *gorm.DB) error {
		if e := commandLock(tx, scope, key); e != nil {
			return e
		}
		row, found, e := command(tx, scope, key)
		if e != nil {
			return e
		}
		if found {
			if row.InputHash != hash || row.MemberID != scope.MemberID || row.Kind != kind || row.OperationID != designID {
				return pod.ErrConflict
			}
			if json.Unmarshal(row.ReceiptJSON, &receipt) != nil {
				return pod.ErrUnavailable
			}
			return guard(ctx)
		}
		if kind == "finished" {
			o, e := read(tx, scope, designID, true)
			if e != nil {
				return e
			}
			if o.Finished == nil {
				return pod.ErrConflict
			}
		}
		if e = guard(ctx); e != nil {
			return e
		}
		operation := collection.StableID(scope.OrganizationID, scope.ActorID, "pod-import", key)
		receipt, e = receive(ctx, tx, scope, operation)
		if e != nil {
			return e
		}
		if receipt.OperationID != operation {
			return pod.ErrUnavailable
		}
		if e = guard(ctx); e != nil {
			return e
		}
		raw, _ := json.Marshal(receipt)
		var id any
		if designID != "" {
			id = designID
		}
		return tx.Exec("INSERT INTO product_pod_commands(organization_id,actor_id,member_id,command_key,input_hash,kind,operation_id,receipt_json) VALUES(?,?,?,?,?,?,?,?::jsonb)", scope.OrganizationID, scope.ActorID, scope.MemberID, key, hash, kind, id, string(raw)).Error
	})
	return
}
