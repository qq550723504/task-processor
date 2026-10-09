import {StrictMode} from "react";
import {act,cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {ProductImageSetPanel} from "./product-image-set-panel";
import {imageTemplateSchema} from "@/lib/contracts/image-set-configuration";
const calls=vi.hoisted(()=>({context:{} as Record<string,unknown>}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>calls.context}));
const operation="11111111-1111-4111-8111-111111111111",runId="22222222-2222-4222-8222-222222222222",templateId="33333333-3333-4333-8333-333333333333";
const sha="a".repeat(64),digest="b".repeat(64),now="2026-10-09T00:00:00Z";
const template={templateId,agentId:"product.image.agent",lifecycle:"ACTIVE",revision:"1",version:"1",schemaVersion:"image-config-v1",name:"Two groups",targetPlatform:"product",createdAt:now,image:{schema:"image-config-v1",mode:"standard",shareOriginals:true,background:"white",language:"en",carousel:[{id:"main",purpose:"product_identity"}],detail:[{id:"detail",purpose:"detail_closeup"}]}};
const entry={agent:{agentId:"product.image.agent",activation:"ENABLED",revision:"1",activationEpoch:"1",defaultTemplate:{templateId,revision:"1"},updatedAt:now},name:"Images",description:"Full images",definitionVersion:"v1.0.0",parameterSchema:"image-config-v1",canConfigure:true,canUse:true,canReadRuns:true,capabilities:[{id:"image.generate",support:"REQUIRED",readiness:"AVAILABLE",reason:"",observedAt:now}]};
const source={ContextKind:"acquisition",ProductID:"p",OperationID:operation,OriginalPublicationID:"publication",OriginalVersion:1,EffectiveVersion:1};
function projection(status="awaiting_plan_approval",confirmationActionId=""){
 return {runId,status,confirmationActionId,generationAdmitted:!!confirmationActionId,planRevision:1,planDigest:sha,quoteDigest:digest,images:2,points:20,settledPoints:status==="awaiting_plan_approval"?0:20,resultDigest:status==="awaiting_plan_approval"?"":sha,approvalAvailable:status==="awaiting_final_approval",candidateSelectionAvailable:["awaiting_final_approval","completed","failed","blocked","cancelled"].includes(status),regenerationAvailable:status==="awaiting_final_approval",plan:{Source:source,Target:{Platform:"product",StoreID:"",Site:"",CategoryID:0}},slots:["main","detail"].map((slotId,i)=>({slotId,status:status==="awaiting_plan_approval"?"pending":"accepted",attempt:status==="awaiting_plan_approval"?0:1,errorCode:"",recipe:{Purpose:i?"detail_closeup":"product_identity",Background:"white",Language:"en",Placement:{Group:i?"detail":"carousel",Order:1},References:[{AssetID:"original"}],Quote:{Points:10}},candidates:status==="awaiting_plan_approval"?[]:[{assetId:`generated-${i}`,url:`https://images.test/generated-${i}.png`,width:1024,height:1024}],closure:status==="awaiting_plan_approval"?null:{Kind:"generated",Points:10}})),originals:[{ID:"original",DisplayURL:"https://images.test/original.png",Width:1024,Height:1024}],recoverableEffects:null,pendingCommand:null,block:null};
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

for (const status of ["awaiting_plan_approval","executing"]) {
 it(`retains the original confirmation after GET reports ${status} and can resume the exact request`,async()=>{
  const body={actionId:operation,planRevision:1,planDigest:sha,quoteDigest:digest};
  state=projection(status,status==="executing"?operation:"");
  localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"confirm",runId,body}));
  render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
  fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
  await waitFor(()=>expect(screen.getByRole("button",{name:"核实原请求"})).toBeEnabled());
  expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
  fireEvent.click(screen.getByRole("button",{name:"继续原确认"}));
  await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
  const requests=fetch.mock.calls.filter(([url])=>String(url).endsWith("/confirm"));
  expect(requests).toHaveLength(1);expect(JSON.parse(String(requests[0][1]!.body))).toEqual(body);
 });
}
it("prepares the displayed immutable default content version even after the template head advances",async()=>{
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).includes("/revisions/")?Promise.resolve(Response.json({...template,revision:"2",version:"1"})):String(url).includes("/templates?")?Promise.resolve(Response.json({items:[{...template,revision:"2",version:"2"}],nextCursor:""})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);await prepare();
 await screen.findByRole("button",{name:"确认点数并生成"});
 const select=screen.getByLabelText("图片模板") as HTMLSelectElement;
 expect(select.selectedOptions[0].textContent).toBe("Two groups v1");
 await screen.findByRole("button",{name:"确认点数并生成"});
 const request=fetch.mock.calls.find(([url])=>String(url).endsWith("/prepare"))!;
 expect(JSON.parse(String(request[1]!.body)).template).toEqual({templateId,revision:"1"});
});
for (const parentStatus of ["awaiting_final_approval","completed","failed","blocked","cancelled"]) {
it(`can combine new subset output with successful ${parentStatus} parent outputs without another generation`,async()=>{
 const parentId="44444444-4444-4444-8444-444444444444";
 const parent={...projection(parentStatus,templateId),runId:parentId};
 const child={...projection("awaiting_final_approval",operation),images:1,slots:projection("awaiting_final_approval").slots.slice(0,1),plan:{...state.plan,Regeneration:{RunID:parentId}}};
 state=child;localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith(`/runs/${parentId}`)?Promise.resolve(Response.json(parent)):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click((await screen.findAllByRole("button",{name:"选择采用"}))[0]);
 const original=await screen.findAllByRole("button",{name:"采用原任务结果"});expect(original).toHaveLength(2);expect(original[1]).toBeEnabled();fireEvent.click(original[1]);
 fireEvent.click(screen.getByRole("button",{name:"预览完整选择"}));
 await screen.findByRole("button",{name:"人工批准并保存素材"});
 const request=fetch.mock.calls.find(([url])=>String(url).endsWith("/preview"))!;
 const choices=JSON.parse(String(request[1]!.body)).choices;
 expect(choices.map((v:{run_id:string,slot_id:string})=>[v.run_id,v.slot_id])).toEqual([[runId,"main"],[parentId,"detail"]]);
 expect(fetch.mock.calls.some(([url])=>/\/(prepare|regenerate|confirm)$/.test(String(url)))).toBe(false);
});


}
it("loads both legal near-limit templates through bounded pages",async()=>{
 const image={schema:"image-config-v1",mode:"custom",shareOriginals:true,background:"",language:"en",carousel:Array.from({length:32},(_,i)=>({id:`task-${i}`,purpose:"custom",brief:"x".repeat(1990)})),detail:[]};
 image.background="x".repeat(65520-new TextEncoder().encode(JSON.stringify(image)).length);
 const rows=["Large first","Large second"].map((name,i)=>imageTemplateSchema.parse({...template,templateId:`45a227bb-b572-4138-8e2a-5f1a0be9861${i}`,name:`${name} ${"😀".repeat(100)}`,image}));
 expect(new TextEncoder().encode(JSON.stringify({items:rows,nextCursor:""})).length).toBeGreaterThan(128*1024);
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation(async(input,init)=>{
  const url=new URL(String(input),"https://app.test");if(!url.pathname.endsWith("/templates"))return real(input,init);
  const page=url.searchParams.get("pageSize")==="1"?{items:[rows[url.searchParams.has("cursor")?1:0]],nextCursor:url.searchParams.has("cursor")?"":"next-page"}:{items:rows,nextCursor:""};
  return new TextEncoder().encode(JSON.stringify(page)).length>128*1024?Response.json({code:"DEPENDENCY_UNAVAILABLE"},{status:503}):Response.json(page);
 });
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 await screen.findByRole("option",{name:/Large first/});
 fireEvent.click(screen.getByRole("button",{name:"读取更多模板"}));
 await screen.findByRole("option",{name:/Large second/});
 fireEvent.change(screen.getByRole("combobox",{name:"图片模板"}),{target:{value:`${rows[1]!.templateId}:1`}});
 fireEvent.click(screen.getByRole("checkbox",{name:"共用原始素材 素材 1"}));
 expect(screen.getByRole("button",{name:"准备整套图片计划（32 项）"})).toBeEnabled();
 expect(fetch.mock.calls.filter(([url])=>String(url).includes("/templates?")).map(([url])=>new URL(String(url),"https://app.test").searchParams.get("pageSize"))).toEqual(["1","1"]);
});
for(const failure of ["templates","recent"]){
 it(`restores the saved run before optional ${failure} discovery fails`,async()=>{
  state=projection("awaiting_final_approval",templateId);
  localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
  const real=fetch.getMockImplementation()!;
  fetch.mockImplementation(async(url,init)=>String(url).includes(failure==="templates"?"/templates?":"/images/runs")&&!String(url).includes(`/runs/${runId}`)?Response.json({code:"DEPENDENCY_UNAVAILABLE"},{status:503}):real(url,init));
  render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
  const choices=await screen.findAllByRole("button",{name:"选择采用"});
  await screen.findByRole("alert");
  expect(choices).toHaveLength(2);choices.forEach(button=>expect(button).toBeEnabled());
  expect(screen.getByRole("button",{name:"选择原图 1"})).toBeEnabled();
  expect(fetch.mock.calls.filter(([,init])=>init?.method==="POST")).toHaveLength(0);
 });
}
for(const failure of ["read","inventory"]){
 it(`keeps current template preparation available when original run ${failure} fails`,async()=>{
  state=projection("awaiting_final_approval",templateId);
  const key=`product-image-set:actor:org:acquisition:${operation}:run`;localStorage.setItem(key,runId);
  const real=fetch.getMockImplementation()!;
  fetch.mockImplementation(async(url,init)=>String(url).endsWith(failure==="read"?`/runs/${runId}`:"/inventory")?Response.json({code:failure==="read"?"IMAGE_NOT_FOUND":"IMAGE_UNAVAILABLE"},{status:failure==="read"?404:503}):real(url,init));
  render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
  await screen.findByRole("alert");
  fireEvent.click(await screen.findByRole("checkbox",{name:"共用原始素材 素材 1"}));
  expect(screen.getByRole("button",{name:"准备整套图片计划（2 项）"})).toBeEnabled();
  expect(localStorage.getItem(key)).toBe(runId);
  expect(fetch.mock.calls.filter(([,init])=>init?.method==="POST")).toHaveLength(0);
 });
}
const sheinTarget={Platform:"shein",StoreID:"saved-store",Site:"US",CategoryID:123,RecordID:"saved-record"} as const;
const officialPlacement={Group:"spu",SKC:0,SKU:0,Type:1,Sort:1,Site:"US"};
const sheinRequirements={platform:"shein",site:"US",categoryId:123,version:"current-official-rules",nativeWidth:1024,nativeHeight:1024,groups:[{group:"spu",skc:0,sku:0,types:[{type:1,minimum:1,maximum:8,nativeCompatible:true}]}]};
function sheinProjection(){const p=projection("awaiting_final_approval",templateId);return {...p,plan:{...p.plan,Source:{...source,EffectiveVersion:2,ApplyReceiptID:operation},Target:sheinTarget},slots:p.slots.map(slot=>({...slot,recipe:{...slot.recipe,OfficialPlacement:officialPlacement}}))}}
for(const restoration of ["initial","recent"]){
it(`restores a SHEIN ${restoration} run's saved target and current rules for original selection and subset regeneration`,async()=>{
 const p=sheinProjection(),real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith("/requirements")?Promise.resolve(Response.json(sheinRequirements)):String(url).endsWith(`/runs/${runId}`)?Promise.resolve(Response.json(p)):String(url).endsWith("/images/runs")?Promise.resolve(Response.json({items:[{runId,contextKind:"acquisition",contextId:operation,status:p.status,targetPlatform:"shein",createdAt:now}],nextCursor:""})):String(url).endsWith("/regenerate")?Promise.resolve(Response.json(state,{status:201})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation} {...restoration==="initial"?{initialRunId:runId}:{target:{...sheinTarget,StoreID:"unrelated-store",RecordID:"unrelated-record",CategoryID:456}}}/>);
 if(restoration==="recent")fireEvent.change(await screen.findByLabelText("本商品最近任务"),{target:{value:runId}});
 await screen.findByText(/规则 current-official-rules/);
 expect(screen.getByLabelText("素材目标")).toHaveValue("shein");
 expect(screen.getByText("店铺 saved-store · 站点 US · 类目 123")).toBeInTheDocument();
 const rules=fetch.mock.calls.find(([url])=>String(url).endsWith("/requirements"))!;
 expect(JSON.parse(String(rules[1]!.body))).toEqual({target:sheinTarget,effectiveCatalogVersion:2,applyReceiptId:operation});
 fireEvent.click(screen.getByRole("button",{name:"选择原图 1"}));
 const position=screen.getAllByLabelText("官方图片位置").at(-1)!;expect(position).toBeEnabled();
 fireEvent.change(position,{target:{value:"spu:0:0"}});fireEvent.click(screen.getByRole("button",{name:"预览完整选择"}));
 await screen.findByRole("button",{name:"人工批准并保存素材"});
 const preview=fetch.mock.calls.find(([url])=>String(url).endsWith("/preview"))!;
 expect(JSON.parse(String(preview[1]!.body)).choices[0].official_placement).toEqual({group:"spu",skc:0,sku:0,type:1,sort:1,site:"US"});
 fireEvent.click(screen.getByRole("checkbox",{name:"共用原始素材 素材 1"}));
 fireEvent.click(screen.getAllByRole("checkbox",{name:"重新生成此项，另行确认点数"})[0]);
 fireEvent.click(screen.getByRole("button",{name:"准备所选 1 项的新计划"}));
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).endsWith("/regenerate"))).toBe(true));
 const regeneration=fetch.mock.calls.find(([url])=>String(url).endsWith("/regenerate"))!;
 expect(JSON.parse(String(regeneration[1]!.body))).toMatchObject({target:sheinTarget,effectiveCatalogVersion:2,applyReceiptId:operation,selectedTaskIds:["main"],officialPlacements:{main:officialPlacement}});
});
}
it("does not expose stale rules when switching SHEIN runs and can reload the saved target after a rules failure",async()=>{
 const first=sheinProjection(),secondId="44444444-4444-4444-8444-444444444444",second={...first,runId:secondId,plan:{...first.plan,Target:{...sheinTarget,StoreID:"second-store",RecordID:"second-record",CategoryID:456}}};
 let unavailable=true;const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith("/requirements")?Promise.resolve(JSON.parse(String(init!.body)).target.RecordID==="second-record"&&unavailable?Response.json({code:"IMAGE_UNAVAILABLE"},{status:503}):Response.json({...sheinRequirements,categoryId:JSON.parse(String(init!.body)).target.CategoryID})):String(url).endsWith(`/runs/${runId}`)?Promise.resolve(Response.json(first)):String(url).endsWith(`/runs/${secondId}`)?Promise.resolve(Response.json(second)):String(url).endsWith("/images/runs")?Promise.resolve(Response.json({items:[first,second].map(p=>({runId:p.runId,contextKind:"acquisition",contextId:operation,status:p.status,targetPlatform:"shein",createdAt:now})),nextCursor:""})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation} initialRunId={runId}/>);
 await screen.findByText(/规则 current-official-rules/);
 fireEvent.change(screen.getByLabelText("本商品最近任务"),{target:{value:secondId}});await screen.findByRole("alert");
 expect(screen.queryByText(/规则 current-official-rules/)).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"选择原图 1"}));expect(screen.getAllByLabelText("官方图片位置").at(-1)).toBeDisabled();
 unavailable=false;fireEvent.click(screen.getByRole("button",{name:"读取当前图片规则"}));
 await waitFor(()=>expect(screen.getAllByLabelText("官方图片位置").at(-1)).toBeEnabled());
 const last=fetch.mock.calls.filter(([url])=>String(url).endsWith("/requirements")).at(-1)!;
 expect(JSON.parse(String(last[1]!.body))).toEqual({target:second.plan.Target,effectiveCatalogVersion:2,applyReceiptId:operation});
});
it("discards late SHEIN rules from a previously refreshed run",async()=>{
 const first=sheinProjection(),secondId="44444444-4444-4444-8444-444444444444",second={...first,runId:secondId,plan:{...first.plan,Target:{...sheinTarget,StoreID:"second-store",RecordID:"second-record",CategoryID:456}}};
 let resolve!:(response:Response)=>void,firstReads=0;const late=new Promise<Response>(ok=>{resolve=ok}),real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>{
  if(String(url).endsWith("/requirements")){const target=JSON.parse(String(init!.body)).target;if(target.RecordID==="saved-record"&&++firstReads===2)return late;return Promise.resolve(Response.json({...sheinRequirements,version:target.RecordID,categoryId:target.CategoryID}))}
  if(String(url).endsWith(`/runs/${runId}`))return Promise.resolve(Response.json(first));
  if(String(url).endsWith(`/runs/${secondId}`))return Promise.resolve(Response.json(second));
  if(String(url).endsWith("/images/runs"))return Promise.resolve(Response.json({items:[first,second].map(p=>({runId:p.runId,contextKind:"acquisition",contextId:operation,status:p.status,targetPlatform:"shein",createdAt:now})),nextCursor:""}));
  return real(url,init);
 });
 render(<ProductImageSetPanel kind="acquisition" contextId={operation} initialRunId={runId}/>);
 await screen.findByText(/规则 saved-record/);fireEvent.click(screen.getByRole("button",{name:"刷新原任务"}));
 await waitFor(()=>expect(firstReads).toBe(2));fireEvent.change(screen.getByLabelText("本商品最近任务"),{target:{value:secondId}});
 await screen.findByText(/规则 second-record/);await act(async()=>resolve(Response.json({...sheinRequirements,version:"late-first-record"})));
 expect(screen.queryByText(/规则 late-first-record/)).not.toBeInTheDocument();expect(screen.getByText("店铺 second-store · 站点 US · 类目 456")).toBeInTheDocument();
});
it("keeps an UNKNOWN parent's successful-looking candidates unavailable for reuse",async()=>{
 const parentId="44444444-4444-4444-8444-444444444444";
 const parent={...projection("blocked",templateId),runId:parentId,candidateSelectionAvailable:false,resultDigest:"",recoverableEffects:[{SlotID:"detail",Attempt:1,Code:"provider_outcome_unknown"}]};
 const child={...projection("awaiting_final_approval",operation),plan:{...state.plan,Regeneration:{RunID:parentId}}};state=child;
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith(`/runs/${parentId}`)?Promise.resolve(Response.json(parent)):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 const choices=await screen.findAllByRole("button",{name:"采用原任务结果"});
 choices.forEach(button=>expect(button).toBeDisabled());
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

for (const action of ["prepare","regenerate"]) {
it(`can replay an unresolved ${action} with its original key and frozen body after not-found`,async()=>{
 const requestKey=templateId,body={target:{Platform:"product"},sharedOriginalIds:["original"],selectedTaskIds:["main","detail"],template:{templateId,revision:"1"}};
 const intent={action,...action==="regenerate"?{runId:operation}:{},requestKey,body};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify(intent));
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).includes("/by-key/")?Promise.resolve(Response.json({code:"IMAGE_NOT_FOUND"},{status:404})):String(url).endsWith(`/${action}`)?Promise.resolve(Response.json(state,{status:201})):real(url,init));
 const view=render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));await screen.findByRole("alert");
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
 view.unmount();render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"继续原操作"}));
 await screen.findByRole("button",{name:"确认点数并生成"});
 const requests=fetch.mock.calls.filter(([url])=>String(url).endsWith(`/${action}`));expect(requests).toHaveLength(1);
 expect(new Headers(requests[0][1]!.headers).get("Idempotency-Key")).toBe(requestKey);
 expect(JSON.parse(String(requests[0][1]!.body))).toEqual(body);
 expect(fetch.mock.calls.some(([url])=>String(url).endsWith("/confirm"))).toBe(false);
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull();
});
}

