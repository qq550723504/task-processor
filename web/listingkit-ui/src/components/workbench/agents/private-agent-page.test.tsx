import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AgentPage } from "./agent-page";
import { PrivateAgentPage } from "./private-agent-page";
const permissions=["workbench.agent.read","workbench.agent.use","workbench.collection.read","workbench.supply.read","workbench.store.read"];
const state=vi.hoisted(()=>({permissions:[] as string[]}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({user:{id:"actor"},effectiveOrganization:{id:"org",name:"测试企业"},permissions:state.permissions,registerOrganizationSwitchGuard:()=>()=>{}})}));
const id="b510c346-54e7-4cbd-91b2-05f7c0149b42",recordId="11111111-1111-4111-8111-111111111111";
const at="2026-10-09T00:00:00Z",hash="a".repeat(64);
const delivery={id,requestId:id,organizationId:"org",definition:"product.quality.check",version:"2.0.0",name:"平台草稿资料质检",createdBy:"staff",createdAt:at};
const draft={recordId,revision:3,sourceId:id,preparationId:id,storeId:id,platform:"shein",site:"shein-us",productKey:"own:fixture",productVersion:"1",title:"盒子",recordHash:hash,productHash:hash,rulesHash:hash,inventoryHash:hash,savedAt:at,readyForUpload:false,issues:[{code:"missing",field:"category_id",message:"选择当前店铺可发布的末级类目"}]};
const run={id,deliveryId:id,organizationId:"org",actorId:"actor",key:id,definition:"product.quality.check",version:"2.0.0",draft,createdAt:at};
const summary={id,deliveryId:id,organizationId:"org",actorId:"actor",version:"2.0.0",title:"盒子",findingCount:1,createdAt:at,draft:Object.fromEntries(Object.entries(draft).filter(([key])=>!["issues","readyForUpload"].includes(key)))};
vi.mock("@/lib/api/supply-chain",()=>({listSupplyPreparations:vi.fn(async()=>({items:[{id,name:"测试批次"}]})),listSupplyStages:vi.fn(async()=>({items:[{sourceId:id,stage:"missing"}]})),readSupplyTarget:vi.fn(async()=>({id:recordId,revision:3,createdAt:at,source:{source:{productKey:"own:fixture"}},result:{product:{multi_language_name_list:[{name:"盒子"}]}}}))}));
vi.mock("@/lib/api/workbench-stores",()=>({listWorkbenchStores:vi.fn(async()=>({items:[{id,name:"测试店铺"}],pagination:{total:1}}))}));
beforeEach(()=>{sessionStorage.clear();vi.restoreAllMocks();state.permissions=[...permissions]});
afterEach(()=>{cleanup();vi.restoreAllMocks()});
function reads(url:unknown){return Response.json(String(url).endsWith("/reports")?{items:[],nextCursor:""}:String(url).includes("/reports/")?run:delivery)}

