import type { CommercialOverview } from "@/lib/api/commercial";

// Explicit component test data. Never imported by application code or used as API fallback.
export function commercialOverviewFixture(organizationId = "org-B"): CommercialOverview {
  const observed = "2026-09-07T00:00:00Z";
  return {
    organization_id: organizationId, observed_at: observed,
    plans: [{ code: "base_payg", name: "基础方案 · 按需使用", source: "approved_product_description", availability: "not_for_sale", price: null, currency: null }],
    subscription: { plan_code: "定制 paid plan", plan_name: "企业实际合同", status: "active", effective_status: "active", starts_at: null, expires_at: null, updated_at: observed },
    entitlements: [
      { module_code: "store_management", status: "active", effective_status: "active", starts_at: null, expires_at: null, updated_at: observed, limits_scope: "explicit_grant_only", uninterpreted_limit_count: 0, limits: [{ metric: "store_count", source_key: "store_count", unit: "store", kind: "finite", raw_value: "0", value: "0" }] },
      { module_code: "listingkit", status: "active", effective_status: "active", starts_at: null, expires_at: null, updated_at: observed, limits_scope: "explicit_grant_only", uninterpreted_limit_count: 1, limits: [{ metric: "product_image_jobs_succeeded", source_key: "product_image_jobs", unit: "operation", kind: "unlimited", raw_value: "0", value: null }] },
    ],
    usage: (["listingkit_generations_succeeded", "product_image_jobs_succeeded", "shein_drafts_succeeded", "shein_publishes_succeeded", "storage_bytes_current"] as const).map((metric, index) => ({
      module_code: index === 4 ? "oss_storage" : "listingkit", metric, source: "subscription_usage_ledger", unit: index === 4 ? "byte" : "operation",
      period_key: index === 4 ? "__current__" : "2026-09", window_start: index === 4 ? null : "2026-09-01T00:00:00Z", window_end: index === 4 ? null : "2026-10-01T00:00:00Z",
      state: index === 1 ? "unknown" : "known", committed: index === 1 ? null : index === 4 ? "9007199254740993" : "0", reserved: index === 1 ? null : index === 4 ? "-1" : "0", updated_at: index === 1 ? null : observed,
    })),
    resource_balance: { state: "unsupported", value: null }, cash_balance: { state: "unsupported", value: null },
  };
}
