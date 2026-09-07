// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ProductTitleReviewError, applyProductTitleProposal, decideProductTitleProposal, fetchProductTitleProposal, fetchProductTitleProposals } from "./product-title-review-client";
import { titleProposalFixture } from "@/test/product-title-review-fixture";
const context = { organizationId: "B", proposalId: titleProposalFixture().proposal_id, idempotencyKey: "explicit-key" };
afterEach(() => vi.unstubAllGlobals());

it("sends the exact revision string/key once and never applies after accept", async () => {
  const fetcher = vi.fn().mockResolvedValue(Response.json(titleProposalFixture())); vi.stubGlobal("fetch", fetcher);
  await decideProductTitleProposal({ ...context, input: { action: "accept", expected_revision: "9007199254740993" } });
  expect(fetcher).toHaveBeenCalledOnce();
  expect(fetcher.mock.calls[0][0]).toBe(`/api/product/text-proposals/${context.proposalId}/decisions`);
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ action: "accept", expected_revision: "9007199254740993" });
  expect(new Headers(fetcher.mock.calls[0][1].headers).get("Idempotency-Key")).toBe(context.idempotencyKey);
});
it.each(["network", "bad-json", "abort", "unknown-envelope"])("classifies dispatched %s as unknown, never sends a repair POST", async (fault) => {
  const controller = new AbortController();
  const fetcher = vi.fn().mockImplementation(async () => {
    if (fault === "network") throw new Error("private transport");
    if (fault === "abort") { controller.abort(); throw controller.signal.reason; }
    return fault === "bad-json" ? new Response("bad") : Response.json({ error: "private SQL" }, { status: 503 });
  }); vi.stubGlobal("fetch", fetcher);
  await expect(applyProductTitleProposal({ ...context, input: { expected_revision: "2" }, signal: controller.signal })).rejects.toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
  expect(fetcher).toHaveBeenCalledOnce();
});
it.each(["decision", "apply"])("preserves a validated 504 unknown response for %s without retrying", async (operation) => {
  const payload = { code: "RESULT_UNVERIFIED", message: "Explicitly verify this operation", requestId: "request-504", fieldErrors: [], outcome: "unknown" } as const;
  const fetcher = vi.fn().mockResolvedValue(Response.json(payload, { status: 504 }));
  vi.stubGlobal("fetch", fetcher);
  const pending = operation === "decision"
    ? decideProductTitleProposal({ ...context, input: { action: "accept", expected_revision: "1" } })
    : applyProductTitleProposal({ ...context, input: { expected_revision: "2" } });
  await expect(pending).rejects.toMatchObject({ status: 504, code: "RESULT_UNVERIFIED", payload, outcome: "unknown" });
  expect(fetcher).toHaveBeenCalledOnce();
});
it("marks invalid/pre-aborted writes not_sent and does not generate keys", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  await expect(applyProductTitleProposal({ ...context, idempotencyKey: "", input: { expected_revision: "2" } })).rejects.toMatchObject({ outcome: "not_sent" });
  await expect(applyProductTitleProposal({ ...context, idempotencyKey: "", input: { expected_revision: "2" } })).rejects.toBeInstanceOf(ProductTitleReviewError);
  const controller = new AbortController(); controller.abort();
  await expect(applyProductTitleProposal({ ...context, input: { expected_revision: "2" }, signal: controller.signal })).rejects.toMatchObject({ outcome: "not_sent" });
  expect(fetcher).not.toHaveBeenCalled();
});
it("retains confirmed rejection and BFF not-sent classifications", async () => {
  const notSent = { code: "AUTHENTICATION_REQUIRED", message: "auth", requestId: "request-auth", fieldErrors: [], outcome: "not_sent" } as const;
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ error: "operation_conflict" }, { status: 409 }))
    .mockResolvedValueOnce(Response.json(notSent, { status: 401 }));
  vi.stubGlobal("fetch", fetcher);
  await expect(applyProductTitleProposal({ ...context, input: { expected_revision: "2" } })).rejects.toMatchObject({ status: 409, code: "operation_conflict", payload: { error: "operation_conflict" }, outcome: "rejected" });
  await expect(applyProductTitleProposal({ ...context, input: { expected_revision: "2" } })).rejects.toMatchObject({ status: 401, code: "AUTHENTICATION_REQUIRED", payload: notSent, outcome: "not_sent" });
  expect(fetcher).toHaveBeenCalledTimes(2);
});
it("reads only the same-origin collection/detail and rejects a mismatched detail ID", async () => {
  const list = { schema_version: 1, coverage: "product-title-proposals-only", items: [], next_cursor: null };
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json(list)).mockResolvedValueOnce(Response.json({ ...titleProposalFixture(), proposal_id: "12345678-1234-4234-8234-123456789abd" }));
  vi.stubGlobal("fetch", fetcher);
  expect(await fetchProductTitleProposals({ organizationId: "B" })).toEqual(list);
  expect(fetcher.mock.calls[0][0]).toBe("/api/product/text-proposals?view=actionable&limit=20");
  await expect(fetchProductTitleProposal(context)).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});
