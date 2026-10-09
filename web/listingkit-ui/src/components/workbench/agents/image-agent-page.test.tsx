import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ImageAgentPage } from "./image-agent-page";
import { imageTemplateSchema } from "@/lib/contracts/image-set-configuration";

const id="product.image.agent";
const detail={agent:{agentId:id,activation:"ENABLED",revision:"1",activationEpoch:"1",defaultTemplate:null,updatedAt:"2026-10-09T00:00:00Z"},name:"商品图片智能体",description:"整套图片",definitionVersion:"v1.0.0",parameterSchema:"image-config-v1",canConfigure:true,canUse:false,canReadRuns:false,capabilities:[]};
afterEach(()=>{cleanup();vi.unstubAllGlobals()});

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
