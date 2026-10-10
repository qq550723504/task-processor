import {afterEach,expect,it,vi} from "vitest";
import {imageSetRequest} from "./product-image-set";
import {imageSetSourcesSchema,imageSetApprovalSchema} from "../contracts/product-image-set";
const id="11111111-1111-4111-8111-111111111111",run="22222222-2222-4222-8222-222222222222",action="33333333-3333-4333-8333-333333333333";
const scope={userId:"actor",organizationId:"org",kind:"acquisition" as const,contextId:id};
afterEach(()=>vi.unstubAllGlobals());
it("rejects cross-context sources even when their schema is valid",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json({contextKind:"acquisition",contextId:run,manualReplacementAvailable:false,source:{ContextKind:"acquisition",OperationID:run,ProductID:"p",OriginalPublicationID:"pub",OriginalVersion:"1",EffectiveVersion:"1"},originals:[],evidence:{}})));
 await expect(imageSetRequest(scope,"sources",imageSetSourcesSchema)).rejects.toMatchObject({code:"CONTEXT_CHANGED"});
});
it("rejects another approval action without issuing any mutation",async()=>{
 const fetch=vi.fn().mockResolvedValue(Response.json({actionId:id,selectionDigest:"a".repeat(64),assets:[{id:"asset",role:"main",url:"https://images.test/1.png"}]}));vi.stubGlobal("fetch",fetch);
 await expect(imageSetRequest(scope,"approval",imageSetApprovalSchema,{runId:run,approvalId:action})).rejects.toMatchObject({code:"CONTEXT_CHANGED"});
 expect(fetch).toHaveBeenCalledTimes(1);expect(fetch.mock.calls[0][1].method).toBe("GET");
});
