import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildWorkbenchBrowserResponse, buildWorkbenchUpstreamRequest } from "./workbench-proxy";

const id = "550e8400-e29b-41d4-a716-446655440000";
const scope = { cookie: "shuomi_effective_organization=org-b", "X-Expected-Organization-ID": "org-b", "X-Expected-User-ID": "user-a" };

describe("AI Workbench BFF boundary", () => {
  beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost"));
  it("allows a scoped Chat create and strips browser identity claims", async () => {
    const request = new Request("http://localhost/api/workbench/chat/conversations", {
      method: "POST", headers: { ...scope, Origin: "http://localhost", "Content-Type": "application/json", "Idempotency-Key": id, "X-Tenant-ID": "victim" }, body: "{}",
    });
    const mapped = await buildWorkbenchUpstreamRequest(request, ["chat", "conversations"], "server-token", "user-a");
    expect(mapped).not.toBeInstanceOf(Response);
    if (mapped instanceof Response) return;
    expect(mapped.url).toMatch(/\/api\/v1\/workbench\/chat\/conversations$/);
    const headers = new Headers(mapped.init.headers);
    expect(headers.get("Authorization")).toBe("Bearer server-token");
    expect(headers.get("X-Requested-Organization-ID")).toBe("org-b");
    expect(headers.get("X-Tenant-ID")).toBeNull();
    expect(mapped.sourceMutation).toBe(true);
  });

  it("rejects a write after the expected enterprise changes", async () => {
    const request = new Request("http://localhost/api/workbench/chat/conversations", {
      method: "POST", headers: { ...scope, "X-Expected-Organization-ID": "org-a", Origin: "http://localhost", "Content-Type": "application/json", "Idempotency-Key": id }, body: "{}",
    });
    const mapped = await buildWorkbenchUpstreamRequest(request, ["chat", "conversations"], "server-token", "user-a");
    expect(mapped).toBeInstanceOf(Response);
    if (mapped instanceof Response) expect(mapped.status).toBe(409);
  });

  it("forwards only the scoped saved-conversation page filter", async () => {
    const headers = { ...scope };
    const cursor = "A".repeat(31) + "B";
    const accepted = new Request(`http://localhost/api/workbench/chat/conversations?limit=50&saved=true&after=${cursor}`, { headers });
    const mapped = await buildWorkbenchUpstreamRequest(accepted, ["chat", "conversations"], "server-token", "user-a");
    expect(mapped).not.toBeInstanceOf(Response);
    if (mapped instanceof Response) return;
    expect(mapped.url).toContain(`?limit=50&saved=true&after=${cursor}`);
    const archived = new Request(`http://localhost/api/workbench/chat/conversations?archived=true&after=${cursor}`, { headers });
    const archivedMapped = await buildWorkbenchUpstreamRequest(archived, ["chat", "conversations"], "server-token", "user-a");
    expect(archivedMapped).not.toBeInstanceOf(Response);
    if (!(archivedMapped instanceof Response)) expect(archivedMapped.url).toContain(`?archived=true&after=${cursor}`);
    for (const suffix of ["saved=false", "saved=true&saved=true", "archived=false", "saved=true&archived=true", "unknown=true", `after=${id}`]) {
      const rejected = new Request(`http://localhost/api/workbench/chat/conversations?${suffix}`, { headers });
      const result = await buildWorkbenchUpstreamRequest(rejected, ["chat", "conversations"], "server-token", "user-a");
      expect(result).toBeInstanceOf(Response);
      if (result instanceof Response) expect(result.status).toBe(400);
    }
  });

  it("accepts only the curated Task projection fields", async () => {
    const task = { id, conversationId: id, proposalId: id, title: "Title suggestion", goalSummary: "Improve title", createdAt: "2026-10-03T00:00:00Z",
      projectionAvailable: true, state: "WAITING_CONFIRMATION", reason: "pending", canStart: false, canReconcile: false, canResume: false, canReview: false,
      productDetailsAvailable: false, secret: "never forward" };
    const response = await buildWorkbenchBrowserResponse(Response.json({ task }), "ai-task-read");
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ task: { id, state: "WAITING_CONFIRMATION" } });
    expect(JSON.stringify(await buildWorkbenchBrowserResponse(Response.json({ task }), "ai-task-read").then(r => r.json()))).not.toContain("never forward");
  });

  it("requires and forwards a canonical key for each Task mutation", async () => {
    for (const action of ["start", "resume", "review"] as const) {
      const body = action === "resume" ? JSON.stringify({ revision: "2", feedback: "Use the source title" }) : undefined;
      const make = (key?: string) => new Request(`http://localhost/api/workbench/tasks/${id}/${action}`, {
        method: "POST", headers: { ...scope, Origin: "http://localhost", ...(body ? { "Content-Type": "application/json" } : {}),
          ...(key ? { "Idempotency-Key": key } : {}) }, ...(body ? { body } : {}),
      });
      const path = ["tasks", id, action];
      const missing = await buildWorkbenchUpstreamRequest(make(), path, "server-token", "user-a");
      expect(missing).toBeInstanceOf(Response);
      if (missing instanceof Response) expect(missing.status).toBe(400);
      const mapped = await buildWorkbenchUpstreamRequest(make(id), path, "server-token", "user-a");
      expect(mapped).not.toBeInstanceOf(Response);
      if (!(mapped instanceof Response)) expect(new Headers(mapped.init.headers).get("Idempotency-Key")).toBe(id);
    }
  });

  it("preserves a verified organization revocation response for Chat", async () => {
    const response = await buildWorkbenchBrowserResponse(Response.json({ code: "ORGANIZATION_ACCESS_REVOKED" }, { status: 403 }), "ai-conversation-read");
    expect(response.status).toBe(403);
    expect(await response.json()).toMatchObject({ code: "ORGANIZATION_ACCESS_REVOKED" });
  });
});
