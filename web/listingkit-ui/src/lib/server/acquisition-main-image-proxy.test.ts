import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { buildWorkbenchBrowserResponse, buildWorkbenchUpstreamRequest } from "./workbench-proxy";

const operation = "11111111-1111-4111-8111-111111111111";
const run = "22222222-2222-4222-8222-222222222222";
const key = "33333333-3333-4333-8333-333333333333";
const base = `/api/workbench/sourcing/1688/acquisitions/${operation}/main-image`;
const segments = ["sourcing", "1688", "acquisitions", operation, "main-image"];

beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000"));
afterEach(() => vi.unstubAllEnvs());

function request(method: string, suffix = "", body?: string, extra: Record<string, string> = {}) {
  return new Request(`http://localhost:3000${base}${suffix}`, { method, headers: {
    Origin: "http://localhost:3000", "Sec-Fetch-Site": "same-origin",
    Cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", "X-Expected-User-ID": "actor",
    ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...extra,
  }, body });
}

it("forwards only the exact image candidates and same-key single-source Start", async () => {
  const candidates = await buildWorkbenchUpstreamRequest(request("GET", "/candidates"), [...segments, "candidates"], "token", "actor");
  expect(candidates).not.toBeInstanceOf(Response);
  if (candidates instanceof Response) return;
  expect(candidates.responseContract).toBe("acquisition-image-candidates");
  expect(candidates.expectedStoreId).toBe(operation);
  const start = await buildWorkbenchUpstreamRequest(request("POST", "", '{"sourceImageId":"catalog-image-2"}', { "Idempotency-Key": key }), segments, "token", "actor");
  expect(start).not.toBeInstanceOf(Response);
  if (start instanceof Response) return;
  expect(start.sourceMutation).toBe(true);
  expect(start.init.body).toBe('{"sourceImageId":"catalog-image-2"}');
  expect(new Headers(start.init.headers).get("Idempotency-Key")).toBe(key);
  for (const bad of [
    request("POST", "", '{"sourceImageId":"catalog-image-1","sourceImageId":"catalog-image-2"}', { "Idempotency-Key": key }),
    request("POST", "", '{"sourceImageId":"catalog-image-1","url":"https://example.com/a.png"}', { "Idempotency-Key": key }),
    request("POST", "", '{"sourceImageId":"catalog-image-1"}'),
    request("POST", "", '{"sourceImageId":"catalog-image-1"}', { "Idempotency-Key": key, Origin: "http://evil.test" }),
    request("POST", "", '{"sourceImageId":"catalog-image-1"}', { "Idempotency-Key": key, "X-Expected-Organization-ID": "other" }),
    request("POST", "", '{"sourceImageId":"catalog-image-1"}', { "Idempotency-Key": key, "X-Expected-User-ID": "other" }),
  ]) {
    const result = await buildWorkbenchUpstreamRequest(bad, segments, "token", "actor");
    expect(result).toBeInstanceOf(Response);
  }
  for (const [method, suffix, tail, body] of [
    ["POST", "", [], '{"sourceImageId":"catalog-image-1"}'],
    ["GET", "/candidates", ["candidates"], undefined],
    ["GET", `/runs/${run}`, ["runs", run], undefined],
    ["POST", `/runs/${run}/approve`, ["runs", run, "approve"], JSON.stringify({ planRevision: 1, resultDigest: "digest", actionId: key })],
  ] as const) {
    for (const [header, value, code] of [
      ["X-Expected-User-ID", "", "IDENTITY_CONTEXT_CHANGED"],
      ["X-Expected-User-ID", "other", "IDENTITY_CONTEXT_CHANGED"],
      ["X-Expected-Organization-ID", "", "ORGANIZATION_CONTEXT_CHANGED"],
      ["X-Expected-Organization-ID", "other", "ORGANIZATION_CONTEXT_CHANGED"],
    ]) {
      const bad = request(method, suffix, body, { "Idempotency-Key": key, [header]: value });
      const result = await buildWorkbenchUpstreamRequest(bad, [...segments, ...tail], "token", "actor");
      expect(result).toBeInstanceOf(Response);
      expect((result as Response).status).toBe(409);
      expect((await (result as Response).json()).code).toBe(code);
    }
  }
});

it("validates viewable approval result and keeps invalid mutation responses unknown", async () => {
  const projection = { runId: run, status: "awaiting_final_approval", planRevision: 1, resultDigest: "digest", approvalAvailable: true, imageUrl: "https://images.example.com/a.png" };
  const read = await buildWorkbenchBrowserResponse(Response.json(projection), "acquisition-image-result", run);
  expect(read.status).toBe(200);
  const noImage = await buildWorkbenchBrowserResponse(Response.json({ ...projection, imageUrl: undefined }), "acquisition-image-result", run);
  expect(noImage.status).toBe(502);
  const approved = await buildWorkbenchBrowserResponse(Response.json({ runId: run, status: "accepted" }, { status: 202 }), "acquisition-image-accepted", run, { sourceMutation: true });
  expect(approved.status).toBe(202);
  const wrong = await buildWorkbenchBrowserResponse(Response.json({ runId: operation, status: "accepted" }, { status: 202 }), "acquisition-image-accepted", run, { sourceMutation: true });
  expect(wrong.status).toBe(503);
  expect((await wrong.json()).code).toBe("OUTCOME_UNKNOWN");
});

it.each(["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"])("clears stale organization selection on confirmed %s", async (code) => {
  const response = await buildWorkbenchBrowserResponse(Response.json({ code }, { status: 403 }), "acquisition-image-accepted", undefined, { sourceMutation: true });
  expect(response.status).toBe(403);
  expect((await response.json()).code).toBe(code);
  expect(response.headers.get("set-cookie")).toContain("shuomi_effective_organization=;");
});