it("clears lost recovery intent only when its exact original effect has a known closed result",async()=>{
 state=projection("awaiting_final_approval",templateId);state.slots[0].closure={Kind:"settled",Points:10};
 const body={actionId:operation,planRevision:1,slotId:"main",attempt:1};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"recover",runId,body}));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
 expect(screen.getAllByRole("button",{name:"选择采用"})[0]).toBeEnabled();
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

for(const mismatch of ["unknown","attempt","revision","closure"]){
it(`keeps a recovery intent with ${mismatch} unresolved and replays its exact action`,async()=>{
 const body={actionId:operation,planRevision:1,slotId:"main",attempt:1};
 state=projection("blocked",templateId);state.slots[0].closure={Kind:"settled",Points:10};
 const raw={...state,recoverableEffects:mismatch==="unknown"?[{SlotID:"main",Attempt:1,Code:"provider_outcome_unknown"}]:null};
 if(mismatch==="attempt")raw.slots[0].attempt=2;
 if(mismatch==="revision")raw.planRevision=2;
 if(mismatch==="closure")raw.slots[0].closure=null;
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"recover",runId,body}));
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith(`/runs/${runId}`)?Promise.resolve(Response.json(raw)):String(url).endsWith("/recover")?Promise.resolve(Response.json({code:"IMAGE_BLOCKED"},{status:409})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(screen.getByRole("button",{name:"核实原请求"})).toBeEnabled());
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
 fireEvent.click(screen.getByRole("button",{name:"继续原操作"}));await screen.findByRole("alert");
 const requests=fetch.mock.calls.filter(([url])=>String(url).endsWith("/recover"));expect(requests).toHaveLength(1);
 expect(JSON.parse(String(requests[0][1]!.body))).toEqual(body);
 expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
});
}

