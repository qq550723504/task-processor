import { expect, it } from "vitest";
import { parseProductTitleProposal, parseProductTitleProposalList, productTitleDecisionSchema, productTitleApplySchema, parseProductTitleReviewFailure } from "./product-title-review";
import { titleProposalFixture } from "@/test/product-title-review-fixture";

it("consumes the sole #343 review-v1 DTO without rounding versions", () => {
  const value = titleProposalFixture(); value.input.base_version = "9007199254740993";
  expect(parseProductTitleProposal(value)).toEqual(value);
  expect(parseProductTitleProposal({ ...value, revision: 1 })).toBeNull();
  expect(parseProductTitleProposal({ ...value, quality: { Overall: 1, EvidenceCoverage: 1, RequiredFieldCoverage: 1 } })).toBeNull();
  expect(parseProductTitleProposal({ ...value, decisions: null })).toBeNull();
  expect(parseProductTitleProposal({ ...value, apply_receipt: null })).toBeNull();
});
it("requires real coverage and canonical bounded versions, IDs and arrays", () => {
  const item = { proposal_id: titleProposalFixture().proposal_id, product_key: "product", base_version: "1", proposal_revision: "1", state: "pending" };
  const list = { schema_version: 1, coverage: "product-title-proposals-only", items: [item], next_cursor: null };
  expect(parseProductTitleProposalList(list)).toEqual(list);
  for (const patch of [{ state: "applied" }, { base_version: "01" }, { base_version: "9223372036854775808" }, { proposal_id: "00000000-0000-0000-0000-000000000000" }]) {
    expect(parseProductTitleProposalList({ ...list, items: [{ ...item, ...patch }] })).toBeNull();
  }
  expect(parseProductTitleProposalList({ ...list, items: [item, item] })).toBeNull();
  expect(parseProductTitleProposalList({ ...list, task_id: "fake" })).toBeNull();
});
it("keeps explicit decisions and apply distinct, validates UTF-8 title bytes without fixing text", () => {
  expect(productTitleDecisionSchema.safeParse({ action: "edit", expected_revision: "2", title: "中".repeat(1365) }).success).toBe(true);
  for (const input of [
    { action: "accept", expected_revision: "2", title: "injected" },
    { action: "edit", expected_revision: "2", title: " title " },
    { action: "edit", expected_revision: "2", title: "中".repeat(1366) },
    { action: "edit", expected_revision: "2", title: "\ud800" },
    { action: "reject", expected_revision: 2 },
  ]) expect(productTitleDecisionSchema.safeParse(input).success).toBe(false);
  expect(productTitleApplySchema.safeParse({ expected_revision: "2" }).success).toBe(true);
  expect(productTitleApplySchema.safeParse({ expected_revision: "2", title: "injected" }).success).toBe(false);
});
it("distinguishes domain, authorization and ambiguous transport failures", () => {
  expect(parseProductTitleReviewFailure({ error: "operation_conflict" }, 409)).toEqual({ error: "operation_conflict" });
  expect(parseProductTitleReviewFailure({ error: "operation_conflict" }, 200)).toBeNull();
  expect(parseProductTitleReviewFailure({ error: "SQL private" }, 503)).toBeNull();
  expect(parseProductTitleReviewFailure({ code: "RESULT_UNVERIFIED", message: "check", requestId: "r1", fieldErrors: [], outcome: "unknown" }, 502)).not.toBeNull();
});
