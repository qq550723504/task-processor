import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ImageAgentPage } from "./image-agent-page";
import { imageTemplateSchema } from "@/lib/contracts/image-set-configuration";

const configurationContext=vi.hoisted(()=>{
  const guards=new Set<()=>boolean>();
  return {guards,registerOrganizationSwitchGuard:vi.fn((guard:()=>boolean)=>{guards.add(guard);return()=>guards.delete(guard);})};
});
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>configurationContext}));

const id="product.image.agent";
const detail={agent:{agentId:id,activation:"ENABLED",revision:"1",activationEpoch:"1",defaultTemplate:null,updatedAt:"2026-10-09T00:00:00Z"},name:"商品图片智能体",description:"整套图片",definitionVersion:"v1.0.0",parameterSchema:"image-config-v1",canConfigure:true,canUse:false,canReadRuns:false,capabilities:[]};
beforeEach(()=>{localStorage.clear();configurationContext.guards.clear();});
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.restoreAllMocks();localStorage.clear();});

it("restores an aborted template request only in its original scope and preserves it through a temporary denial",async()=>{
  let attempts=0;
  const fetch=vi.fn(async(url:unknown,init?:RequestInit)=>{
    if(init?.method==="POST"){
      attempts++;
      if(attempts===1)return new Promise<Response>((_,reject)=>init.signal?.addEventListener("abort",()=>reject(new Error("lost response")),{once:true}));
      if(attempts===2)return Response.json({code:"FORBIDDEN"},{status:403});
      return Response.json({commandId:"8fc227bb-b572-4138-8e2a-5f1a0be98617",operation:"create-template",agentId:id,templateId:"45a227bb-b572-4138-8e2a-5f1a0be98617",revision:"1",version:"1",noop:false,committedAt:"2026-10-09T00:00:00Z"});
    }
    return Response.json(String(url).includes("/templates")?{items:[],nextCursor:""}:detail);
  });
  vi.stubGlobal("fetch",fetch);
  const first=render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-a"}} organization="企业A"/>);
  fireEvent.click(await screen.findByRole("button",{name:"新建图片模板"}));
  fireEvent.change(screen.getByLabelText("模板名称"),{target:{value:"必须保留的完整模板"}});
  fireEvent.click(screen.getByRole("button",{name:"保存图片模板"}));
  await waitFor(()=>expect(attempts).toBe(1));
  first.unmount();
  const other=render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-b"}} organization="企业B"/>);
  await screen.findByRole("button",{name:"新建图片模板"});
  expect(screen.queryByRole("button",{name:"核实原操作"})).not.toBeInTheDocument();
  other.unmount();
  render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-a"}} organization="企业A"/>);
  const verify=await screen.findByRole("button",{name:"核实原操作"});
  expect(attempts).toBe(1);
  expect([...configurationContext.guards].every(guard=>guard())).toBe(false);
  expect(screen.getByRole("button",{name:"新建图片模板"})).toBeDisabled();
  fireEvent.click(verify);
  await screen.findByText("当前身份没有操作权限。");
  const retry=screen.getByRole("button",{name:"核实原操作"});
  await waitFor(()=>expect(retry).toBeEnabled());
  fireEvent.click(retry);
  await screen.findByText("操作已保存，正在读取当前企业配置。");
  const saves=fetch.mock.calls.filter(([,init])=>init?.method==="POST");
  expect(saves).toHaveLength(3);
  for(const [,init] of saves){
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe(new Headers(saves[0][1]?.headers).get("Idempotency-Key"));
    expect(new Headers(init?.headers).get("X-Expected-Organization-ID")).toBe("org-a");
    expect(init?.body).toBe(saves[0][1]?.body);
  }
  expect(localStorage.length).toBe(0);
  await waitFor(()=>expect([...configurationContext.guards].every(guard=>guard())).toBe(true));
});

it("does not dispatch configuration when the original command cannot be stored",async()=>{
  const fetch=vi.fn<(url:unknown,init?:RequestInit)=>Promise<Response>>(async(url)=>Response.json(String(url).includes("/templates")?{items:[],nextCursor:""}:detail));
  vi.stubGlobal("fetch",fetch);
  render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-a"}} organization="企业A"/>);
  fireEvent.click(await screen.findByRole("button",{name:"新建图片模板"}));
  fireEvent.change(screen.getByLabelText("模板名称"),{target:{value:"存储失败时不能发送"}});
  vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw new DOMException("Full","QuotaExceededError");});
  fireEvent.click(screen.getByRole("button",{name:"保存图片模板"}));
  await screen.findByText("当前能力或依赖不可用，请稍后重试。");
  expect(fetch.mock.calls.some(([,init])=>init?.method==="POST")).toBe(false);
});

