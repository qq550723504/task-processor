import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
import { buildWorkbenchUpstreamRequest, buildWorkbenchBrowserResponse } from "./workbench-proxy";

const key = "22222222-2222-4222-8222-22222222222b";
const op = "11111111-1111-4111-8111-11111111111a";
const base = "sourcing/1688/browser-captures";
beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost"));
afterEach(() => vi.unstubAllEnvs());
function request(method = "POST", suffix = "", body = JSON.stringify(browserCaptureFixture()), change?: (h: Headers) => void) {
  const headers = new Headers({ cookie: "shuomi_effective_organization=org-B", "X-Expected-Organization-ID": "org-B", "X-Expected-User-ID": "actor", origin: "http://localhost", "content-type": "application/json", "Idempotency-Key": key });
  change?.(headers);
  return new Request(`http://localhost/api/workbench/${base}${suffix}`, { method, headers, ...(method === "POST" ? { body } : {}) });
}
async function build(req: Request, suffix = "") { return buildWorkbenchUpstreamRequest(req, `${base}${suffix}`.split("/"), "server-private-token", "actor"); }
describe("Browser BFF four exact routes", () => {
  it.each([["POST", ""], ["POST", "/verify"], ["GET", `/by-key/${key}`], ["GET", `/${op}`]])("admits %s %s without trusting payload authority", async (method, suffix) => {
    const raw = JSON.stringify(browserCaptureFixture(), null, 2);
    const result = await build(request(method, suffix, raw), suffix);
    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response) return;
    const headers = new Headers(result.init.headers);
    expect(headers.get("Authorization")).toBe("Bearer server-private-token");
    expect(headers.get("X-Requested-Organization-ID")).toBe("org-B");
    expect(result.url).toContain(`/workbench/${base}${suffix}`);
    expect(result.init.body).toBe(method === "POST" ? raw : undefined);
    expect(result.expectedStoreId).toBe(suffix === `/${op}` ? op : undefined);
    expect(result.sourceMutation).toBe(method === "POST");
  });
  it.each(["/", "/child", `/by-key/${key}/child`, "/by-key/not-a-key", "/verify/", "/by-key", `/../browser-captures/${op}`])("denies non-contract path %s", async (suffix) => {
    expect(await build(request("GET", suffix), suffix)).toBeInstanceOf(Response);
  });
  it.each([
    ["duplicate", '{"captureVersion":1,' + JSON.stringify(browserCaptureFixture()).slice(1)],
    ["unknown", JSON.stringify({ ...browserCaptureFixture(), tenant: "other" })],
    ["too large", " ".repeat(2 * 1024 * 1024 + 1)],
    ["surrogate", JSON.stringify({ ...browserCaptureFixture(), evidence: { ...browserCaptureFixture().evidence, title: "\ud800" } })],
  ])("rejects %s before upstream", async (_, body) => {
    const result = await build(request("POST", "", body));
    expect(result).toBeInstanceOf(Response);
    expect((result as Response).status).toBeGreaterThanOrEqual(400);
  });
  it.each([
    (h: Headers) => h.set("X-Expected-User-ID", "other"),
    (h: Headers) => h.set("X-Expected-Organization-ID", "org-C"),
    (h: Headers) => h.set("cookie", "shuomi_effective_organization=org-B; shuomi_effective_organization=org-C"),
    (h: Headers) => h.set("origin", "https://evil.invalid"),
    (h: Headers) => h.set("content-encoding", "gzip"),
    (h: Headers) => h.set("Idempotency-Key", key.toUpperCase()),
  ])("rejects context or envelope substitution %#", async (change) => {
    expect(await build(request("POST", "", undefined, change))).toBeInstanceOf(Response);
  });
  it.each(["?", "?org=org-C"])('denies any query "%s"', async (query) => {
    const original = request();
    const req = new Request(original.url + query, original);
    expect(await build(req)).toBeInstanceOf(Response);
  });
  it("does not confuse original key with returned operation ID and rejects mismatched ID", async () => {
    const receipt = { schemaVersion: 1, operationId: op, outcome: "published", replayed: true, productKey: "crawler:1688:981645030344", publicationId: `source-run:acquisition:${op}`, catalogVersion: "1", warnings: [], missingFacts: [] };
    expect((await buildWorkbenchBrowserResponse(Response.json(receipt), "product-acquisition")).status).toBe(200);
    expect((await buildWorkbenchBrowserResponse(Response.json(receipt), "product-acquisition", key)).status).toBe(502);
    const failure = await buildWorkbenchBrowserResponse(new Response("lost", { status: 502 }), "product-acquisition", undefined, { sourceMutation: true });
    expect(failure.status).toBe(503);
    expect((await failure.json()).code).toBe("OUTCOME_UNKNOWN");
  });
});
