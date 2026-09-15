import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
const auth = vi.hoisted(() => ({ actor: "actor", token: "fixture-only", gate: null as Promise<void> | null }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (request: NextRequest & { auth: object }, context: unknown) => unknown) => async (request: NextRequest, context: unknown) => { if (auth.gate) await auth.gate; return handler(Object.assign(request, { auth: {} }), context); } }));
vi.mock("@/lib/server/zitadel-server-token", () => ({ readZitadelServerAccessToken: () => auth.token }));
vi.mock("@/lib/server/zitadel-auth", async (original) => ({ ...await original<typeof import("@/lib/server/zitadel-auth")>(), readZitadelIdentityFromSession: () => auth.actor ? { userId: auth.actor } : null }));
import { POST, GET } from "./route";
const key = "22222222-2222-4222-8222-22222222222b", op = "11111111-1111-4111-8111-11111111111a";
const base = "sourcing/1688/browser-captures";
const receipt = { schemaVersion: 1, operationId: op, outcome: "published", replayed: true, productKey: "crawler:1688:981645030344", publicationId: `source-run:acquisition:${op}`, catalogVersion: "1", warnings: [], missingFacts: [] };
function call(method: string, suffix: string, signal?: AbortSignal) {
  const request = new NextRequest(`http://localhost/api/workbench/${base}${suffix}`, { method, signal, headers: { cookie: "shuomi_effective_organization=org-B", origin: "http://localhost", "X-Expected-Organization-ID": "org-B", "X-Expected-User-ID": "actor", "Idempotency-Key": key, "Content-Type": "application/json" }, ...(method === "POST" ? { body: JSON.stringify(browserCaptureFixture()) } : {}) });
  return (method === "GET" ? GET : POST)(request, { params: Promise.resolve({ path: `${base}${suffix}`.split("/") }) }) as Promise<Response>;
}
beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost"));
afterEach(() => { auth.actor = "actor"; auth.token = "fixture-only"; auth.gate = null; vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
describe("real catchall Browser dispatch lifecycle", () => {
  it.each([["POST", "", 22_000], ["POST", "/verify", 22_000], ["GET", `/by-key/${key}`, 22_000], ["GET", `/${op}`, 22_000], ["GET", "/by-key/invalid", 15_000], ["POST", "/", 15_000]])("bounds blocked authentication: %s %s", async (method, suffix, budget) => {
    vi.useFakeTimers(); let release!: () => void; auth.gate = new Promise<void>((resolve) => { release = resolve; });
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher); let done = false;
    const pending = call(method, suffix).then((result) => { done = true; return result; });
    await vi.advanceTimersByTimeAsync(Number(budget) - 1); expect(done).toBe(false);
    await vi.advanceTimersByTimeAsync(1); expect(done).toBe(true); expect((await pending).status).toBeGreaterThanOrEqual(500); release(); expect(fetcher).not.toHaveBeenCalled();
  });
  it.each([["POST", ""], ["POST", "/verify"], ["GET", `/by-key/${key}`], ["GET", `/${op}`]])("forwards exact %s %s with bounded safe receipt", async (method, suffix) => {
    const fetcher = vi.fn().mockResolvedValue(Response.json(receipt)); vi.stubGlobal("fetch", fetcher);
    const response = await call(method, suffix); expect(response.status).toBe(200); expect(await response.json()).toEqual(receipt); expect(fetcher).toHaveBeenCalledTimes(1); expect(response.headers.get("cache-control")).toContain("no-store");
  });
  it("lost response after POST is unknown, not a retry", async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error("transport loss")); vi.stubGlobal("fetch", fetcher);
    const response = await call("POST", ""); expect(response.status).toBe(503); expect((await response.json()).code).toBe("OUTCOME_UNKNOWN"); expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("missing verified actor fails before dispatch", async () => {
    auth.actor = ""; const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
    expect((await call("POST", "")).status).toBe(401); expect(fetcher).not.toHaveBeenCalled();
  });
});
