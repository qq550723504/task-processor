import { afterEach, describe, expect, it, vi } from "vitest";
import { auditQuery, getAccountAudit, parseAccountAudit } from "./account-audit";

// Synthetic transport fixtures; these do not stand in for source integration.
const empty = { schemaVersion: "account-audit-v1", userId: "user-1", effectiveOrganizationId: "B", source: "source_account_committed_operations", items: [], nextCursor: null };
const options = { expectedUserId: "user-1", expectedOrganizationId: "B" };
const reference = "0198d4f0-0000-7000-8000-000000000001";
const committed = { eventType: "source_account.operation_committed", actor: "actor-2", time: "2026-09-12T00:00:00Z", objectType: "source_account", objectReference: reference, operation: "disable", result: "succeeded", relation: { type: "source_account_version", reference, version: "2" } };
afterEach(() => vi.unstubAllGlobals());
describe("account audit query boundary", () => {
  it("keeps the longest supported cursor and filter combination within the shared query budget", () => {
    const query = auditQuery(100, "a".repeat(3072), "b".repeat(128), "disable", "q".repeat(80), "m".repeat(128), "30d");
    expect(query.length).toBeLessThanOrEqual(4096);
  });
  it("forwards bounded content, period and target member with the current audit scope", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(empty));
    vi.stubGlobal("fetch", fetch);
    await getAccountAudit({ ...options, content: "模型实际用量", period: "30d", memberId: "member-1" });
    const url = new URL(String(fetch.mock.calls[0][0]), "http://localhost");
    expect(url.searchParams.get("query")).toBe("模型实际用量");
    expect(url.searchParams.get("period")).toBe("30d");
    expect(url.searchParams.get("member")).toBe("member-1");
  });
  it.each(["a\u0085b", "a\u0001b", "a\u007fb"])("rejects Unicode control in content before request", content => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    expect(() => auditQuery(20, undefined, undefined, undefined, content)).toThrow();
    expect(fetch).not.toHaveBeenCalled();
  });
  it("reads committed member allocations and monthly caps as current resource facts", async () => {
    const transfer = { eventType: "account_member_resource.changed", actor: "actor-1", time: "2026-09-29T08:00:00Z", objectType: "member_resource", objectReference: "member-1", operation: "allocate_member_resource", result: "succeeded", relation: { type: "organization_resource_operation", reference: "allocate-1", version: "1" }, resource: { type: "data_row", quantity: "100" } };
    const cap = { ...transfer, eventType: "account_member_ai_point_limit.changed", objectType: "member_ai_point_limit", operation: "set_member_ai_point_limit", relation: { ...transfer.relation, reference: "cap-1" }, resource: { type: "ai_point", quantity: "0" } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [transfer, cap] })));
    expect((await getAccountAudit(options)).items).toEqual([transfer, cap]);
    for (const item of [{ ...transfer, resource: { type: "ai_point", quantity: "100" } }, { ...transfer, resource: { type: "data_row", quantity: "0" } }, { ...cap, resource: { type: "data_row", quantity: "0" } }, { ...cap, resource: { type: "ai_point", quantity: "9223372036854775808" } }]) {
      expect(() => parseAccountAudit({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [item] })).toThrow();
    }
  });
  it("preserves opaque operation IDs allowed by the resource writer within its UTF-8 byte limit", async () => {
    const cap = { eventType: "account_member_ai_point_limit.changed", actor: "actor-1", time: "2026-09-29T08:00:00Z", objectType: "member_ai_point_limit", objectReference: "member-1", operation: "set_member_ai_point_limit", result: "succeeded", relation: { type: "organization_resource_operation", reference: "limit key", version: "1" }, resource: { type: "ai_point", quantity: "1000" } };
    for (const reference of ["limit key", "月限操作 / member 1", "x".repeat(128), "\uFEFFlimit key"]) {
      const item = { ...cap, relation: { ...cap.relation, reference } };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [item] })));
      expect((await getAccountAudit(options)).items[0].relation.reference).toBe(reference);
    }
    for (const reference of ["", " limit key", "limit key ", "\u0085limit", "月".repeat(43)]) {
      expect(() => parseAccountAudit({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [{ ...cap, relation: { ...cap.relation, reference } }] })).toThrow();
    }
  });
  it("preserves image point debits as exact strings and does not infer token usage", async () => {
    const debit = { eventType: "account_ai_points.committed", actor: "actor-1", time: "2026-09-26T01:00:00Z", objectType: "image_generation", objectReference: "run-1", operation: "consume", result: "succeeded", relation: { type: "organization_resource_event", reference, version: "" }, points: { memberId: "grant-1", quantity: "9007199254740993", priceVersion: "price-1", intentId: "intent-1" } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+image_ai_point_debits", items: [debit] })));
    expect((await getAccountAudit(options)).items).toEqual([debit]);
    for (const points of [{ ...debit.points, quantity: 12 }, { ...debit.points, quantity: "invalid" }, { ...debit.points, quantity: "0" }, { ...debit.points, quantity: "9223372036854775808" }]) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, items: [{ ...debit, points }] })));
      await expect(getAccountAudit(options)).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
    }
  });
  it("accepts only the native observed model usage projection without inventing an actor", async () => {
    const usage = { eventType: "ai_invocation.usage_observed", actor: "", time: "2026-09-25T01:00:00Z", objectType: "ai_invocation", objectReference: "inv-1", operation: "observe", result: "observed", relation: { type: "ai_invocation", reference: reference, version: "" }, usage: { memberId: "grant-1", quantity: 7, metric: "model_tokens" } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+ai_invocations", items: [usage] })));
    expect((await getAccountAudit(options)).items).toEqual([usage]);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+ai_invocations", items: [{ ...usage, actor: "grant-1" }] })));
    await expect(getAccountAudit(options)).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  });
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
