package ownerreconcile

import "fmt"

var ownerReconciliationInventory = buildOwnerReconciliationInventory()

// Inventory returns a defensive copy of the fixed owner reconciliation inventory.
// Keeping the SQL identifiers in this package prevents runtime callers from
// turning them into user-controlled input.
func Inventory() []TableSpec {
	return append([]TableSpec(nil), ownerReconciliationInventory...)
}

func buildOwnerReconciliationInventory() []TableSpec {
	legacySimple := []string{
		"listing_store",
		"listing_category",
		"listing_filter_rule",
		"listing_generation_topic_override",
		"listing_generation_topic_policy",
		"listing_operation_strategy",
		"listing_pricing_rule",
		"listing_profit_rule",
		"listing_scheduled_task_config",
		"listing_sensitive_word",
	}
	native := []string{
		"listing_kit_tasks",
		"listingkit_shein_pod_image_indexes",
		"listingkit_studio_async_jobs",
		"listingkit_studio_batches",
		"listingkit_studio_batch_items",
		"listingkit_studio_generation_attempts",
		"listingkit_studio_materialized_designs",
		"listingkit_studio_batch_task_links",
		"listingkit_studio_batch_runs",
		"listingkit_studio_batch_run_items",
		"shein_studio_sessions",
	}

	inventory := make([]TableSpec, 0, len(legacySimple)+len(native)+3)
	for _, table := range legacySimple {
		inventory = append(inventory, legacySimpleSpec(table))
	}
	inventory = append(inventory, legacyImportTaskSpec())
	inventory = append(inventory, legacyImportRelationSpec("listing_product_import_mapping"))
	inventory = append(inventory, legacyImportRelationSpec("listing_product_data"))
	for _, table := range native {
		inventory = append(inventory, nativeSpec(table))
	}
	return inventory
}

func legacySimpleSpec(table string) TableSpec {
	return TableSpec{
		Table:          table,
		TenantDomain:   TenantDomainLegacyNumeric,
		UpdateQuery:    fmt.Sprintf("WITH target AS (SELECT id FROM %s WHERE tenant_id = $2 AND NULLIF(BTRIM(owner_user_id::text), '') IS NULL AND %s AND %s ORDER BY id LIMIT $5) UPDATE %s AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id", table, exactCandidate("creator", "$3"), exactCandidate("created_by", "$4"), table),
		UpdateLimitArg: 5,
		Query: fmt.Sprintf(`SELECT tenant_id::text, creator::text, created_by::text, COUNT(*)::bigint AS row_count
FROM %s
WHERE NULLIF(BTRIM(owner_user_id::text), '') IS NULL
GROUP BY tenant_id, creator, created_by`, table),
		Columns:          []string{"tenant_id", "creator", "created_by", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}, {Name: "created_by", Source: "created_by"}},
	}
}

func legacyImportTaskSpec() TableSpec {
	return TableSpec{
		Table:          "listing_product_import_task",
		TenantDomain:   TenantDomainLegacyNumeric,
		UpdateQuery:    fmt.Sprintf("WITH target AS (SELECT t.id FROM listing_product_import_task AS t WHERE t.tenant_id = $2 AND NULLIF(BTRIM(t.owner_user_id::text), '') IS NULL AND %s AND %s AND (($5 = '' AND $6 = '') OR EXISTS (SELECT 1 FROM listing_store AS s WHERE s.id = t.store_id AND s.tenant_id = t.tenant_id AND %s)) ORDER BY t.id LIMIT $7) UPDATE listing_product_import_task AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id", exactCandidate("t.creator", "$3"), exactCandidate("t.created_by", "$4"), optionalCandidatePair("s.creator", "$5", "s.created_by", "$6")),
		UpdateLimitArg: 7,
		Query: `SELECT t.tenant_id::text, t.creator::text AS own_creator, t.created_by::text AS own_created_by, s.creator::text AS store_creator, s.created_by::text AS store_created_by, COUNT(*)::bigint AS row_count
FROM listing_product_import_task AS t
LEFT JOIN listing_store AS s ON s.id = t.store_id AND s.tenant_id = t.tenant_id
WHERE NULLIF(BTRIM(t.owner_user_id::text), '') IS NULL
		GROUP BY t.tenant_id, t.creator, t.created_by, s.creator, s.created_by`,
		Columns: []string{"tenant_id", "own_creator", "own_created_by", "store_creator", "store_created_by", "row_count"},
		CandidateColumns: []CandidateColumn{
			{Name: "own_creator", Source: "creator"},
			{Name: "own_created_by", Source: "created_by"},
			{Name: "store_creator", Source: "store"},
			{Name: "store_created_by", Source: "store_created_by"},
		},
	}
}

