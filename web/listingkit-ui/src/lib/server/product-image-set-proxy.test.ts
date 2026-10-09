import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {buildWorkbenchBrowserResponse,buildWorkbenchUpstreamRequest} from "./workbench-proxy";
const context="11111111-1111-4111-8111-111111111111",run="22222222-2222-4222-8222-222222222222",action="33333333-3333-4333-8333-333333333333";
const path=["sourcing","1688","acquisitions",context,"images"];
const expected=JSON.stringify({kind:"acquisition",contextId:context,runId:run});
beforeEach(()=>vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost:3000"));
afterEach(()=>vi.unstubAllEnvs());
function request(method:string,suffix:string,body?:string,extra:Record<string,string>={}){
 return new Request(`http://localhost:3000/api/workbench/${path.join("/")}${suffix}`,{method,headers:{Origin:"http://localhost:3000","Sec-Fetch-Site":"same-origin",Cookie:"shuomi_effective_organization=B","X-Expected-Organization-ID":"B","X-Expected-User-ID":"actor",...body?{"Content-Type":"application/json"}:{},...extra},body});
}
it("binds the full image endpoints to the source owner, original run and same preparation key",async()=>{
 const read=await buildWorkbenchUpstreamRequest(request("GET",`/runs/${run}`),[...path,"runs",run],"token","actor");
 expect(read).not.toBeInstanceOf(Response);if(read instanceof Response)return;
 expect(read.expectedStoreId).toBe(expected);
 const body=JSON.stringify({target:{Platform:"product"},sharedOriginalIds:["original-1"],selectedTaskIds:["main-identity","detail-closeup"]});
 const prepared=await buildWorkbenchUpstreamRequest(request("POST","/prepare",body,{"Idempotency-Key":action}),[...path,"prepare"],"token","actor");
 expect(prepared).not.toBeInstanceOf(Response);if(prepared instanceof Response)return;
 expect(prepared.sourceMutation).toBe(true);expect(new Headers(prepared.init.headers).get("Idempotency-Key")).toBe(action);
 for(const invalid of [body.replace('"product"','"product","url":"https://evil.test/image"'),'{"target":{"Platform":"product"},"target":{"Platform":"shein"}}']){
  expect(await buildWorkbenchUpstreamRequest(request("POST","/prepare",invalid,{"Idempotency-Key":action}),[...path,"prepare"],"token","actor")).toBeInstanceOf(Response);
 }
});
it("rejects a different source owner and preserves uncertainty for a mismatched accepted run",async()=>{
 const source={contextKind:"supply",contextId:context,manualReplacementAvailable:false,source:{ContextKind:"supply",ProductID:"p",OperationID:context,OriginalPublicationID:"pub",OriginalVersion:1,EffectiveVersion:1},originals:[],evidence:{}};
 const badSource=await buildWorkbenchBrowserResponse(Response.json(source),"image-set-sources",JSON.stringify({kind:"acquisition",contextId:context}));
 expect(badSource.status).toBe(502);
 const badRun=await buildWorkbenchBrowserResponse(Response.json({runId:context,status:"accepted"},{status:202}),"image-set-confirm",expected,{sourceMutation:true});
 expect(badRun.status).toBe(503);expect((await badRun.json()).code).toBe("OUTCOME_UNKNOWN");
 const good=await buildWorkbenchBrowserResponse(Response.json({runId:run,status:"accepted"},{status:202}),"image-set-confirm",expected,{sourceMutation:true});expect(good.status).toBe(202);
});
it("verifies the immutable approval action instead of inferring it from a current inventory",async()=>{
 const binding=JSON.stringify({kind:"acquisition",contextId:context,runId:run,approvalId:action});
 const payload={actionId:context,selectionDigest:"a".repeat(64),assets:[{id:"asset",role:"main",url:"https://images.test/1.png"}]};
 expect((await buildWorkbenchBrowserResponse(Response.json(payload),"image-set-approval",binding)).status).toBe(502);
 expect((await buildWorkbenchBrowserResponse(Response.json({...payload,actionId:action}),"image-set-approval",binding)).status).toBe(200);
});
