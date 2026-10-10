import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {proxyPOD} from "./pod-proxy";
const id="12752596-6056-4316-9f2f-380c97df9675";
function request(path:string,init:RequestInit={}){return new Request("https://console.example/api/workbench/pod/"+path,{...init,headers:{"X-Expected-User-ID":"user-a","X-Expected-Organization-ID":"org-a",cookie:"shuomi_effective_organization=org-a",...init.headers}})}
beforeEach(()=>{vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","https://api.example/api/v1");vi.stubEnv("LISTINGKIT_SDS_POD_ENABLED","true");vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","https://console.example")});afterEach(()=>vi.unstubAllGlobals());
it("rejects tenant drift and unrecognized provider parameters without dispatch",async()=>{const fetcher=vi.fn();vi.stubGlobal("fetch",fetcher);expect((await proxyPOD(request("templates",{headers:{"X-Expected-Organization-ID":"other"}}),"private-token","user-a")).status).toBe(409);expect((await proxyPOD(request("templates?account=other"),"private-token","user-a")).status).toBe(400);expect(fetcher).not.toHaveBeenCalled()});
it("retains uncertain sent operation and never returns provider private fields",async()=>{const fetcher=vi.fn(async()=>Response.json({id,name:"saved",state:"SAVED",createdAt:"2026-10-10T00:00:00Z",credential:"secret"}));vi.stubGlobal("fetch",fetcher);const r=await proxyPOD(request("designs/"+id+"/select",{method:"POST",headers:{Origin:"https://console.example","Content-Type":"application/json","Idempotency-Key":id},body:"{}"}),"private-token","user-a");expect(fetcher).toHaveBeenCalledTimes(1);expect(await r.json()).toEqual({code:"OUTCOME_UNKNOWN"})});
it("accepts only the original approval action after a sent write",async()=>{
 const body={actionId:id,selection:{itemId:id,originalPublicationId:"publication-a",originalSnapshotVersion:1,effectiveCatalogVersion:1,targetPlatform:"sds"},images:[{id,role:"design"}],approved:[]};
 for(const actionId of ["b99c0426-fc19-4375-87f7-460d4b8180be",id]){
  const receipt={actionId,assetIds:["asset-a"],image:{url:"https://images.example/art.png",hash:"a".repeat(64),width:1334,height:2000}};
  const fetcher=vi.fn(async()=>Response.json(receipt));vi.stubGlobal("fetch",fetcher);
  const result=await proxyPOD(request("approvals",{method:"POST",headers:{Origin:"https://console.example","Content-Type":"application/json","Idempotency-Key":id},body:JSON.stringify(body)}),"private-token","user-a");
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(await result.json()).toEqual(actionId===id?receipt:{code:"OUTCOME_UNKNOWN"});
 }
});
