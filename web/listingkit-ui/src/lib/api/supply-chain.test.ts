import { afterEach,expect,it,vi } from "vitest";
import { supplyCommand,supplyInventory,readSupplyCommand,listSupplyOperationItems,type SupplyIntent } from "./supply-chain";
import { titleProposalFixture } from "@/test/product-title-review-fixture";
const id="11111111-1111-4111-8111-111111111111",other="22222222-2222-4222-8222-222222222222";
const intent:SupplyIntent={userId:"actor",organizationId:"org",key:id,route:"create-operation",command:{preparationId:id,expectedRevision:1,storeId:id,action:"adapt"}};
const operation={id,input:intent.command,count:205,completed:0,status:"pending",execution:"pending",createdAt:"2026-10-08T00:00:00Z"};
const reply=(v:unknown)=>new Response(JSON.stringify(v),{headers:{"Content-Type":"application/json"}});
afterEach(()=>vi.unstubAllGlobals());

it("reads approved inventory for the exact signed-64-bit catalog version",async()=>{
 const version="9223372036854775807";
 const fetch=vi.fn().mockResolvedValue(reply({scope:{tenant_id:"org",product_key:"product",target_platform:"shein",source_snapshot_version:version},assets:[{id:"approved",role:"main",url:"https://images.test/approved.png"}]}));vi.stubGlobal("fetch",fetch);
 const result=await supplyInventory(intent,{itemId:id,originalPublicationId:id,originalSnapshotVersion:version,effectiveCatalogVersion:version,targetPlatform:"shein"});
 expect(result.scope.source_snapshot_version).toBe(version);
 expect(JSON.parse(String(fetch.mock.calls[0][1].body))).toMatchObject({originalSnapshotVersion:version,effectiveCatalogVersion:version});
 expect(result.assets[0].id).toBe("approved");
});
it("retains an uncertain write when a structurally valid result belongs to another store",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({operation:{...operation,input:{...operation.input as object,storeId:other}},replayed:false})));
 await expect(supplyCommand(intent)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
});
it("checks the original full selection when recovering an operation",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({...operation,input:{...operation.input as object,sourceIds:[other]}})));
 await expect(readSupplyCommand(intent)).rejects.toMatchObject({code:"DEPENDENCY_UNAVAILABLE"});
});
it("accepts a retained original operation with its canonical sorted source set",async()=>{
 const selected={...intent,command:{...intent.command as object,sourceIds:[other,id]}};
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({...operation,input:{...operation.input as object,sourceIds:[id,other]}})));
 await expect(readSupplyCommand(selected)).resolves.toMatchObject({id});
});
it("rejects an approval receipt for a different original command key",async()=>{
 const approval:SupplyIntent={...intent,route:"approve",command:{selection:{itemId:id,originalPublicationId:id,originalSnapshotVersion:"1",effectiveCatalogVersion:"1",targetPlatform:"shein"},images:[{id,role:"main"}],approved:[]}};
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({action_id:other,asset_ids:["source-image"]})));
 await expect(supplyCommand(approval)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
});
it("uses the original Review owner and retains its exact consent key for recovery",async()=>{
 const proposal={...titleProposalFixture(),proposal_id:id,owner:"actor",state:"accepted",revision:"2",input:{product_key:"product",base_version:"1"}};
 const review={...intent,route:"review-decision",command:{proposalId:id,sourceId:id,recordId:id,productKey:"product",baseVersion:"1",input:{action:"accept",expected_revision:"1"}}} as SupplyIntent;
 const fetcher=vi.fn().mockImplementation(()=>Promise.resolve(reply(proposal)));vi.stubGlobal("fetch",fetcher);
 await expect(supplyCommand(review)).resolves.toMatchObject({proposal_id:id,state:"accepted"});
 await expect(readSupplyCommand(review)).resolves.toMatchObject({proposal_id:id,state:"accepted"});
 expect(fetcher.mock.calls[0][0]).toBe(`/api/product/text-proposals/${id}/decisions`);
 expect(fetcher.mock.calls[1][1].headers["Idempotency-Key"]).toBe(id);
});
it("preserves an uncertain review result when the owner or baseline does not match",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({...titleProposalFixture(),proposal_id:id,owner:"another",state:"accepted",revision:"2",input:{product_key:"product",base_version:"1"}})));
 const review={...intent,route:"review-decision",command:{proposalId:id,sourceId:id,recordId:id,productKey:"product",baseVersion:"1",input:{action:"accept",expected_revision:"1"}}} as SupplyIntent;
 await expect(supplyCommand(review)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
});

it("resolves the original upload with a readback request and preserves uncertain results",async()=>{
 const resolve:SupplyIntent={...intent,route:"resolve-upload",command:{recordId:id,attemptId:other,spu:"spu-a"}};
 const fetcher=vi.fn().mockImplementation(()=>Promise.resolve(reply({recordId:id,attemptId:other,status:"outcome_unknown",message:"待核实"})));vi.stubGlobal("fetch",fetcher);
 await expect(supplyCommand(resolve)).resolves.toMatchObject({status:"outcome_unknown"});await expect(readSupplyCommand(resolve)).resolves.toMatchObject({status:"outcome_unknown"});
 expect(fetcher.mock.calls[0][0]).toMatch(/uploads\/resolve$/);expect(fetcher.mock.calls[0][1].headers["Idempotency-Key"]).toBeUndefined();expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual(resolve.command);
});
it("rejects a readback response for a different original attempt",async()=>{
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({recordId:id,attemptId:id,status:"outcome_unknown"})));
 await expect(supplyCommand({...intent,route:"resolve-upload",command:{recordId:id,attemptId:other,spu:"spu-a"}})).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
});

it("reads original UNKNOWN history with confirmed provider facts and rejects success claims without an original attempt",async()=>{
 const item={sourceId:id,recordId:id,recordRevision:1,status:"unknown",resultReference:other,confirmedProduct:{spu_name:"spu-a",skc_list:[{skc_name:"skc-a",sku_list:[{sku_code:"sku-a",supplier_sku:"original-sku"}]}]}};
 const fetcher=vi.fn().mockImplementation(()=>Promise.resolve(reply({items:[item],total:1})));vi.stubGlobal("fetch",fetcher);
 await expect(listSupplyOperationItems({userId:"actor",organizationId:"org"},id,{limit:100})).resolves.toMatchObject({items:[item]});
 fetcher.mockImplementation(()=>Promise.resolve(reply({items:[{...item,resultReference:"not-an-attempt"}],total:1})));
 await expect(listSupplyOperationItems({userId:"actor",organizationId:"org"},id,{limit:100})).rejects.toMatchObject({code:"DEPENDENCY_UNAVAILABLE"});
});
