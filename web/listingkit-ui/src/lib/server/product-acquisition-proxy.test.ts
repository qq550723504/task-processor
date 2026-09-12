import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildWorkbenchBrowserResponse, buildWorkbenchUpstreamRequest } from "./workbench-proxy";
import { ACQUISITION_BASE, acquisitionResultSchema, canonical1688Source } from "../contracts/product-acquisition";

const key = "01991e24-61ab-4f5f-85d1-3157bb8b75c1";
const path = ["sourcing", "1688", "acquisitions"];
beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000"));
afterEach(() => vi.unstubAllEnvs());
const result = { schemaVersion: 1, operationId: key, outcome: "published", replayed: false,
  productKey: "crawler:1688:981645030344", publicationId: `source-run:acquisition:${key}`, catalogVersion: "1", warnings: [], missingFacts: [] };
function request(body = '{"source":"981645030344"}', extra: Record<string,string> = {}) {
  return new Request(`http://localhost:3000${ACQUISITION_BASE}`, { method: "POST", headers: {
    "content-type": "application/json", origin: "http://localhost:3000", "sec-fetch-site": "same-origin",
    cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", "X-Expected-User-ID": "actor",
    "Idempotency-Key": key, ...extra,
  }, body });
}

describe("Public acquisition BFF", () => {
  it("admits only canonical source and unchanged explicit operation key", async () => {
    expect(canonical1688Source("http://DETAIL.1688.COM/offer/981645030344.html?x=1")).toBe("https://detail.1688.com/offer/981645030344.html");
    expect(canonical1688Source("https://detail.1688.com/a/../offer/1.html")).toBeNull();
    const built = await buildWorkbenchUpstreamRequest(request(), path, "server-token", "actor");
    expect(built).not.toBeInstanceOf(Response);
    if (built instanceof Response) return;
    expect(built.url).toMatch(/\/workbench\/sourcing\/1688\/acquisitions$/);
    const headers = new Headers(built.init.headers);
    expect(headers.get("Authorization")).toBe("Bearer server-token");
    expect(headers.get("X-Requested-Organization-ID")).toBe("B");
    expect(headers.get("Idempotency-Key")).toBe(key);
    expect(headers.get("Cookie")).toBeNull(); expect(headers.get("X-Expected-User-ID")).toBeNull();
    expect(built.init.body).toBe('{"source":"981645030344"}'); expect(built.sourceMutation).toBe(true);
  });
  it("rejects duplicate/unknown fields and identity/org/origin drift before dispatch", async () => {
    for (const req of [request('{"source":"1","source":"2"}'), request('{"source":"1","envelope":{}}'),
      request(undefined, {"X-Expected-User-ID":"other"}),request(undefined,{"X-Expected-Organization-ID":"A"}),request(undefined,{origin:"http://evil.test"})]) {
      const built = await buildWorkbenchUpstreamRequest(req,path,"server-token","actor");
      expect(built).toBeInstanceOf(Response);expect((built as Response).status).toBeGreaterThanOrEqual(400);
    }
  });
  it("projects exact safe result and keeps invalid dispatched outcomes unknown", async () => {
    const built = await buildWorkbenchUpstreamRequest(request(),path,"server-token","actor");
    expect(built).not.toBeInstanceOf(Response);if (built instanceof Response) return;
    const good = await buildWorkbenchBrowserResponse(Response.json(result),built.responseContract,undefined,{sourceMutation:true});
    expect(good.status).toBe(200);expect(await good.json()).toEqual(result);
    expect(acquisitionResultSchema.safeParse({...result,catalogVersion:1}).success).toBe(false);
    expect(acquisitionResultSchema.safeParse({...result,publicationId:"other"}).success).toBe(false);
    const invalid = await buildWorkbenchBrowserResponse(Response.json({...result,command:"secret"}),built.responseContract,undefined,{sourceMutation:true});
    expect(invalid.status).toBe(503);expect((await invalid.json()).code).toBe("OUTCOME_UNKNOWN");
    const failure = await buildWorkbenchBrowserResponse(Response.json({schemaVersion:1,error:{code:"OUTCOME_UNKNOWN"}},{status:503}),built.responseContract,undefined,{sourceMutation:true});
    expect(failure.status).toBe(503);expect((await failure.json()).code).toBe("OUTCOME_UNKNOWN");
  });
});
