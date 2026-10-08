import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildWorkbenchBrowserResponse, buildWorkbenchUpstreamRequest } from "./workbench-proxy";

const id = "550e8400-e29b-41d4-a716-446655440000";
const scope = { cookie: "shuomi_effective_organization=org-a", "X-Expected-Organization-ID": "org-a", "X-Expected-User-ID": "actor-a" };
const request = (body: string, extra: Record<string, string> = {}) => new Request("http://localhost/api/workbench/collections/commands", {
  method: "POST", headers: { ...scope, Origin: "http://localhost", "Content-Type": "application/json", "Idempotency-Key": id, ...extra }, body,
});

describe("private product collection proxy", () => {
  beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost"));
  it("binds writes to the verified actor and current enterprise", async () => {
    const mapped = await buildWorkbenchUpstreamRequest(request('{"action":"create_batch","name":"批次"}'), ["collections", "commands"], "server-token", "actor-a");
    expect(mapped).not.toBeInstanceOf(Response);
    if (mapped instanceof Response) return;
    expect(mapped.sourceMutation).toBe(true);
    expect(mapped.url).toMatch(/\/api\/v1\/workbench\/collections\/commands$/);
    expect(new Headers(mapped.init.headers).get("X-Requested-Organization-ID")).toBe("org-a");
    const assertions: Record<string, string>[] = [{ "X-Expected-Organization-ID": "org-b" }, { "X-Expected-User-ID": "actor-b" }];
    for (const assertion of assertions) {
      const rejected = await buildWorkbenchUpstreamRequest(request('{"action":"create_batch","name":"批次"}', assertion), ["collections", "commands"], "server-token", "actor-a");
      expect(rejected).toBeInstanceOf(Response);
      if (rejected instanceof Response) expect(rejected.status).toBe(409);
    }
  });
  it("rejects duplicate and out-of-action fields before dispatch", async () => {
    for (const body of ['{"action":"create_batch","name":"a","name":"b"}', '{"action":"create_batch","name":"批次","organizationId":"victim"}', `{"action":"archive_batch","batchId":"${id}","expectedRevision":1,"product":{"title":"unexpected"}}`]) {
      const result = await buildWorkbenchUpstreamRequest(request(body), ["collections", "commands"], "server-token", "actor-a");
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(400);
    }
  });
  it("rejects extra and repeated list query fields", async () => {
    for (const query of ["keyword=a&keyword=b", "organizationId=victim", "limit=101"]) {
      const result = await buildWorkbenchUpstreamRequest(new Request(`http://localhost/api/workbench/collections/batches?${query}`, { headers: scope }), ["collections", "batches"], "server-token", "actor-a");
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(400);
    }
  });
  it("projects only public batch fields and preserves an uncertain write", async () => {
    const batch = { id, name: "批次", kind: "manual", revision: 1, count: 0, createdAt: "2026-10-08T00:00:00Z", credential: "never expose" };
    const response = await buildWorkbenchBrowserResponse(Response.json({ items: [batch], total: 1 }), "collection-batches");
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ items: [{ id, name: batch.name, kind: batch.kind, revision: 1, count: 0, createdAt: batch.createdAt }], total: 1 });
    const unknown = await buildWorkbenchBrowserResponse(Response.json({ success: true }), "collection-command", undefined, { sourceMutation: true });
    expect(unknown.status).toBe(503);
    expect(await unknown.json()).toMatchObject({ code: "OUTCOME_UNKNOWN" });
  });
});
