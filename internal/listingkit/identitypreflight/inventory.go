package identitypreflight

// OwnerTable identifies one direct tenant-and-user-owned persistence table.
// Every identifier is a compile-time repository constant, never external input.
type OwnerTable struct {
	Table        string
	TenantColumn string
	UserColumn   string
}

var ownerTableInventory = [...]OwnerTable{
	{Table: "listing_kit_tasks", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_shein_pod_image_indexes", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_async_jobs", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_batches", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_batch_items", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_generation_attempts", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_materialized_designs", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_batch_task_links", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_batch_runs", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listingkit_studio_batch_run_items", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "shein_studio_sessions", TenantColumn: "tenant_id", UserColumn: "user_id"},
	{Table: "listing_store", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_category", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_filter_rule", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_generation_topic_override", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_generation_topic_policy", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_product_import_task", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_operation_strategy", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_pricing_rule", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_product_import_mapping", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_profit_rule", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_product_data", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_scheduled_task_config", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
	{Table: "listing_sensitive_word", TenantColumn: "tenant_id", UserColumn: "owner_user_id"},
}
