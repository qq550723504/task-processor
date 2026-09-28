import {QueryClient,QueryClientProvider} from "@tanstack/react-query";
import {act,cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach,expect,it,vi} from "vitest";
import {WorkbenchContextProvider,useWorkbenchContext} from "@/components/providers/workbench-context-provider";
import {OrganizationSwitcher} from "@/components/workbench/organization-switcher";
import {KnowledgePage} from "./knowledge-page";
afterEach(()=>{cleanup();vi.useRealTimers();vi.unstubAllGlobals();});
const baseId="4841d296-ef14-4c16-8d25-a7667e534feb",sourceId="d6c33a5b-95dd-4a3c-a420-652ec52b5f08",oldId="f5dd72eb-e639-436e-b057-38c713d12ce9",latestId="b6f6b343-b4d6-4db4-ac21-98585bc527f6";
const context={user:{id:"reader"},homeOrganizationId:"org-a",effectiveOrganizationId:"org-a",selectionRequired:false,organizations:[{id:"org-a",name:"企业甲",roles:["listingkit_admin"]},{id:"org-b",name:"企业乙",roles:["listingkit_admin"]}]};
const timestamp="2026-09-28T10:00:00Z";
const base={id:baseId,name:"品牌指南",state:"ACTIVE",version:1,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp};
const old={id:oldId,number:1,filename:"old.txt",contentType:"text/plain",sizeBytes:5,state:"AVAILABLE",createdAt:timestamp,updatedAt:timestamp};
it("refreshes an externally renamed and disabled base alongside its sources",async()=>{
 vi.useFakeTimers();let current=base;
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(url.endsWith("/sources"))return Response.json({items:[{id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp}]});
 if(url.endsWith("/preview"))return Response.json({revisionId:oldId,text:"已保存正文"});return Response.json(current);
 }));const unmount=mount(baseId);await act(async()=>{await vi.advanceTimersByTimeAsync(10);});
 expect(screen.getByRole("heading",{name:"品牌指南"})).toBeVisible();
 current={...base,name:"更新后的指南",version:2};await act(async()=>{await vi.advanceTimersByTimeAsync(5010);});
 expect(screen.getByRole("heading",{name:"更新后的指南"})).toBeVisible();
 fireEvent.click(screen.getByRole("button",{name:"预览"}));await act(async()=>{await vi.advanceTimersByTimeAsync(10);});expect(screen.getByText("已保存正文")).toBeVisible();
 current={...current,state:"DISABLED",version:3};await act(async()=>{await vi.advanceTimersByTimeAsync(5010);});
 expect(screen.getByText("知识库已停用")).toBeVisible();expect(screen.queryByText("已保存正文")).not.toBeInTheDocument();
 for(const name of ["编辑名称","停用知识库","上传资料","重新上传","预览"])expect(screen.queryByRole("button",{name})).not.toBeInTheDocument();unmount();
});
function RefreshContext(){const context=useWorkbenchContext();return <button onClick={()=>void context.retry()}>更新企业授权</button>;}
function mount(baseId?:string,refresh=false,client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})){const view=render(<QueryClientProvider client={client}><WorkbenchContextProvider><OrganizationSwitcher/>{refresh?<RefreshContext/>:null}<KnowledgePage baseId={baseId}/></WorkbenchContextProvider></QueryClientProvider>);return ()=>{view.unmount();client.clear();};}
it.each(["listingkit_operator","listingkit_viewer","admin",""])("hides creation for current organization role %s even with admin in another organization",async(role)=>{
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>url==="/api/workbench/context"?Response.json({...context,organizations:[{...context.organizations[0],roles:role?[role]:[]},context.organizations[1]]}):Response.json({items:[base],pagination:{page:1,pageSize:20,total:1}})));
 const unmount=mount();if(role==="listingkit_operator"){await screen.findByText("品牌指南");expect(screen.getByRole("link",{name:"打开知识库"})).toBeVisible();}else{await screen.findByText("当前身份没有知识库读取权限");expect(screen.queryByText("品牌指南")).not.toBeInTheDocument();}expect(screen.queryByRole("button",{name:"创建知识库"})).not.toBeInTheDocument();unmount();
});
it.each([undefined,baseId])("clears protected reads on same-organization read revocation for %s",async(id)=>{
 let roles=["listingkit_operator"];
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json({...context,organizations:[{...context.organizations[0],roles},context.organizations[1]]});
 if(url.endsWith("/sources"))return Response.json({items:[{id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp}]});
 if(url.endsWith("/preview"))return Response.json({revisionId:oldId,text:"受保护正文"});return Response.json(id?base:{items:[base],pagination:{page:1,pageSize:20,total:1}});
 }));const unmount=mount(id,true);await screen.findByRole("heading",{name:"品牌指南"});
 if(id){await userEvent.click(await screen.findByRole("button",{name:"预览"}));await screen.findByText("受保护正文");}
 roles=["listingkit_viewer"];await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await screen.findByText("当前身份没有知识库读取权限");
 for(const value of ["品牌指南","产品资料","受保护正文"])expect(screen.queryAllByText(value)).toHaveLength(0);expect(screen.queryByText(/old.txt/)).not.toBeInTheDocument();
 roles=["listingkit_operator"];await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await screen.findByRole("heading",{name:"品牌指南"});unmount();
});
it.each(["list","base","sources","preview"])("suppresses all cached knowledge after a %s authorization error",async(route)=>{
 vi.useFakeTimers();let denied=false;const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 const kind=url.endsWith("/preview")?"preview":url.endsWith("/sources")?"sources":url.includes("page=")?"list":"base";
 if(denied && kind===route)return Response.json({code:"PERMISSION_DENIED"},{status:403});
 if(kind==="sources")return Response.json({items:[{id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp}]});
 if(kind==="preview")return Response.json({revisionId:oldId,text:"受保护正文"});return Response.json(kind==="list"?{items:[base],pagination:{page:1,pageSize:20,total:1}}:base);
 }));const unmount=mount(route==="list"?undefined:baseId,false,client);await act(async()=>{await vi.advanceTimersByTimeAsync(10);});
 if(route!=="list"){fireEvent.click(screen.getByRole("button",{name:"预览"}));await act(async()=>{await vi.advanceTimersByTimeAsync(10);});expect(screen.getByText("受保护正文")).toBeVisible();}
 denied=true;await act(async()=>{await client.invalidateQueries({queryKey:["knowledge","reader","org-a"]});await vi.advanceTimersByTimeAsync(10);});
 for(const value of ["品牌指南","产品资料","受保护正文"])expect(screen.queryAllByText(value)).toHaveLength(0);expect(screen.queryByText(/old.txt/)).not.toBeInTheDocument();expect(screen.getByText("当前身份没有知识库管理或读取权限。")).toBeVisible();unmount();
});
it("cancels an old read when roles lose access and rejects its late result",async()=>{
 let roles=["listingkit_operator"],sourceReads=0;let release!:(response:Response)=>void;const late=new Promise<Response>(resolve=>{release=resolve;});let signal:AbortSignal|undefined;
 const source={id:sourceId,knowledgeBaseId:baseId,name:"已撤销资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp};
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context")return Response.json({...context,organizations:[{...context.organizations[0],roles},context.organizations[1]]});
 if(url.endsWith("/sources")){if(sourceReads++===0){signal=init?.signal??undefined;return late;}return Response.json({items:[{...source,name:"重新授权资料"}]});}return Response.json(base);
 }));const unmount=mount(baseId,true);await waitFor(()=>expect(signal).toBeDefined());
 roles=["listingkit_viewer"];await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await screen.findByText("当前身份没有知识库读取权限");expect(signal?.aborted).toBe(true);
 await act(async()=>release(Response.json({items:[source]})));expect(screen.queryByText("已撤销资料")).not.toBeInTheDocument();
 roles=["listingkit_operator"];await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await screen.findByText("重新授权资料");expect(screen.queryByText("已撤销资料")).not.toBeInTheDocument();unmount();
});
it("keeps a preview authorization failure latched when a late source response changes its revision",async()=>{
 vi.useFakeTimers();let denied=false,release!:(response:Response)=>void;let sourceSignal:AbortSignal|undefined;const late=new Promise<Response>(resolve=>{release=resolve;});
 const source={id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp};
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context")return Response.json(context);
 if(url.endsWith("/sources")){if(denied){sourceSignal=init?.signal??undefined;return late;}return Response.json({items:[source]});}
 if(url.endsWith("/preview"))return denied?Response.json({code:"PERMISSION_DENIED"},{status:403}):Response.json({revisionId:oldId,text:"受保护正文"});return Response.json(base);
 }));const unmount=mount(baseId);await act(async()=>{await vi.advanceTimersByTimeAsync(10);});fireEvent.click(screen.getByRole("button",{name:"预览"}));await act(async()=>{await vi.advanceTimersByTimeAsync(10);});expect(screen.getByText("受保护正文")).toBeVisible();
 denied=true;await act(async()=>{await vi.advanceTimersByTimeAsync(5010);});expect(screen.getByText("当前身份没有知识库管理或读取权限。")).toBeVisible();
 await act(async()=>{release(Response.json({items:[{...source,latestRevision:{...old,id:latestId,number:2},currentReadableRevision:{...old,id:latestId,number:2}}]}));await vi.advanceTimersByTimeAsync(10);});
 for(const value of ["品牌指南","产品资料","受保护正文"])expect(screen.queryAllByText(value)).toHaveLength(0);expect(screen.queryByText(/old.txt/)).not.toBeInTheDocument();expect(sourceSignal?.aborted).toBe(true);
 denied=false;fireEvent.click(screen.getByRole("button",{name:"重新确认"}));await act(async()=>{await vi.advanceTimersByTimeAsync(10);});await act(async()=>{await vi.advanceTimersByTimeAsync(10);});expect(screen.getByRole("heading",{name:"品牌指南"})).toBeVisible();unmount();
});
it("lets an operator preview saved text while hiding all management controls",async()=>{
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json({...context,organizations:[{...context.organizations[0],roles:["listingkit_operator"]},context.organizations[1]]});
 if(url.endsWith("/sources"))return Response.json({items:[{id:sourceId,knowledgeBaseId:baseId,name:"产品资料",state:"ACTIVE",version:1,latestRevision:old,currentReadableRevision:old,createdBy:"reader",updatedBy:"reader",createdAt:timestamp,updatedAt:timestamp}]});
 if(url.endsWith("/preview"))return Response.json({revisionId:oldId,text:"只读正文"});return Response.json(base);
 }));const unmount=mount(baseId);await screen.findByText("产品资料");
 for(const name of ["编辑名称","停用知识库","上传资料","上传并处理","重新上传","停用"])expect(screen.queryByRole("button",{name})).not.toBeInTheDocument();
 expect(screen.queryByLabelText("上传新资料")).not.toBeInTheDocument();expect(screen.queryByLabelText("资料名称")).not.toBeInTheDocument();
 await userEvent.click(screen.getByRole("button",{name:"预览"}));expect(await screen.findByText("只读正文")).toBeVisible();unmount();
});
it.each(["listingkit_admin","platform_admin"])("retains management controls for verified %s including configured admin projection",async(role)=>{
 vi.stubGlobal("fetch",vi.fn(async(url:string)=>{
 if(url==="/api/workbench/context")return Response.json({...context,organizations:[{...context.organizations[0],roles:[role]},context.organizations[1]]});
 if(url.endsWith("/sources"))return Response.json({items:[]});return Response.json(base);
 }));const unmount=mount(baseId);await screen.findByText("尚未上传资料");
 expect(screen.getByRole("button",{name:"编辑名称"})).toBeVisible();expect(screen.getByRole("button",{name:"停用知识库"})).toBeVisible();expect(screen.getByRole("button",{name:"上传资料"})).toBeVisible();expect(screen.getByLabelText("上传新资料")).toBeVisible();unmount();
});
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
it("retains the original unknown intent when refreshed authority becomes read-only",async()=>{
 const keys:string[]=[];let contextReads=0;
 vi.stubGlobal("fetch",vi.fn(async(url:string,init?:RequestInit)=>{
 if(url==="/api/workbench/context"){const roles=[["listingkit_admin"],["listingkit_operator"],["listingkit_viewer"]][contextReads++]??["listingkit_admin"];return Response.json({...context,organizations:[{...context.organizations[0],roles},context.organizations[1]]});}
 if(init?.method==="POST"){keys.push(new Headers(init.headers).get("Idempotency-Key")!);return keys.length===1?Response.json({code:"KNOWLEDGE_UNAVAILABLE"},{status:503}):keys.length===2?Response.json({code:"PERMISSION_DENIED"},{status:403}):Response.json({knowledgeBase:base},{status:201});}
 return Response.json({items:[],pagination:{page:1,pageSize:20,total:0}});
 }));const unmount=mount(undefined,true);await screen.findByText("当前企业还没有知识库");
 await userEvent.click(screen.getByRole("button",{name:"创建知识库"}));await userEvent.type(screen.getByLabelText("知识库名称"),"品牌指南");await userEvent.click(screen.getByRole("button",{name:"保存"}));await screen.findByRole("button",{name:"重试同一次操作"});
 await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await waitFor(()=>expect(screen.queryByRole("button",{name:"创建知识库"})).not.toBeInTheDocument());
 await userEvent.selectOptions(screen.getByLabelText("当前企业"),"org-b");await waitFor(()=>expect(screen.getByLabelText("当前企业")).toHaveValue("org-a"));
 await userEvent.click(screen.getByRole("button",{name:"重试同一次操作"}));await waitFor(()=>expect(keys).toHaveLength(2));expect(keys[1]).toBe(keys[0]);await screen.findByRole("button",{name:"重试同一次操作"});
 await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await screen.findByText("当前身份没有知识库读取权限");expect(screen.queryByRole("button",{name:"重试同一次操作"})).not.toBeInTheDocument();
 await userEvent.click(screen.getByRole("button",{name:"更新企业授权"}));await waitFor(()=>expect(screen.getByRole("button",{name:"创建知识库"})).toBeDisabled());
 await userEvent.click(screen.getByRole("button",{name:"重试同一次操作"}));await screen.findByText("操作已保存。");expect(keys).toHaveLength(3);expect(keys[2]).toBe(keys[0]);unmount();
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
 await userEvent.type(screen.getByLabelText("资料名称"),"{Home} {End} ");
 await userEvent.click(screen.getByRole("button",{name:"上传并处理"}));await waitFor(()=>expect(sent).toBeDefined());
 expect((sent!.get("file") as File).name).toBe(filename);expect([...(sent!.get("name") as string)].length).toBeLessThanOrEqual(120);unmount();
});
it("keeps a near-limit file from exceeding the whole multipart request budget",async()=>{
 const requests:RequestInit[]=[];const fetch=vi.fn(async(url:string,init?:RequestInit)=>{
 requests.push(init??{});
 if(url==="/api/workbench/context")return Response.json(context);
 if(url.endsWith("/sources"))return Response.json({items:[]});return Response.json(base);
 });vi.stubGlobal("fetch",fetch);const unmount=mount(baseId);await screen.findByText("尚未上传资料");
 await userEvent.upload(screen.getByLabelText("上传新资料"),new File([new Uint8Array(10*1024*1024-1)],"near-limit.txt",{type:"text/plain"}));
 expect(screen.getByRole("button",{name:"上传并处理"})).toBeDisabled();expect(screen.getByText("文件接近大小上限，请缩小一点后上传。")).toBeVisible();
 expect(requests.every(init=>init.method!=="POST")).toBe(true);unmount();
});
