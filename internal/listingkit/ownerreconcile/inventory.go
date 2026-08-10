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
		Table:        table,
		TenantDomain: TenantDomainLegacyNumeric,
		UpdateQuery:  fmt.Sprintf("UPDATE %s SET owner_user_id = $1 WHERE tenant_id = $2 AND NULLIF(BTRIM(owner_user_id::text), '') IS NULL AND creator::text = $3", table),
		Query: fmt.Sprintf(`SELECT tenant_id::text, creator::text, COUNT(*)::bigint AS row_count
FROM %s
WHERE NULLIF(BTRIM(owner_user_id::text), '') IS NULL
GROUP BY tenant_id, creator`, table),
		Columns:          []string{"tenant_id", "creator", "row_count"},
		CandidateColumns: []CandidateColumn{{Name: "creator", Source: "creator"}},
	}
}

func legacyImportTaskSpec() TableSpec {
	return TableSpec{
		Table:        "listing_product_import_task",
		TenantDomain: TenantDomainLegacyNumeric,
		UpdateQuery:  `UPDATE listing_product_import_task AS t SET owner_user_id = $1 FROM listing_store AS s WHERE t.tenant_id = $2 AND NULLIF(BTRIM(t.owner_user_id::text), '') IS NULL AND t.store_id = s.id AND s.tenant_id = t.tenant_id AND ($3 = '' OR t.creator::text = $3) AND ($4 = '' OR s.creator::text = $4)`,
		Query: `SELECT t.tenant_id::text, t.creator::text AS own_creator, s.creator::text AS store_creator, COUNT(*)::bigint AS row_count
FROM listing_product_import_task AS t
LEFT JOIN listing_store AS s ON s.id = t.store_id AND s.tenant_id = t.tenant_id
WHERE NULLIF(BTRIM(t.owner_user_id::text), '') IS NULL
GROUP BY t.tenant_id, t.creator, s.creator`,
		Columns: []string{"tenant_id", "own_creator", "store_creator", "row_count"},
		CandidateColumns: []CandidateColumn{
			{Name: "own_creator", Source: "creator"},
			{Name: "store_creator", Source: "store"},
		},
	}
}

func legacyImportRelationSpec(table string) TableSpec {
	return TableSpec{
		Table:        table,
		TenantDomain: TenantDomainLegacyNumeric,
		UpdateQuery:  fmt.Sprintf("UPDATE %s AS row SET owner_user_id = $1 FROM listing_product_import_task AS task LEFT JOIN listing_store AS store ON store.id = row.store_id AND store.tenant_id = row.tenant_id WHERE row.tenant_id = $2 AND NULLIF(BTRIM(row.owner_user_id::text), '') IS NULL AND task.id = row.import_task_id AND task.tenant_id = row.tenant_id AND ($3 = '' OR row.creator::text = $3) AND ($4 = '' OR task.creator::text = $4) AND ($5 = '' OR store.creator::text = $5)", table),
		Query: fmt.Sprintf(`SELECT row.tenant_id::text, row.creator::text AS own_creator, task.creator::text AS task_creator, store.creator::text AS store_creator, COUNT(*)::bigint AS row_count
FROM %s AS row
LEFT JOIN listing_product_import_task AS task ON task.id = row.import_task_id AND task.tenant_id = row.tenant_id
LEFT JOIN listing_store AS store ON store.id = row.store_id AND store.tenant_id = row.tenant_id
WHERE NULLIF(BTRIM(row.owner_user_id::text), '') IS NULL
GROUP BY row.tenant_id, row.creator, task.creator, store.creator`, table),
		Columns: []string{"tenant_id", "own_creator", "task_creator", "store_creator", "row_count"},
		CandidateColumns: []CandidateColumn{
			{Name: "own_creator", Source: "creator"},
			{Name: "task_creator", Source: "task"},
			{Name: "store_creator", Source: "store"},
		},
	}
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
