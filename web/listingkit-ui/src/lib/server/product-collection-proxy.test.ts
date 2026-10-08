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
  it("forwards bounded file bytes with the current actor and rejects an account switch",async()=>{
    const bytes=new Uint8Array([80,75,3,4]);
    for(const [suffix,type,hash] of [["imports/preview","application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",undefined],["media","application/octet-stream","a".repeat(64)]] as const){
      const headers={...scope,Origin:"http://localhost","Content-Type":type,...hash?{"X-Content-SHA256":hash}:{}};
      const result=await buildWorkbenchUpstreamRequest(new Request(`http://localhost/api/workbench/collections/${suffix}`,{method:"POST",headers,body:bytes}),["collections",...suffix.split("/")],"server-token","actor-a");
      expect(result).not.toBeInstanceOf(Response);if(result instanceof Response)continue;
      expect(result.init.body).toBeInstanceOf(Blob);expect(Array.from(new Uint8Array(await (result.init.body as Blob).arrayBuffer()))).toEqual([...bytes]);
      expect(result.sourceMutation).toBe(!!hash);
      const stale=await buildWorkbenchUpstreamRequest(new Request(`http://localhost/api/workbench/collections/${suffix}`,{method:"POST",headers:{...headers,"X-Expected-User-ID":"old"},body:bytes}),["collections",...suffix.split("/")],"server-token","actor-a");
      expect(stale).toBeInstanceOf(Response);if(stale instanceof Response)expect(stale.status).toBe(409);
    }
    const over=await buildWorkbenchUpstreamRequest(new Request("http://localhost/api/workbench/collections/media",{method:"POST",headers:{...scope,Origin:"http://localhost","Content-Type":"application/octet-stream","X-Content-SHA256":"a".repeat(64)},body:new Uint8Array(3*1024*1024+1)}),["collections","media"],"server-token","actor-a");
    expect(over).toBeInstanceOf(Response);if(over instanceof Response)expect(over.status).toBe(413);
  });
  it("preserves the original media hash and byte count through upload and refresh verification",async()=>{
    const hash="a".repeat(64),bytes=new Uint8Array([255,216,255,217]);
    const image={hash,bytes:bytes.length,url:"https://images.example.org/product.jpg",mediaType:"image/jpeg",width:900,height:900};
    for(const method of ["POST","GET"]){
      const path=method==="POST"?["collections","media"]:["collections","media",hash];
      const suffix=method==="POST"?"media":`media/${hash}?bytes=${bytes.length}`;
      const headers=method==="POST"?{...scope,Origin:"http://localhost","Content-Type":"application/octet-stream","X-Content-SHA256":hash}:scope;
      const mapped=await buildWorkbenchUpstreamRequest(new Request(`http://localhost/api/workbench/collections/${suffix}`,{method,headers,...method==="POST"?{body:bytes}:{}}),path,"server-token","actor-a");
      expect(mapped).not.toBeInstanceOf(Response);if(mapped instanceof Response)continue;
      expect(mapped.expectedStoreId).toBe(`${hash}:${bytes.length}`);
      const result=await buildWorkbenchBrowserResponse(Response.json(image),mapped.responseContract,mapped.expectedStoreId,{sourceMutation:mapped.sourceMutation});
      expect(result.status).toBe(200);expect(await result.json()).toEqual(image);
      const wrong=await buildWorkbenchBrowserResponse(Response.json({...image,bytes:bytes.length+1}),mapped.responseContract,mapped.expectedStoreId,{sourceMutation:mapped.sourceMutation});
      expect(wrong.status).toBe(method==="POST"?503:502);
    }
  });
});