it("labels offline trial reports after a server read and hides unavailable editing links",async()=>{
  vi.stubGlobal("fetch",vi.fn(async(url:unknown)=>Response.json(String(url).endsWith("/reports")?{items:[{...summary,draft:{...summary.draft,offlineTrial:true}}],nextCursor:""}:String(url).includes("/reports/")?{...run,draft:{...draft,offlineTrial:true}}:delivery)));
  render(<PrivateAgentPage id={id} offlineTrial />);
  expect(await screen.findByText(/这是隔离离线试用/)).toBeInTheDocument();
  await screen.findByRole("button",{name:"查看报告"});
  fireEvent.click(screen.getByRole("button",{name:"查看报告"}));
  expect(await screen.findByText("规则来源：离线测试规则，未获取 SHEIN 官方规则。",{exact:true})).toBeInTheDocument();
  expect(screen.queryByRole("link",{name:/补全/})).not.toBeInTheDocument();
});
async function choose(){await screen.findByRole("option",{name:"测试批次"});await screen.findByRole("option",{name:"测试店铺"});fireEvent.change(screen.getByRole("combobox",{name:"供应链批次"}),{target:{value:id}});fireEvent.change(screen.getByRole("combobox",{name:"目标店铺"}),{target:{value:id}});await screen.findByRole("option",{name:"盒子 · 草稿 v3"});fireEvent.change(screen.getByRole("combobox",{name:"平台草稿"}),{target:{value:recordId}})}
it("keeps private draft delivery visible when title configuration is unavailable",async()=>{
  vi.spyOn(globalThis,"fetch").mockImplementation(async url=>String(url).includes("agent-customization/agents")?Response.json({items:[delivery],nextCursor:""}):Response.json({code:"DEPENDENCY_UNAVAILABLE"},{status:503}));
  render(<AgentPage mode="mine"/>);expect(await screen.findByRole("link",{name:"进入使用"})).toHaveAttribute("href","/workbench/agents/mine/private/"+id);expect(await screen.findByText("智能体配置不可用")).toBeInTheDocument();
});
it("selects a saved draft and restores only its frozen original reference and key after UNKNOWN",async()=>{
  const fetch=vi.spyOn(globalThis,"fetch").mockImplementation(async(url,init)=>{if(init?.method==="POST")throw new Error("response lost");return reads(url)});
  const first=render(<PrivateAgentPage id={id}/>);await choose();expect(screen.queryByRole("textbox",{name:"商品名称"})).not.toBeInTheDocument();fireEvent.click(screen.getByRole("button",{name:"检查并保存报告"}));await screen.findByText("结果尚未确认，请核实同一次检查。");
  const original=fetch.mock.calls.find(v=>v[1]?.method==="POST")!;expect(JSON.parse(String(original[1]?.body))).toEqual({recordId,expectedRevision:3});first.unmount();
  fetch.mockImplementation(async(url,init)=>init?.method==="POST"?Response.json({...run,key:new Headers(init.headers).get("Idempotency-Key")}):reads(url));
  render(<PrivateAgentPage id={id}/>);fireEvent.click(await screen.findByRole("button",{name:"核实同一次检查"}));await screen.findByRole("heading",{name:"草稿质检报告 · 盒子"});
  const calls=fetch.mock.calls.filter(v=>v[1]?.method==="POST");expect(calls).toHaveLength(2);expect(calls[1][1]?.body).toBe(original[1]?.body);expect(new Headers(calls[1][1]?.headers).get("Idempotency-Key")).toBe(new Headers(original[1]?.headers).get("Idempotency-Key"));
  await waitFor(()=>expect(screen.queryByRole("button",{name:"核实同一次检查"})).not.toBeInTheDocument());expect(screen.getByText("选择当前店铺可发布的末级类目")).toBeInTheDocument();
});
it("requires a new explicit selection after known revision rejection without auto retry",async()=>{
  const fetch=vi.spyOn(globalThis,"fetch").mockImplementation(async(url,init)=>init?.method==="POST"?Response.json({code:"CUSTOMIZATION_REVISION_MISMATCH"},{status:412}):reads(url));
  render(<PrivateAgentPage id={id}/>);await choose();fireEvent.click(screen.getByRole("button",{name:"检查并保存报告"}));await screen.findByText("草稿版本或可用状态已变化，请重新选择当前草稿。");expect(fetch.mock.calls.filter(v=>v[1]?.method==="POST")).toHaveLength(1);expect(screen.getByRole("button",{name:"检查并保存报告"})).toBeDisabled();
});
it.each([400,404,412])("keeps the frozen UNKNOWN command after verification returns %i and restores it unchanged",async status=>{
  const fetch=vi.spyOn(globalThis,"fetch").mockImplementation(async(url,init)=>{if(init?.method==="POST")throw new Error("response lost");return reads(url)});
  const first=render(<PrivateAgentPage id={id}/>);await choose();fireEvent.click(screen.getByRole("button",{name:"检查并保存报告"}));await screen.findByText("结果尚未确认，请核实同一次检查。");
  const original=fetch.mock.calls.find(v=>v[1]?.method==="POST")!;first.unmount();
  fetch.mockImplementation(async(url,init)=>init?.method==="POST"?Response.json({code:"CUSTOMIZATION_NOT_FOUND"},{status}):reads(url));
  const second=render(<PrivateAgentPage id={id}/>);fireEvent.click(await screen.findByRole("button",{name:"核实同一次检查"}));await waitFor(()=>expect(fetch.mock.calls.filter(v=>v[1]?.method==="POST")).toHaveLength(2));
  await screen.findByText("暂时无法核实原检查，请保留原草稿引用与请求编号，恢复来源访问后核实。");expect(screen.getByRole("button",{name:"核实同一次检查"})).toBeInTheDocument();expect(screen.getByRole("button",{name:"检查并保存报告"})).toBeDisabled();second.unmount();
  fetch.mockImplementation(async(url,init)=>init?.method==="POST"?Response.json({...run,key:new Headers(init.headers).get("Idempotency-Key")}):reads(url));
  render(<PrivateAgentPage id={id}/>);fireEvent.click(await screen.findByRole("button",{name:"核实同一次检查"}));await screen.findByRole("heading",{name:"草稿质检报告 · 盒子"});
  const calls=fetch.mock.calls.filter(v=>v[1]?.method==="POST");expect(calls).toHaveLength(3);for(const call of calls){expect(call[1]?.body).toBe(original[1]?.body);expect(new Headers(call[1]?.headers).get("Idempotency-Key")).toBe(new Headers(original[1]?.headers).get("Idempotency-Key"))}
});
it("reads actor-private summaries and fetches complete details only on selection",async()=>{
  const fetch=vi.spyOn(globalThis,"fetch").mockImplementation(async url=>String(url).endsWith("/reports")?Response.json({items:[summary],nextCursor:""}):reads(url));
  const page=render(<PrivateAgentPage id={id}/>);fireEvent.click(await screen.findByRole("button",{name:"查看报告"}));await screen.findByRole("heading",{name:"草稿质检报告 · 盒子"});expect(fetch.mock.calls.some(([url])=>String(url).endsWith(`/reports/${id}`))).toBe(true);
  state.permissions=permissions.filter(p=>p!=="workbench.collection.read");page.rerender(<PrivateAgentPage id={id}/>);expect(screen.queryByRole("heading",{name:"草稿质检报告 · 盒子"})).not.toBeInTheDocument();
});
it("keeps old 1.0.0 facts read-only without offering a manual executor",async()=>{
  const old={...delivery,version:"1.0.0"};vi.spyOn(globalThis,"fetch").mockImplementation(async url=>String(url).endsWith("/reports")?Response.json({items:[],nextCursor:""}):Response.json(old));render(<PrivateAgentPage id={id}/>);await screen.findByText("此版本为原手工输入试用的只读历史，不能执行新的检查。");expect(screen.queryByRole("button",{name:"检查并保存报告"})).not.toBeInTheDocument();
});
