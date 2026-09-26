import { afterEach, expect, it, vi } from "vitest";
import { buildWorkbenchUpstreamRequest, buildWorkbenchBrowserResponse } from "./workbench-proxy";
const op = "11111111-1111-4111-8111-111111111111", key = "22222222-2222-4222-8222-222222222222";
const path = ["sourcing", "1688", "acquisitions", op, "product-agent", "runs"];
afterEach(() => vi.unstubAllEnvs());
function req(body = '{"targetPlatform":"shein"}',  extra: Record<string, string> = {}) { return new Request(`http://localhost:3000/api/workbench/${path.join("/")}`, { method: "POST", headers: { Origin: "http://localhost:3000", "Sec-Fetch-Site": "same-origin", Cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", "X-Expected-User-ID": "actor", "Content-Type": "application/json", "Idempotency-Key": key, ...extra }, body }); }
it("admits an explicitly selected platform for Agent Start and rejects client authority and stale identity", async () => {
    vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
    const result = await buildWorkbenchUpstreamRequest(req(), path, "token", "actor");
    expect(result).not.toBeInstanceOf(Response);
    if (result instanceof Response)
        return;
    expect(result.init.body).toBe('{"targetPlatform":"shein"}');
    expect(new Headers(result.init.headers).get("Idempotency-Key")).toBe(key);
    expect(result.sourceMutation).toBe(true);
    for (const request of [req('{}'), req('{"targetPlatform":"product"}'), req('{"model":"other"}'), req('{"organizationId":"A"}'), req('{}', { "X-Expected-User-ID": "other" }), req('{}', { Origin: "http://evil.test" }), req('{}', { "Idempotency-Key": "" })]) {
        expect(await buildWorkbenchUpstreamRequest(request, path, "token", "actor")).toBeInstanceOf(Response);
    }
});
it("does not turn a mismatched result or unmounted Agent into successful generation", async () => {
    const unavailable = await buildWorkbenchBrowserResponse(Response.json({ code: "PRODUCT_AGENT_UNAVAILABLE" }, { status: 503 }), "product-agent-result", key, { sourceMutation: true });
    expect(unavailable.status).toBe(503);
    expect((await unavailable.json()).code).toBe("PRODUCT_AGENT_UNAVAILABLE");
    const wrong = await buildWorkbenchBrowserResponse(Response.json({ proposalId: op, requestKey: op, operationId: op }), "product-agent-review", key, { sourceMutation: true });
    expect(wrong.status).toBe(503);
    expect((await wrong.json()).code).toBe("OUTCOME_UNKNOWN");
});
