import {afterEach,expect,it,vi} from "vitest";
import {imageSetRequest} from "./product-image-set";
import {imageSetSourcesSchema,imageSetApprovalSchema,imageSetAcceptedSchema} from "../contracts/product-image-set";
const id="11111111-1111-4111-8111-111111111111",run="22222222-2222-4222-8222-222222222222",action="33333333-3333-4333-8333-333333333333";
const scope={userId:"actor",organizationId:"org",kind:"acquisition" as const,contextId:id};
afterEach(()=>{vi.unstubAllGlobals();vi.useRealTimers()});
it.each([
 ...(["acquisition","supply"] as const).flatMap(kind=>[
  {kind,duration:31_000,cancelEarly:false,success:true},
  {kind,duration:36_000,cancelEarly:false,success:false},
  {kind,duration:31_000,cancelEarly:true,success:false},
 ]),
])("allows the image proxy window and preserves bounded cancellation: $kind $duration $cancelEarly",async({kind,duration,cancelEarly,success})=>{
 vi.useFakeTimers();const caller=new AbortController();
 const payload={contextKind:kind,contextId:id,manualReplacementAvailable:false,source:{ContextKind:kind,OperationID:id,ProductID:"p",OriginalPublicationID:"pub",OriginalVersion:"1",EffectiveVersion:"1"},originals:[],evidence:{}};
 const fetch=vi.fn<typeof globalThis.fetch>((_input,init)=>new Promise((resolve,reject)=>{
  const timer=setTimeout(()=>resolve(Response.json(payload)),duration);
  init?.signal?.addEventListener("abort",()=>{clearTimeout(timer);reject(new DOMException("aborted","AbortError"))},{once:true});
 }));vi.stubGlobal("fetch",fetch);
 let settled=false;
 const pending=imageSetRequest({...scope,kind},"sources",imageSetSourcesSchema,{signal:caller.signal}).then(value=>{settled=true;return {value,error:null}},error=>{settled=true;return {value:null,error}});
 await vi.advanceTimersByTimeAsync(cancelEarly?5_000:30_000);
 expect(settled).toBe(false);
 if(cancelEarly)caller.abort();else await vi.advanceTimersByTimeAsync(success?1_000:5_000);
 const result=await pending;
 if(success)expect(result.value).toEqual(payload);else expect(result.error).toMatchObject({code:"IMAGE_UNAVAILABLE"});
 expect(fetch).toHaveBeenCalledOnce();expect(fetch.mock.calls[0][1]?.signal?.aborted).toBe(!success);expect(vi.getTimerCount()).toBe(0);
});
it("retains UNKNOWN at the browser deadline without resending an image mutation",async()=>{
 vi.useFakeTimers();
 const fetch=vi.fn<typeof globalThis.fetch>((_input,init)=>new Promise((_resolve,reject)=>{
  init?.signal?.addEventListener("abort",()=>reject(new DOMException("aborted","AbortError")),{once:true});
 }));vi.stubGlobal("fetch",fetch);
 let settled=false;
 const pending=imageSetRequest(scope,"confirm",imageSetAcceptedSchema,{runId:run,body:{actionId:action,planRevision:1,planDigest:"a".repeat(64),quoteDigest:"b".repeat(64)}}).then(()=>{settled=true;return null},error=>{settled=true;return error});
 await vi.advanceTimersByTimeAsync(34_999);expect(settled).toBe(false);
 await vi.advanceTimersByTimeAsync(1);expect(await pending).toMatchObject({code:"OUTCOME_UNKNOWN",status:503});
 expect(fetch).toHaveBeenCalledOnce();expect(vi.getTimerCount()).toBe(0);
});
it("rejects cross-context sources even when their schema is valid",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json({contextKind:"acquisition",contextId:run,manualReplacementAvailable:false,source:{ContextKind:"acquisition",OperationID:run,ProductID:"p",OriginalPublicationID:"pub",OriginalVersion:"1",EffectiveVersion:"1"},originals:[],evidence:{}})));
 await expect(imageSetRequest(scope,"sources",imageSetSourcesSchema)).rejects.toMatchObject({code:"CONTEXT_CHANGED"});
});
it("rejects another approval action without issuing any mutation",async()=>{
 const fetch=vi.fn().mockResolvedValue(Response.json({actionId:id,selectionDigest:"a".repeat(64),assets:[{id:"asset",role:"main",url:"https://images.test/1.png"}]}));vi.stubGlobal("fetch",fetch);
 await expect(imageSetRequest(scope,"approval",imageSetApprovalSchema,{runId:run,approvalId:action})).rejects.toMatchObject({code:"CONTEXT_CHANGED"});
 expect(fetch).toHaveBeenCalledTimes(1);expect(fetch.mock.calls[0][1].method).toBe("GET");
});
