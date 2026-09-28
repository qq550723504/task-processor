import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {act,cleanup,render,screen,waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach,expect,it,vi} from "vitest";
import {WorkbenchContextProvider} from "@/components/providers/workbench-context-provider";
import {OrganizationSwitcher} from "@/components/workbench/organization-switcher";
import {KnowledgePage} from "./knowledge-page";
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
const baseId="4841d296-ef14-4c16-8d25-a7667e534feb",sourceId="d6c33a5b-95dd-4a3c-a420-652ec52b5f08",oldId="f5dd72eb-e639-436e-b057-38c713d12ce9",latestId="b6f6b343-b4d6-4db4-ac21-98585bc527f6";
const context={user:{id:"reader"},homeOrganizationId:"org-a",effectiveOrganizationId:"org-a",selectionRequired:false,organizations:[{id:"org-a",name:"企业甲",roles:[]},{id:"org-b",name:"企业乙",roles:[]}]};
const timestamp="2026-09-28T10:00:00Z";
const base={id:baseId,name:"品牌指南",state:"ACTIVE",version:1,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp};
const old={id:oldId,number:1,filename:"old.txt",contentType:"text/plain",sizeBytes:5,state:"AVAILABLE",createdAt:timestamp,updatedAt:timestamp};
function mount(baseId?:string){const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});const view=render(<QueryClientProvider client={client}><WorkbenchContextProvider><OrganizationSwitcher/><KnowledgePage baseId={baseId}/></WorkbenchContextProvider></QueryClientProvider>);return ()=>{view.unmount();client.clear();};}
it("shows processing replacements while previewing the prior readable revision",async()=>{
 const fetch=vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(url.endsWith("/sources"))return Response.json({items:[{id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:2,latestRevision:{...old,id:latestId,number:2,state:"PROCESSING"},currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp}]});
 if(url.endsWith("/preview"))return Response.json({revisionId:oldId,text:"原版本正文"});
 return Response.json(base);
 });vi.stubGlobal("fetch",fetch);const unmount=mount(baseId);
 expect(await screen.findByText("处理中")).toBeVisible();await userEvent.click(screen.getByRole("button",{name:"预览旧版本"}));expect(await screen.findByText("原版本正文")).toBeVisible();
 expect(fetch.mock.calls.some(([url])=>url.includes(oldId+"/preview"))).toBe(true);expect(screen.getByRole("button",{name:"重新上传"})).toBeDisabled();unmount();
});
it("hides a late response from the previous organization",async()=>{
 let release!:(value:Response)=>void;const late=new Promise<Response>(resolve=>{release=resolve;});const requests:RequestInit[]=[];
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(url==="/api/workbench/context/effective-organization")return Response.json({...context,effectiveOrganizationId:"org-b"});
 requests.push(init!);return new Headers(init?.headers).get("X-Expected-Organization-ID")==="org-a"?late:Response.json({items:[],pagination:{page:1,pageSize:20,total:0}});
 }));const unmount=mount();await waitFor(()=>expect(requests.length).toBe(1));await userEvent.selectOptions(screen.getByLabelText("当前企业"),"org-b");expect(await screen.findByText("当前企业还没有知识库")).toBeVisible();
 await act(async()=>release(Response.json({items:[base],pagination:{page:1,pageSize:20,total:1}})));expect(screen.queryByText("品牌指南")).not.toBeInTheDocument();expect(requests[0].signal?.aborted).toBe(true);unmount();
});
it("reuses the original command after admission succeeded but the HTTP response was 503",async()=>{
 const keys:string[]=[];
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(init?.method==="POST"){keys.push(new Headers(init.headers).get("Idempotency-Key")!);return keys.length===1?Response.json({code:"KNOWLEDGE_UNAVAILABLE"},{status:503}):Response.json({knowledgeBase:base},{status:201});}
 return Response.json({items:[],pagination:{page:1,pageSize:20,total:0}});
 }));
 const unmount=mount();await screen.findByText("当前企业还没有知识库");await userEvent.click(screen.getByRole("button",{name:"创建知识库"}));await userEvent.type(screen.getByLabelText("知识库名称"),"品牌指南");await userEvent.click(screen.getByRole("button",{name:"保存"}));
 await userEvent.click(await screen.findByRole("button",{name:"重试同一次操作"}));await screen.findByText("操作已保存。");expect(keys).toHaveLength(2);expect(keys[1]).toBe(keys[0]);unmount();
});
it("uploads a legal long filename with a separately bounded source display name",async()=>{
 let sent:FormData|undefined;
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(init?.method==="POST"){sent=init.body as FormData;return Response.json({code:"OUTCOME_UNKNOWN"},{status:503});}
 if(url.endsWith("/sources"))return Response.json({items:[]});return Response.json(base);
 }));
 const unmount=mount(baseId);await screen.findByText("尚未上传资料");
 const filename="a".repeat(121)+".txt";await userEvent.upload(screen.getByLabelText("上传新资料"),new File(["hello"],filename,{type:"text/plain"}));
 await userEvent.click(screen.getByRole("button",{name:"上传并处理"}));await waitFor(()=>expect(sent).toBeDefined());
 expect((sent!.get("file") as File).name).toBe(filename);expect([...(sent!.get("name") as string)].length).toBeLessThanOrEqual(120);unmount();
});
