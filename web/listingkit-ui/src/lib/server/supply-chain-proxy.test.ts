import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildWorkbenchUpstreamRequest, buildWorkbenchBrowserResponse } from "./workbench-proxy";
const id="550e8400-e29b-41d4-a716-446655440000";
const scope={cookie:"shuomi_effective_organization=org-a","X-Expected-Organization-ID":"org-a","X-Expected-User-ID":"actor-a"};
describe("private supply chain proxy",()=>{
 beforeEach(()=>vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost"));
 it("binds a full batch transfer to the verified original actor",async()=>{
  const request=new Request("http://localhost/api/workbench/supply-preparations/transfer",{method:"POST",headers:{...scope,Origin:"http://localhost","Content-Type":"application/json","Idempotency-Key":id},body:JSON.stringify({batchId:id,expectedRevision:1})});
  const result=await buildWorkbenchUpstreamRequest(request,["supply-preparations","transfer"],"server-token","actor-a");
  expect(result).not.toBeInstanceOf(Response);if(result instanceof Response)return;
  expect(result.url).toMatch(/\/api\/v1\/workbench\/supply-preparations\/transfer$/);expect(result.sourceMutation).toBe(true);
  expect(new Headers(result.init.headers).get("X-Requested-Organization-ID")).toBe("org-a");
 });
 it("strips private material from a retained transfer receipt",async()=>{
  const upstream=Response.json({preparation:{id,sourceBatchId:id,sourceRevision:1,name:"测试批次",count:205,revision:1,createdAt:"2026-10-08T00:00:00Z",owner:"private",credential:"private"},replayed:true,credential:"private"});
  const response=await buildWorkbenchBrowserResponse(upstream,"supply-transfer",undefined,{sourceMutation:true});
  expect(response.status).toBe(200);expect(await response.text()).not.toContain("private");
 });
 it("rejects a different expected actor before any upload",async()=>{
  const request=new Request(`http://localhost/api/workbench/supply-preparations/operations/${id}/ensure`,{method:"POST",headers:{...scope,"X-Expected-User-ID":"other",Origin:"http://localhost"}});
  const result=await buildWorkbenchUpstreamRequest(request,["supply-preparations","operations",id,"ensure"],"server-token","actor-a");expect(result).toBeInstanceOf(Response);if(result instanceof Response)expect(result.status).toBe(409);
 });
 it("binds official resolution to the current actor and rejects browser evidence",async()=>{
  const path=["supply-preparations","uploads","resolve"];
  const input={recordId:id,attemptId:id,spu:"spu-a"};
  const request=(body:unknown,origin="http://localhost")=>new Request("http://localhost/api/workbench/supply-preparations/uploads/resolve",{method:"POST",headers:{...scope,Origin:origin,"Content-Type":"application/json"},body:JSON.stringify(body)});
  const safe=await buildWorkbenchUpstreamRequest(request(input),path,"server-token","actor-a");expect(safe).not.toBeInstanceOf(Response);if(safe instanceof Response)return;expect(safe.sourceMutation).toBe(true);expect(new Headers(safe.init.headers).get("Idempotency-Key")).toBeNull();
  for(const r of [request({...input,evidence:{outcome:"succeeded"}}),request(input,"https://another.example")]){const rejected=await buildWorkbenchUpstreamRequest(r,path,"server-token","actor-a");expect(rejected).toBeInstanceOf(Response);if(rejected instanceof Response)expect(rejected.status).toBeGreaterThanOrEqual(400)}
 });
 it("bounds whole-batch stage queries before dispatch",async()=>{
  for(const query of [`storeId=${id}&stage=ready&stage=review`,`storeId=${id}&stage=fake`,`storeId=${id}&stage=ready&limit=101`]){
   const result=await buildWorkbenchUpstreamRequest(new Request(`http://localhost/api/workbench/supply-preparations/${id}/stages?${query}`,{headers:scope}),["supply-preparations",id,"stages"],"server-token","actor-a");expect(result).toBeInstanceOf(Response);if(result instanceof Response)expect(result.status).toBe(400);
  }
 });
});
