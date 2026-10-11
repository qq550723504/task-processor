import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {proxySupplyMarket} from "./supply-market-proxy";
const id="12752596-6056-4316-9f2f-380c97df9675";
const request=(path:string,init:RequestInit={})=>new Request("https://console.example/api/workbench/supply-market/"+path,{...init,headers:{"X-Expected-User-ID":"user-a","X-Expected-Organization-ID":"org-a",cookie:"shuomi_effective_organization=org-a",...init.headers}});
describe("market BFF disclosure and uncertain writes",()=>{
 beforeEach(()=>{vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","https://api.example/api/v1");vi.stubEnv("LISTINGKIT_SUPPLY_MARKET_ENABLED","true");vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","https://console.example")});
 afterEach(()=>vi.unstubAllGlobals());
 it("proxies the member overview with strict facts and rejects query/admin/private fields",async()=>{
  const overview={eligibleProducts:54,reviewing:27,supplementRequired:4,approved:7,products:[]};const fetcher=vi.fn(async()=>Response.json(overview));vi.stubGlobal("fetch",fetcher);
  const result=await proxySupplyMarket(request("application-overview"),"private-token","user-a");expect(result.status).toBe(200);expect(await result.json()).toEqual(overview);
  expect((await proxySupplyMarket(request("application-overview?ended=true"),"private-token","user-a")).status).toBe(400);
  const admin=request("application-overview");expect((await proxySupplyMarket(new Request(admin.url.replace("/workbench/","/admin/"),{headers:admin.headers}),"private-token","user-a")).status).toBe(400);
  expect(fetcher).toHaveBeenCalledTimes(1);fetcher.mockResolvedValueOnce(Response.json({...overview,organizationId:"secret"}));expect((await proxySupplyMarket(request("application-overview"),"private-token","user-a")).status).toBe(502);
 });
 it("rejects scope drift before reading any upstream data",async()=>{
  const fetcher=vi.fn();vi.stubGlobal("fetch",fetcher);
  const r=await proxySupplyMarket(request("releases",{headers:{"X-Expected-Organization-ID":"org-b"}}),"private-token","user-a");
  expect(r.status).toBe(409);expect(fetcher).not.toHaveBeenCalled();
 });
 it("does not forward private fields from a released product",async()=>{
  vi.stubGlobal("fetch",vi.fn(async()=>Response.json({items:[{id,channel:"official",revision:1,active:true,publishedAt:"2026-10-10T00:00:00Z",product:{title:"Released",images:["https://images.example/p.png"]},supply:{stock:1,minimumQuantity:1,leadDays:1,province:"浙江",city:"杭州"},source:{organizationId:"secret"}}],total:1})));
  const r=await proxySupplyMarket(request("releases"),"private-token","user-a");
  expect(r.status).toBe(502);expect(await r.text()).not.toContain("secret");
 });
 it("retains UNKNOWN when a sent selection response is malformed",async()=>{
  const fetcher=vi.fn(async()=>new Response("{",{headers:{"Content-Type":"application/json"}}));vi.stubGlobal("fetch",fetcher);
  const r=await proxySupplyMarket(request("select",{method:"POST",headers:{Origin:"https://console.example","Content-Type":"application/json","Idempotency-Key":id,"If-Match":'"1"'},body:JSON.stringify({action:"select_release",id,expectedRevision:1})}),"private-token","user-a");
  expect(fetcher).toHaveBeenCalledTimes(1);expect(await r.json()).toEqual({code:"OUTCOME_UNKNOWN"});
 });
 it("rejects duplicate or unknown command fields before dispatch",async()=>{
  const fetcher=vi.fn();vi.stubGlobal("fetch",fetcher);
  const r=await proxySupplyMarket(request("commands",{method:"POST",headers:{Origin:"https://console.example","Content-Type":"application/json","Idempotency-Key":id},body:'{"action":"submit_connection","action":"submit_connection","organizationId":"other"}'}),"private-token","user-a");
  expect(r.status).toBe(400);expect(fetcher).not.toHaveBeenCalled();
 });
});
