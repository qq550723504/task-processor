package preparationpersistence

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
)

func (r *Repository) ListStageFacts(ctx context.Context, scope preparation.Scope, preparationID, storeID, supplierHash, after string) ([]preparation.SourceStageFacts, error) {
	if scope.Validate() != nil || !collection.ValidID(preparationID) || !collection.ValidID(storeID) || after != "" && !collection.ValidID(after) || len(supplierHash) != 64 {
		return nil, preparation.ErrInvalid
	}
	type projection struct {
		SourceID, ProductKey, Title, SourceKind, SKUs, RecordID, ApplyReceiptID, PublishedRecordID, OptimizationOperationID, OptimizationRecordID, OptimizationStatus, ResultReference, Note string
		RecordRevision, EffectiveVersion, OptimizationRecordRevision                                                                                                                         int64
		Ready                                                                                                                                                                                bool
	}
	rows := []projection{}
	query := r.db.WithContext(ctx).Table("listing_preparation_sources s").Select(`s.id AS source_id,s.product_key,s.source_kind,COALESCE((SELECT string_agg(variant->>'sku',' ') FROM json_array_elements(CASE WHEN json_typeof(v.snapshot_json->'variants')='array' THEN v.snapshot_json->'variants' ELSE '[]'::json END) AS variant),'') AS sk_us,COALESCE(v.snapshot_json->>'title','') AS title,COALESCE(h.current_record_id::text,'') AS record_id,COALESCE(h.revision,0) AS record_revision,COALESCE((t.body_json->>'effectiveVersion')::bigint,0) AS effective_version,COALESCE(t.body_json->>'applyReceiptId','') AS apply_receipt_id,COALESCE((t.body_json->'result'->>'ready_for_upload')::boolean,false) AS ready,COALESCE(p.body_json->>'record_id','') AS published_record_id,COALESCE(o.operation_id::text,'') AS optimization_operation_id,COALESCE(o.record_id::text,'') AS optimization_record_id,COALESCE(o.record_revision,0) AS optimization_record_revision,COALESCE(o.status,'') AS optimization_status,COALESCE(o.result_reference,'') AS result_reference,COALESCE(o.note,'') AS note`).
		Joins("JOIN product_snapshot_versions v ON v.tenant_id=s.organization_id AND v.product_key=s.product_key AND v.version=s.original_version AND v.publication_id=s.publication_id").
		Joins("LEFT JOIN listing_preparation_targets h ON h.organization_id=s.organization_id AND h.actor_id=s.actor_id AND h.member_id=s.member_id AND h.source_id=s.id AND h.store_id=? AND h.site='shein-us'", storeID).
		Joins("LEFT JOIN listing_target_records t ON t.organization_id=h.organization_id AND t.actor_id=h.actor_id AND t.member_id=h.member_id AND t.id=h.current_record_id AND t.revision=h.revision").
		Joins("LEFT JOIN listing_submission_official_receipts p ON p.organization_id=s.organization_id AND p.actor_id=s.actor_id AND p.member_id=s.member_id AND p.store_id=? AND p.kind='publish' AND p.body_json->>'product_key'=s.product_key AND p.body_json->'binding'->>'supplier_identity_hash'=?", storeID, supplierHash).
		Joins(`LEFT JOIN LATERAL (SELECT i.* FROM listing_preparation_operation_items i JOIN listing_preparation_operations op ON op.organization_id=i.organization_id AND op.actor_id=i.actor_id AND op.member_id=i.member_id AND op.id=i.operation_id WHERE i.organization_id=s.organization_id AND i.actor_id=s.actor_id AND i.member_id=s.member_id AND i.source_id=s.id AND op.preparation_id=s.preparation_id AND op.store_id=? AND op.action='optimize' ORDER BY op.created_at DESC,op.id DESC LIMIT 1) o ON true`, storeID).
		Where("s.organization_id=? AND s.actor_id=? AND s.member_id=? AND s.preparation_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, preparationID)
	if after != "" {
		query = query.Where("s.id>?", after)
	}
	if err := query.Order("s.id").Limit(100).Scan(&rows).Error; err != nil {
		return nil, preparation.ErrUnavailable
	}
	out := make([]preparation.SourceStageFacts, 0, len(rows))
	for _, v := range rows {
		if v.EffectiveVersion < 0 {
			return nil, preparation.ErrUnavailable
		}
		out = append(out, preparation.SourceStageFacts{SourceID: v.SourceID, ProductKey: v.ProductKey, Title: v.Title, SourceKind: v.SourceKind, SKUs: v.SKUs, RecordID: v.RecordID, RecordRevision: v.RecordRevision, EffectiveVersion: uint64(v.EffectiveVersion), ApplyReceiptID: v.ApplyReceiptID, Ready: v.Ready, PublishedRecordID: v.PublishedRecordID, OptimizationOperationID: v.OptimizationOperationID, Optimization: preparation.OperationItem{SourceID: v.SourceID, RecordID: v.OptimizationRecordID, RecordRevision: v.OptimizationRecordRevision, Status: v.OptimizationStatus, ResultReference: v.ResultReference, Note: v.Note}})
	}
	return out, nil
}
func (r *Repository) LatestOptimization(ctx context.Context, scope preparation.Scope, sourceID, storeID string) (preparation.OperationItem, error) {
	if scope.Validate() != nil || !collection.ValidID(sourceID) || !collection.ValidID(storeID) {
		return preparation.OperationItem{}, preparation.ErrInvalid
	}
	var item operationItemRow
	err := r.db.WithContext(ctx).Table("listing_preparation_operation_items i").Select("i.*").Joins("JOIN listing_preparation_operations op ON op.organization_id=i.organization_id AND op.actor_id=i.actor_id AND op.member_id=i.member_id AND op.id=i.operation_id").Where("i.organization_id=? AND i.actor_id=? AND i.member_id=? AND i.source_id=? AND op.store_id=? AND op.action='optimize'", scope.OrganizationID, scope.ActorID, scope.MemberID, sourceID, storeID).Order("op.created_at DESC,op.id DESC").Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return preparation.OperationItem{}, preparation.ErrNotFound
	}
	if err != nil {
		return preparation.OperationItem{}, preparation.ErrUnavailable
	}
	return item.value(), nil
}
