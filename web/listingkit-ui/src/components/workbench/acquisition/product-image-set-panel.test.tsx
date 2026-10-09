import {StrictMode} from "react";
import {act,cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {ProductImageSetPanel} from "./product-image-set-panel";
const calls=vi.hoisted(()=>({context:{} as Record<string,unknown>}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>calls.context}));
const operation="11111111-1111-4111-8111-111111111111",runId="22222222-2222-4222-8222-222222222222",templateId="33333333-3333-4333-8333-333333333333";
const sha="a".repeat(64),digest="b".repeat(64),now="2026-10-09T00:00:00Z";
const template={templateId,agentId:"product.image.agent",lifecycle:"ACTIVE",revision:"1",version:"1",schemaVersion:"image-config-v1",name:"Two groups",targetPlatform:"product",createdAt:now,image:{schema:"image-config-v1",mode:"standard",shareOriginals:true,background:"white",language:"en",carousel:[{id:"main",purpose:"product_identity"}],detail:[{id:"detail",purpose:"detail_closeup"}]}};
const entry={agent:{agentId:"product.image.agent",activation:"ENABLED",revision:"1",activationEpoch:"1",defaultTemplate:{templateId,revision:"1"},updatedAt:now},name:"Images",description:"Full images",definitionVersion:"v1.0.0",parameterSchema:"image-config-v1",canConfigure:true,canUse:true,canReadRuns:true,capabilities:[{id:"image.generate",support:"REQUIRED",readiness:"AVAILABLE",reason:"",observedAt:now}]};
const source={ContextKind:"acquisition",ProductID:"p",OperationID:operation,OriginalPublicationID:"publication",OriginalVersion:1,EffectiveVersion:1};
function projection(status="awaiting_plan_approval",confirmationActionId=""){
 return {runId,status,confirmationActionId,generationAdmitted:!!confirmationActionId,planRevision:1,planDigest:sha,quoteDigest:digest,images:2,points:20,settledPoints:status==="awaiting_plan_approval"?0:20,resultDigest:status==="awaiting_plan_approval"?"":sha,approvalAvailable:status==="awaiting_final_approval",regenerationAvailable:status==="awaiting_final_approval",plan:{Source:source,Target:{Platform:"product",StoreID:"",Site:"",CategoryID:0}},slots:["main","detail"].map((slotId,i)=>({slotId,status:status==="awaiting_plan_approval"?"pending":"accepted",attempt:status==="awaiting_plan_approval"?0:1,errorCode:"",recipe:{Purpose:i?"detail_closeup":"product_identity",Background:"white",Language:"en",Placement:{Group:i?"detail":"carousel",Order:1},References:[{AssetID:"original"}],Quote:{Points:10}},candidates:status==="awaiting_plan_approval"?[]:[{assetId:`generated-${i}`,url:`https://images.test/generated-${i}.png`,width:1024,height:1024}],closure:status==="awaiting_plan_approval"?null:{Kind:"generated",Points:10}})),originals:[{ID:"original",DisplayURL:"https://images.test/original.png",Width:1024,Height:1024}],recoverableEffects:null,pendingCommand:null,block:null};
}
let state:ReturnType<typeof projection>,approved:boolean,fetch:ReturnType<typeof vi.fn<(input:unknown,init?:RequestInit)=>Promise<Response>>>;
beforeEach(()=>{
 localStorage.clear();approved=false;state=projection();
 calls.context={user:{id:"actor"},effectiveOrganization:{id:"org"},permissions:["imageagent.read","imageagent.write"],registerOrganizationSwitchGuard:vi.fn(()=>vi.fn())};
 fetch=vi.fn(async(input:unknown,init?:RequestInit)=>{
  const url=String(input),body=init?.body?JSON.parse(String(init.body)):undefined;
  if(url.includes("/agents/"))return Response.json(url.includes("/revisions/")?template:url.includes("/templates")?{items:[template],nextCursor:""}:entry);
  if(url.endsWith("/sources"))return Response.json({contextKind:"acquisition",contextId:operation,source,manualReplacementAvailable:false,originals:[{id:"original",displayUrl:"https://images.test/original.png",width:1024,height:1024}],evidence:{}});
  if(url.endsWith("/images/runs"))return Response.json({items:[],nextCursor:""});
  if(url.endsWith("/prepare"))return Response.json(state,{status:201});
  if(url.includes("/by-key/"))return Response.json(state);
  if(url.endsWith("/confirm")){state=projection("awaiting_final_approval",body.actionId);return Response.json({runId,status:"accepted"},{status:202})}
  if(url.endsWith("/preview"))return Response.json({digest,head:{action_id:"",payload_hash:""},assets:[{id:"generated-0",role:"main",url:"https://images.test/generated-0.png"},{id:"generated-1",role:"detail",url:"https://images.test/generated-1.png"}]});
  if(url.endsWith("/approve")){approved=true;state={...state,status:"completed",approvalAvailable:false};return Response.json({runId,status:"accepted"},{status:202})}
  if(url.endsWith("/inventory"))return Response.json({target:{head:approved?{action_id:templateId,payload_hash:sha}:{action_id:"",payload_hash:""},assets:approved?[{id:"generated-0",role:"main",url:"https://images.test/generated-0.png",presentation:{group:"carousel",order:1}},{id:"generated-1",role:"detail",url:"https://images.test/generated-1.png",presentation:{group:"detail",order:1}}]:[]},generic:null});
  if(url.endsWith(`/runs/${runId}`))return Response.json(state);
  throw new Error(`unexpected ${url}`);
 });vi.stubGlobal("fetch",fetch);
});
afterEach(()=>{cleanup();vi.unstubAllGlobals();localStorage.clear()});
async function prepare(){
 fireEvent.click(await screen.findByRole("checkbox",{name:"共用原始素材 素材 1"}));
 const button=screen.getByRole("button",{name:"准备整套图片计划（2 项）"});await waitFor(()=>expect(button).toBeEnabled());fireEvent.click(button);
}
it("runs both groups from shared originals and saves only a previewed explicit complete selection",async()=>{
 const saved=vi.fn();render(<StrictMode><ProductImageSetPanel kind="acquisition" contextId={operation} onSaved={saved}/></StrictMode>);
 await prepare();fireEvent.click(await screen.findByRole("button",{name:"确认点数并生成"}));
 const choices=await screen.findAllByRole("button",{name:"选择采用"});expect(choices).toHaveLength(2);choices.forEach(button=>fireEvent.click(button));
 fireEvent.click(screen.getByRole("button",{name:"预览完整选择"}));fireEvent.click(await screen.findByRole("button",{name:"人工批准并保存素材"}));
 await screen.findByText("本次批准已保存为正式商品素材。");expect(saved).toHaveBeenCalledOnce();
 const preparation=fetch.mock.calls.find(([url])=>String(url).endsWith("/prepare"))!;
 const request=JSON.parse(String(preparation[1]!.body));expect(request.sharedOriginalIds).toEqual(["original"]);expect(request.selectedTaskIds).toEqual(["main","detail"]);
 const approval=fetch.mock.calls.find(([url])=>String(url).endsWith("/approve"))!;
 const selected=JSON.parse(String(approval[1]!.body));expect(selected.selectionDigest).toBe(digest);expect(selected.choices.map((v:{presentation:unknown})=>v.presentation)).toEqual([{group:"carousel",order:1},{group:"detail",order:1}]);
 expect(selected.choices.every((v:Record<string,unknown>)=>!("url" in v))).toBe(true);
});
it("retains an uncertain preparation across reload and verifies its original key with GET only",async()=>{
 const real=fetch.getMockImplementation()!;fetch.mockImplementation(async(url,init)=>{if(String(url).endsWith("/prepare"))throw new Error("lost acknowledgement");return real(url,init)});
 const view=render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);await prepare();await screen.findByRole("button",{name:"核实原请求"});
 const first=fetch.mock.calls.find(([url])=>String(url).endsWith("/prepare"))!;const key=new Headers(first[1]!.headers).get("Idempotency-Key");
 view.unmount();render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));await screen.findByRole("button",{name:"确认点数并生成"});
 expect(fetch.mock.calls.filter(([url])=>String(url).endsWith("/prepare"))).toHaveLength(1);
 expect(fetch.mock.calls.find(([url])=>String(url).endsWith(`/by-key/${key}`))?.[1]?.method).toBe("GET");
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull();
});
it("does not treat another confirmation's status as proof of the frozen original command",async()=>{
 state=projection("executing",templateId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"confirm",runId,body:{actionId:operation,planRevision:1,planDigest:sha,quoteDigest:digest}}));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).endsWith(`/runs/${runId}`))).toBe(true));
 expect(screen.getByRole("button",{name:"核实原请求"})).toBeInTheDocument();expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