for(const receiptFound of [true,false]){
it(`notifies a saved SHEIN result and verifies its original resume receipt despite unavailable current rules (receipt=${receiptFound})`,async()=>{
 const p={...sheinProjection(),status:"completed",approvalAvailable:false,regenerationAvailable:false},saved=vi.fn(),real=fetch.getMockImplementation()!;
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"resume",runId,body:{actionId:operation}}));
 fetch.mockImplementation((url,init)=>String(url).endsWith("/requirements")?Promise.resolve(Response.json({code:"IMAGE_UNAVAILABLE"},{status:503})):String(url).endsWith(`/runs/${runId}`)?Promise.resolve(Response.json(p)):String(url).endsWith(`/approvals/${operation}`)?Promise.resolve(receiptFound?Response.json({actionId:operation,selectionDigest:digest,assets:[{id:"image",role:"main",url:"https://images.test/result.png"}]}):Response.json({code:"IMAGE_NOT_FOUND"},{status:404})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation} onSaved={saved}/>);
 await waitFor(()=>expect(saved).toHaveBeenCalledOnce());fireEvent.click(screen.getByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).endsWith(`/approvals/${operation}`))).toBe(true));
 if(receiptFound)await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
 else expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull();
 expect(saved).toHaveBeenCalledOnce();expect(screen.queryByText(/规则 current-official-rules/)).not.toBeInTheDocument();
 expect(fetch.mock.calls.some(([url])=>/\/(prepare|regenerate|confirm|resume|approve)$/.test(String(url)))).toBe(false);
});
}
for (const receiptFound of [true,false]) {
it(`verifies a lost resume response using its original immutable approval receipt (found=${receiptFound})`,async()=>{
 state=projection("completed",templateId);
 const body={actionId:operation};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"resume",runId,body}));
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith(`/approvals/${operation}`)?Promise.resolve(receiptFound?Response.json({actionId:operation,selectionDigest:digest,assets:[{id:"image",role:"main",url:"https://images.test/result.png"}]}):Response.json({code:"IMAGE_NOT_FOUND"},{status:404})):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 if(receiptFound)await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
 else {await screen.findByRole("alert");expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).not.toBeNull()}
 expect(fetch.mock.calls.some(([url])=>String(url).endsWith(`/approvals/${operation}`))).toBe(true);
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});
}

