import { afterEach, expect, it, vi } from "vitest";
import { getAccountAuditSummary, parseAccountAuditSummary } from "./account-audit";
const summary = { schemaVersion: "account-audit-summary-v1", userId: "u1", effectiveOrganizationId: "B", coverage: "current_account_audit_committed_events", window: { from: "2026-08-30T08:00:00Z", asOf: "2026-09-29T08:00:00Z" }, counts: { operations: "120", members: "3", permissions: "1", resources: "5" } };
afterEach(() => vi.unstubAllGlobals());
it("reads a scoped, uncached summary without page or filter input", async () => {
 const fetch = vi.fn().mockResolvedValue(Response.json(summary)); vi.stubGlobal("fetch", fetch);
 expect(await getAccountAuditSummary({ expectedUserId: "u1", expectedOrganizationId: "B" })).toEqual(summary);
 expect(fetch.mock.calls[0][0]).toBe("/api/account/audit/summary");
 expect(fetch.mock.calls[0][1].cache).toBe("no-store");
});
it("keeps exact integers and rejects incomplete or contradictory upstream facts", () => {
 expect(parseAccountAuditSummary({ ...summary, counts: { operations: "9007199254740993", members: "0", permissions: "0", resources: "0" } }).counts.operations).toBe("9007199254740993");
 for (const invalid of [
  { ...summary, counts: { ...summary.counts, operations: "-1" } },
  { ...summary, counts: { ...summary.counts, resources: "9223372036854775808" } },
  { ...summary, counts: { ...summary.counts, permissions: "4" } },
  { ...summary, counts: { ...summary.counts, members: "121" } },
  { ...summary, window: { ...summary.window, from: summary.window.asOf } },
  { ...summary, raw: "secret" }, { ...summary, counts: { operations: "0" } },
 ]) expect(() => parseAccountAuditSummary(invalid)).toThrow();
});
it("rejects mismatched organizations and preserves explicit missing-source errors", async () => {
 vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...summary, effectiveOrganizationId: "A" })));
 await expect(getAccountAuditSummary({ expectedUserId: "u1", expectedOrganizationId: "B" })).rejects.toMatchObject({ code: "ORGANIZATION_CONTEXT_CHANGED" });
 vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "SUMMARY_NOT_CONFIGURED", message: "", requestId: "", fieldErrors: [] }, { status: 503 })));
 await expect(getAccountAuditSummary({ expectedUserId: "u1", expectedOrganizationId: "B" })).rejects.toMatchObject({ code: "SUMMARY_NOT_CONFIGURED" });
});