it("pages legal near-limit image templates within the configuration response cap",async()=>{
  const image={schema:"image-config-v1",mode:"custom",shareOriginals:true,background:"",language:"en",carousel:Array.from({length:32},(_,i)=>({id:`task-${i}`,purpose:"custom",brief:"x".repeat(1990)})),detail:[]};
  image.background="x".repeat(65520-new TextEncoder().encode(JSON.stringify(image)).length);
  const rows=["Large first","Large second"].map((name,i)=>imageTemplateSchema.parse({templateId:`45a227bb-b572-4138-8e2a-5f1a0be9861${i}`,agentId:id,lifecycle:"ACTIVE",revision:"1",version:"1",schemaVersion:"image-config-v1",name:`${name} ${"😀".repeat(100)}`,targetPlatform:"product",createdAt:"2026-10-09T00:00:00Z",image}));
  expect(new TextEncoder().encode(JSON.stringify({items:rows,nextCursor:""})).length).toBeGreaterThan(128*1024);
  const fetch=vi.fn(async(input:unknown)=>{
    const url=new URL(String(input),"https://app.test");
    if(!url.pathname.endsWith("/templates"))return Response.json(detail);
    const page=url.searchParams.get("pageSize")==="1"?{items:[rows[url.searchParams.has("cursor")?1:0]],nextCursor:url.searchParams.has("cursor")?"":"next-page"}:{items:rows,nextCursor:""};
    return new TextEncoder().encode(JSON.stringify(page)).length>128*1024?Response.json({code:"DEPENDENCY_UNAVAILABLE"},{status:503}):Response.json(page);
  });
  vi.stubGlobal("fetch",fetch);
  render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-a"}} organization="企业A"/>);
  await screen.findByRole("heading",{name:/Large first/});
  fireEvent.click(screen.getByRole("button",{name:"下一页"}));
  await screen.findByRole("heading",{name:/Large second/});
  fireEvent.click(screen.getByRole("button",{name:"编辑"}));
  expect(screen.getByLabelText("模板名称")).toHaveValue(rows[1]!.name);
  expect(fetch.mock.calls.filter(([url])=>String(url).includes("/templates?")).map(([url])=>new URL(String(url),"https://app.test").searchParams.get("pageSize"))).toEqual(["1","1"]);
});

it("keeps the original full template save frozen when its receipt is unknown",async()=>{
  let attempts=0;
  const fetch=vi.fn(async(url:unknown,init?:RequestInit)=>{
    if(init?.method==="POST"){
      if(++attempts===1)throw new Error("lost response");
      return Response.json({commandId:"8fc227bb-b572-4138-8e2a-5f1a0be98617",operation:"create-template",agentId:id,templateId:"45a227bb-b572-4138-8e2a-5f1a0be98617",revision:"1",version:"1",noop:false,committedAt:"2026-10-09T00:00:00Z"});
    }
    return Response.json(String(url).includes("/templates")?{items:[],nextCursor:""}:detail);
  });
  vi.stubGlobal("fetch",fetch);
  render(<ImageAgentPage scope={{userId:"actor",organizationId:"org-a"}} organization="企业A"/>);
  fireEvent.click(await screen.findByRole("button",{name:"新建图片模板"}));
  fireEvent.change(screen.getByLabelText("模板名称"),{target:{value:"完整商品图片"}});
  fireEvent.click(screen.getByRole("button",{name:"保存图片模板"}));
  const verify=await screen.findByRole("button",{name:"核实原操作"});
  expect(screen.getByRole("button",{name:"保存图片模板"})).toBeDisabled();
  expect(screen.getByRole("button",{name:"停用智能体"})).toBeDisabled();
  fireEvent.click(verify);
  await waitFor(()=>expect(attempts).toBe(2));
  const saves=fetch.mock.calls.filter(([,init])=>init?.method==="POST");
  expect(new Headers(saves[0][1]?.headers).get("Idempotency-Key")).toBe(new Headers(saves[1][1]?.headers).get("Idempotency-Key"));
  expect(saves[0][1]?.body).toBe(saves[1][1]?.body);
  const payload=JSON.parse(String(saves[0][1]?.body));
  expect(payload.image.carousel).toHaveLength(1);
  expect(payload.image.detail).toHaveLength(1);
  expect(payload.image.shareOriginals).toBe(true);
  expect(payload.defaultKnowledgeBaseId).toBeUndefined();
});