func legacyImportRelationSpec(table string) TableSpec {
	return TableSpec{
		Table:          table,
		TenantDomain:   TenantDomainLegacyNumeric,
		UpdateQuery:    fmt.Sprintf("WITH target AS (SELECT row.id FROM %s AS row WHERE row.tenant_id = $2 AND NULLIF(BTRIM(row.owner_user_id::text), '') IS NULL AND %s AND %s AND (($5 = '' AND $6 = '') OR EXISTS (SELECT 1 FROM listing_product_import_task AS task WHERE task.id = row.import_task_id AND task.tenant_id = row.tenant_id AND %s)) AND (($7 = '' AND $8 = '') OR EXISTS (SELECT 1 FROM listing_store AS store WHERE store.id = row.store_id AND store.tenant_id = row.tenant_id AND %s)) ORDER BY row.id LIMIT $9) UPDATE %s AS row SET owner_user_id = $1 FROM target WHERE row.id = target.id", table, exactCandidate("row.creator", "$3"), exactCandidate("row.created_by", "$4"), optionalCandidatePair("task.creator", "$5", "task.created_by", "$6"), optionalCandidatePair("store.creator", "$7", "store.created_by", "$8"), table),
		UpdateLimitArg: 9,
		Query: fmt.Sprintf(`SELECT row.tenant_id::text, row.creator::text AS own_creator, row.created_by::text AS own_created_by, task.creator::text AS task_creator, task.created_by::text AS task_created_by, store.creator::text AS store_creator, store.created_by::text AS store_created_by, COUNT(*)::bigint AS row_count
FROM %s AS row
LEFT JOIN listing_product_import_task AS task ON task.id = row.import_task_id AND task.tenant_id = row.tenant_id
LEFT JOIN listing_store AS store ON store.id = row.store_id AND store.tenant_id = row.tenant_id
WHERE NULLIF(BTRIM(row.owner_user_id::text), '') IS NULL
GROUP BY row.tenant_id, row.creator, row.created_by, task.creator, task.created_by, store.creator, store.created_by`, table),
		Columns: []string{"tenant_id", "own_creator", "own_created_by", "task_creator", "task_created_by", "store_creator", "store_created_by", "row_count"},
		CandidateColumns: []CandidateColumn{
			{Name: "own_creator", Source: "creator"},
			{Name: "own_created_by", Source: "created_by"},
			{Name: "task_creator", Source: "task"},
			{Name: "task_created_by", Source: "task_created_by"},
			{Name: "store_creator", Source: "store"},
			{Name: "store_created_by", Source: "store_created_by"},
		},
	}
}

func exactCandidate(column, parameter string) string {
	return fmt.Sprintf("((%s = '' AND NULLIF(BTRIM(%s::text), '') IS NULL) OR (%s <> '' AND %s::text = %s))", parameter, column, parameter, column, parameter)
}

func optionalCandidatePair(firstColumn, firstParameter, secondColumn, secondParameter string) string {
	return fmt.Sprintf("((%s = '' AND %s = '') OR (%s AND %s))", firstParameter, secondParameter, exactCandidate(firstColumn, firstParameter), exactCandidate(secondColumn, secondParameter))
}

func nativeSpec(table string) TableSpec {
	return TableSpec{
		Table:        table,
		TenantDomain: TenantDomainZITADELOrganization,
		Query: fmt.Sprintf(`SELECT tenant_id::text, COUNT(*)::bigint AS row_count
FROM %s
WHERE NULLIF(BTRIM(user_id::text), '') IS NULL
GROUP BY tenant_id`, table),
		Columns: []string{"tenant_id", "row_count"},
	}
}
