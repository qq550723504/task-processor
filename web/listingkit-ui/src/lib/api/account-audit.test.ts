import { afterEach, describe, expect, it, vi } from "vitest";
import { getAccountAudit } from "./account-audit";

// Synthetic transport fixtures; these do not stand in for source integration.
const empty = { schemaVersion: "account-audit-v1", userId: "user-1", effectiveOrganizationId: "B", source: "source_account_committed_operations", items: [], nextCursor: null };
const options = { expectedUserId: "user-1", expectedOrganizationId: "B" };
const reference = "0198d4f0-0000-7000-8000-000000000001";
const committed = { eventType: "source_account.operation_committed", actor: "actor-2", time: "2026-09-12T00:00:00Z", objectType: "source_account", objectReference: reference, operation: "disable", result: "succeeded", relation: { type: "source_account_version", reference, version: "2" } };
afterEach(() => vi.unstubAllGlobals());
describe("account audit query boundary", () => {
  it("preserves only a structured operation from the source", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, items: [committed] })));
    expect((await getAccountAudit(options)).items).toEqual([committed]);
  });
  it.each([
    { ...committed, requestFingerprint: "private" },
    { ...committed, rawText: "private" },
    { ...committed, operation: "arbitrary" },
    { ...committed, actor: "actor\nspoof" },
    { ...committed, result: "unknown" },
    { ...committed, relation: { ...committed.relation, reference: "0198d4f0-0000-7000-8000-000000000002" } },
  ])("rejects unsafe or unsupported event fields", async item => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, items: [item] })));
    await expect(getAccountAudit(options)).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  });
  it("reads a successful empty page separately from unavailable", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(Response.json(empty)).mockResolvedValueOnce(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "private provider detail", requestId: "", fieldErrors: [] }, { status: 503 }));
    vi.stubGlobal("fetch", fetch);
    expect((await getAccountAudit(options)).items).toEqual([]);
    await expect(getAccountAudit(options)).rejects.toMatchObject({ status: 503, code: "DEPENDENCY_UNAVAILABLE" });
    expect(fetch.mock.calls[0][0]).toBe("/api/account/audit?limit=20");
    expect(fetch.mock.calls[0][1]).toMatchObject({ method: "GET", cache: "no-store", credentials: "same-origin" });
  });
  it.each([
    [{ ...empty, userId: "other" }, "IDENTITY_CONTEXT_CHANGED"],
    [{ ...empty, effectiveOrganizationId: "A" }, "ORGANIZATION_CONTEXT_CHANGED"],
    [{ ...empty, rawPayload: "secret" }, "INVALID_UPSTREAM_RESPONSE"],
    [{ ...empty, nextCursor: "x".repeat(2049) }, "INVALID_UPSTREAM_RESPONSE"],
  ])("rejects mismatched scope or unallowlisted data", async (payload, code) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(payload)));
    await expect(getAccountAudit(options)).rejects.toMatchObject({ code });
  });
  it.each([0, -1, 101, 1.5, NaN])("rejects out of bounds page size before fetch", async limit => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    await expect(getAccountAudit({ ...options, limit })).rejects.toMatchObject({ code: "INVALID_REQUEST" });
    expect(fetch).not.toHaveBeenCalled();
  });
  it("rejects a late result even when the transport ignores abort", async () => {
    const controller = new AbortController();
    vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => { controller.abort(); return Response.json(empty); }));
    await expect(getAccountAudit({ ...options, signal: controller.signal })).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
  });
});
