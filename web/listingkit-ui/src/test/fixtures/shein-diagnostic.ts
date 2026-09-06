import type { SheinDiagnostic } from "@/lib/api/shein-diagnostic";

// Synthetic #322 wire fixture: UI behavior evidence only, never a runtime fallback.
export function diagnosticFixture(action: "publish" | "save_draft" = "publish") {
  const blocker = { rule: "product_name", code: "required", category: "content", status: "blocking" as const, paths: ["product.name"], message: "缺少商品名称", guidance: "请补充商品名称后重新检查。" };
  const warning = { rule: "description", code: "recommended", category: "content", status: "warning" as const, paths: ["product.description"], message: "商品描述有待完善", guidance: "建议补充材质和使用说明。" };
  return {
    diagnostic_only: true as const,
    scope: "shein.offline_package" as const,
    target: { marketplace: "shein" as const, site: "" as const },
    action,
    rule_version: "shein.offline_package.v2" as const,
    input: { actual_digest: `sha256:${"a".repeat(64)}`, binding_version: "shein.persisted-input.go-json.v1" as const, read_at: "2026-09-06T04:00:00Z", evaluated_at: "2026-09-06T04:00:01Z" },
    external_freshness: { status: "not_evaluated" as const, coverage: [] as string[] },
    not_evaluated: ["external_package_freshness", "online_template_freshness", "human_review", "approved_asset_provenance_and_consent", "submission_gate"],
    not_evaluated_reasons: { external_package_freshness: "no_authoritative_package_freshness" },
    offline_checks: { status: "blocked" as const, checks: [blocker, warning], blockers: [blocker], warnings: [warning] },
    action_policy: { readiness_blockers_allowed: action === "save_draft" },
  } satisfies SheinDiagnostic;
}
