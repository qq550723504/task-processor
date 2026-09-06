/** Component/unit fixture for the #343 planned contract; never a runtime fallback. */
export function titleProposalFixture() {
  return {
    schema_version: 1 as const, coverage: "product-title-proposals-only" as const,
    proposal_id: "12345678-1234-4234-8234-123456789abc", owner: "owner",
    input: { product_key: "product", base_version: "1" }, before: "Original title",
    after: "Suggested title", original_title: "Suggested title", policy: "title-review-v1" as const,
    state: "pending" as const, revision: "1", evidence: [{ id: "evidence", reference_type: "captured", reference_id: "ref", snapshot_id: "capture", checksum: "sha256:sample" }],
    quality: { overall: 1, evidence_coverage: 1, required_field_coverage: 1 }, unresolved: [], decisions: [],
  };
}