it("clears a lost original confirmation after exact admitted terminal evidence",async()=>{
 state=projection("completed",operation);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"confirm",runId,body:{actionId:operation,planRevision:1,planDigest:sha,quoteDigest:digest}}));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

it("unlocks original effect recovery once the exact confirmation produced durable blocked effects",async()=>{
 const blocked={...projection("blocked",operation),recoverableEffects:[{SlotID:"main",Attempt:1,Code:"provider_outcome_unknown"}]};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:intent`,JSON.stringify({action:"confirm",runId,body:{actionId:operation,planRevision:1,planDigest:sha,quoteDigest:digest}}));
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation((url,init)=>String(url).endsWith(`/runs/${runId}`)?Promise.resolve(Response.json(blocked)):real(url,init));
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(localStorage.getItem(`product-image-set:actor:org:acquisition:${operation}:intent`)).toBeNull());
 expect(screen.getByRole("button",{name:"核实原调用"})).toBeEnabled();
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

it("restores an unfinished failed run through its original workflow without a new preparation or confirmation",async()=>{
 state=projection("failed",operation);state.slots[1]={...state.slots[1],status:"pending",attempt:0,candidates:[],closure:null};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 const real=fetch.getMockImplementation()!;
 fetch.mockImplementation(async(url,init)=>{
  if(String(url).endsWith("/restart")){state=projection("awaiting_final_approval",operation);return Response.json({runId,status:"accepted"},{status:202})}
  return real(url,init);
 });
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"恢复原失败任务"}));
 await waitFor(()=>expect(fetch.mock.calls.some(([url])=>String(url).endsWith("/restart"))).toBe(true));
 await screen.findByText("等待人工选择");
 const requests=fetch.mock.calls.filter(([,init])=>init?.method==="POST");
 expect(requests).toHaveLength(1);expect(String(requests[0][0])).toContain(`/runs/${runId}/restart`);
 expect(JSON.parse(String(requests[0][1]!.body))).toEqual({planRevision:1,planDigest:sha,quoteDigest:digest});
});

it("keeps a lost restart acknowledgement across reload and explicitly replays only its original plan",async()=>{
 state=projection("failed",operation);
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 const real=fetch.getMockImplementation()!;let attempts=0;
 fetch.mockImplementation(async(url,init)=>{
  if(String(url).endsWith("/restart")){
   if(++attempts===1)throw new Error("lost acknowledgement");
   state=projection("blocked",operation);return Response.json({runId,status:"accepted"},{status:202});
  }
  return real(url,init);
 });
 const view=render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"恢复原失败任务"}));
 await screen.findByRole("button",{name:"核实原请求"});
 const key=`product-image-set:actor:org:acquisition:${operation}:intent`,frozen=localStorage.getItem(key);
 expect(JSON.parse(frozen!)).toEqual({action:"restart",runId,body:{planRevision:1,planDigest:sha,quoteDigest:digest}});
 view.unmount();render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await screen.findByText("原操作尚未得到明确回执，请继续核实同一编号。");
 expect(attempts).toBe(1);expect(localStorage.getItem(key)).toBe(frozen);
 fireEvent.click(screen.getByRole("button",{name:"继续原操作"}));
 await waitFor(()=>expect(localStorage.getItem(key)).toBeNull());
 const posts=fetch.mock.calls.filter(([,init])=>init?.method==="POST");
 expect(posts).toHaveLength(2);expect(posts[1][0]).toBe(posts[0][0]);expect(posts[1][1]!.body).toBe(posts[0][1]!.body);
});

it("requires the original plan before read-only progress can verify a failed-run restart",async()=>{
 const key=`product-image-set:actor:org:acquisition:${operation}:intent`;
 localStorage.setItem(key,JSON.stringify({action:"restart",runId,body:{planRevision:1,planDigest:sha,quoteDigest:digest}}));
 state={...projection("executing",operation),planDigest:"c".repeat(64)};
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 fireEvent.click(await screen.findByRole("button",{name:"核实原请求"}));
 await screen.findByText("原操作尚未得到明确回执，请继续核实同一编号。");
 expect(localStorage.getItem(key)).not.toBeNull();
 state=projection("executing",operation);
 fireEvent.click(screen.getByRole("button",{name:"核实原请求"}));
 await waitFor(()=>expect(localStorage.getItem(key)).toBeNull());
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

it("keeps a known closed failed run on the result-review and regeneration path",async()=>{
 state={...projection("failed",operation),regenerationAvailable:true};
 localStorage.setItem(`product-image-set:actor:org:acquisition:${operation}:run`,runId);
 render(<ProductImageSetPanel kind="acquisition" contextId={operation}/>);
 await screen.findByRole("button",{name:"准备所选 0 项的新计划"});
 expect(screen.queryByRole("button",{name:"恢复原失败任务"})).not.toBeInTheDocument();
 expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});
