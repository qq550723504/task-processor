import { afterEach,expect,it,vi } from "vitest";
import { supplyCommand,readSupplyCommand,type SupplyIntent } from "./supply-chain";
const id="11111111-1111-4111-8111-111111111111",other="22222222-2222-4222-8222-222222222222";
const intent:SupplyIntent={userId:"actor",organizationId:"org",key:id,route:"create-operation",command:{preparationId:id,expectedRevision:1,storeId:id,action:"adapt"}};
const operation={id,input:intent.command,count:205,completed:0,status:"pending",execution:"pending",createdAt:"2026-10-08T00:00:00Z"};
const reply=(v:unknown)=>new Response(JSON.stringify(v),{headers:{"Content-Type":"application/json"}});
afterEach(()=>vi.unstubAllGlobals());
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
 const approval:SupplyIntent={...intent,route:"approve",command:{selection:{itemId:id,originalPublicationId:id,originalSnapshotVersion:1,effectiveCatalogVersion:1,targetPlatform:"shein"},images:[{id,role:"main"}],approved:[]}};
 vi.stubGlobal("fetch",vi.fn().mockResolvedValue(reply({action_id:other,asset_ids:["source-image"]})));
 await expect(supplyCommand(approval)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
});