it("discards a late preparation response after the user changes enterprise",async()=>{
 let resolve!:(response:Response)=>void;
 const late=new Promise<Response>(ok=>{resolve=ok}),real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith("/prepare")?late:real(url,init));
 const view=render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);await prepare();
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).endsWith("/prepare"))).toBe(true));
 const first=fetch.mock.calls.find(([url])=>String(url).endsWith("/prepare"))!;
 calls.context={...calls.context,effectiveOrganization:{id:"other"}};view.rerender(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 expect(first[1]!.signal?.aborted).toBe(true);await act(async()=>resolve(Response.json(state,{status:201})));
 expect(screen.queryByRole("button",{name:"确认点数并生成"})).not.toBeInTheDocument();
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
 expect(localStorage.getItem(`product-image-set:actor:other:acquisition:${operation}:run`)).toBeNull();
});

it("keeps an admitted result reviewable after enterprise activation is disabled",async()=>{
 state=projection("awaiting_final_approval",templateId);localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 const real=fetch.getMockImplementation()!;fetch.mockImplementation((url,init)=>String(url).endsWith("/agents/product.image.agent")?Promise.resolve(Response.json({...entry,agent:{...entry.agent,activation:"DISABLED"}})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 expect(await screen.findAllByRole("button",{name:"选择采用"})).toHaveLength(2);
 expect(screen.queryByRole("button",{name:"准备整套图片计划（2 项）"})).not.toBeInTheDocument();
});
