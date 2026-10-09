import {act,cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {beforeEach,afterEach,it,expect,vi} from "vitest";
import {ProjectPage} from "./project-page";
import {ProjectError} from "@/lib/api/project-center";
const f=vi.hoisted(()=>({request:vi.fn(),push:vi.fn(),context:{user:{id:"user-a"},effectiveOrganization:{id:"org-a"},permissions:["workbench.project.read","workbench.project.manage"],projectCenterAvailable:true,isLoading:false,isSwitching:false,selectionRequired:false,error:null,blockingError:null}}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>f.context}));
vi.mock("@/lib/api/project-center",async original=>({...await original<typeof import("@/lib/api/project-center")>(),projectRequest:f.request}));
vi.mock("next/navigation",()=>({useRouter:()=>({push:f.push})}));
const id="550e8400-e29b-41d4-a716-446655440000";
const p={id,title:"原企业项目",goal:"长期目标",kind:"OTHER",dueDate:"",revision:1,archived:false,createdAt:"2026-10-09T00:00:00Z",updatedAt:"2026-10-09T00:00:00Z",storeScope:false,references:[],taskTotal:0,taskCompleted:0,taskPending:0,taskSummaryAvailable:true};
beforeEach(()=>{f.context.effectiveOrganization={id:"org-a"};f.context.isSwitching=false;f.context.projectCenterAvailable=true;f.context.permissions=["workbench.project.read","workbench.project.manage"];f.request.mockReset();f.push.mockReset();sessionStorage.clear();f.request.mockResolvedValue({projects:[],next:""});HTMLDialogElement.prototype.showModal=vi.fn();HTMLDialogElement.prototype.close=vi.fn();});
afterEach(()=>{cleanup();sessionStorage.clear();});
it("releases a recovered operation after definitive revision rejection so fresh editing is possible",async()=>{
 const intent={path:"/"+id+"/archive",method:"POST",body:{},key:id,revision:1};
 sessionStorage.setItem("project-center:intent:user-a:org-a",JSON.stringify(intent));
 f.request.mockImplementation(async(_scope,path,_schema,_signal,command)=>command && !path.endsWith("visit")?Promise.reject(new ProjectError("REVISION_MISMATCH")):path.endsWith("visit")?{id,revision:2,replayed:false}:{...p,revision:2});
 render(<ProjectPage projectId={id}/>);await screen.findByRole("button",{name:"重试原操作"});fireEvent.click(screen.getByRole("button",{name:"重试原操作"}));
 await screen.findByText("项目已更新，请刷新后重新操作。");await waitFor(()=>expect(sessionStorage.getItem("project-center:intent:user-a:org-a")).toBeNull());
 expect(screen.queryByRole("button",{name:"重试原操作"})).toBeNull();expect(screen.getByRole("button",{name:"编辑项目"})).toBeEnabled();
});
it("requires restoration before saving a template from an archived project",async()=>{
 f.request.mockImplementation(async(_scope,path)=>path.endsWith("visit")?{id,revision:2,replayed:false}:{...p,revision:2,archived:true});render(<ProjectPage projectId={id}/>);await screen.findByRole("button",{name:"恢复项目"});expect(screen.queryByLabelText("模板名称")).toBeNull();
});
it("does not require AI Workbench or a model to create a personal project",async()=>{
 render(<ProjectPage/>);await screen.findByRole("button",{name:"＋ 新建项目"});await waitFor(()=>expect(screen.getByRole("button",{name:"＋ 新建项目"})).toBeEnabled());fireEvent.click(screen.getByRole("button",{name:"＋ 新建项目"}));fireEvent.change(screen.getByLabelText("项目名称"),{target:{value:"目标项目"}});fireEvent.change(screen.getByLabelText("项目目标"),{target:{value:"实际目标"}});
 f.request.mockResolvedValueOnce({id,revision:1,replayed:false});fireEvent.submit(screen.getByLabelText("项目名称").closest("form")!);await waitFor(()=>expect(f.push).toHaveBeenCalledWith("/workbench/ai/projects/"+id));const call=f.request.mock.calls.find(c=>c[4]?.method==="POST");expect(call?.[4].body).toEqual({title:"目标项目",goal:"实际目标",kind:"OTHER",dueDate:"",storeId:""});
});
it("keeps the exact intent after an unknown response and remount",async()=>{
 const view=render(<ProjectPage projectId={id}/>);f.request.mockImplementation(async(_s,path,_z,_signal,intent)=>{if(intent && !path.endsWith("visit"))throw new ProjectError("OUTCOME_UNKNOWN");return path.endsWith("visit")?{id,revision:1,replayed:false}:p;});view.rerender(<ProjectPage projectId={id}/>);await screen.findByRole("button",{name:"归档项目"});fireEvent.click(screen.getByRole("button",{name:"归档项目"}));await screen.findByRole("button",{name:"重试原操作"});const original=JSON.parse(sessionStorage.getItem("project-center:intent:user-a:org-a")!);view.unmount();render(<ProjectPage projectId={id}/>);await screen.findByRole("button",{name:"重试原操作"});f.request.mockResolvedValueOnce({id,revision:2,replayed:true});fireEvent.click(screen.getByRole("button",{name:"重试原操作"}));await waitFor(()=>expect(sessionStorage.getItem("project-center:intent:user-a:org-a")).toBeNull());expect(f.request.mock.calls.some(c=>c[4]?.key===original.key && c[4]?.revision===original.revision && c[4]?.path===original.path)).toBe(true);
});
it("discards old reads across A to B to A without using a cached response",async()=>{
 let finish!:(v:unknown)=>void;f.request.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));const v=render(<ProjectPage/>);await waitFor(()=>expect(f.request).toHaveBeenCalled());f.context.effectiveOrganization={id:"org-b"};v.rerender(<ProjectPage/>);await screen.findByText("还没有项目");f.context.effectiveOrganization={id:"org-a"};v.rerender(<ProjectPage/>);await act(async()=>finish({projects:[p],next:""}));expect(screen.queryByText("原企业项目")).toBeNull();expect(f.request.mock.calls.filter(c=>c[0].organizationId==="org-a")).toHaveLength(2);
});
it("does not expose protected reference fields or fabricated completion on unavailable tasks",async()=>{
 f.request.mockImplementation(async(_s,path)=>path.endsWith("visit")?{id,revision:1,replayed:false}:{...p,references:[{slotId:id,kind:"BUSINESS_TASK",available:false}],taskTotal:1,taskSummaryAvailable:false});render(<ProjectPage projectId={id}/>);await screen.findByText("任务状态暂不可用");fireEvent.click(screen.getByRole("button",{name:"任务"}));expect(screen.getByText("内容当前不可访问")).toBeVisible();expect(screen.queryByRole("progressbar")).toBeNull();expect(screen.getByRole("button",{name:"移除关联"})).toBeVisible();
});

it("keeps an unknown create key through permission denial and later authentication denial until a receipt arrives",async()=>{
 const intent={path:"",method:"POST",body:{title:"目标",goal:"长期目标",kind:"OTHER",dueDate:"",storeId:""},key:id};
 sessionStorage.setItem("project-center:intent:user-a:org-a",JSON.stringify(intent));
 f.request.mockImplementation(async(_scope,_path,_schema,_signal,command)=>{if(command)throw new ProjectError("FORBIDDEN");return {projects:[],next:""};});
 let view=render(<ProjectPage/>);await screen.findByRole("button",{name:"重试原操作"});fireEvent.click(screen.getByRole("button",{name:"重试原操作"}));await screen.findByText("当前权限不可用，请重新确认企业。");expect(JSON.parse(sessionStorage.getItem("project-center:intent:user-a:org-a")!).key).toBe(id);view.unmount();
 f.request.mockImplementation(async(_scope,_path,_schema,_signal,command)=>{if(command)throw new ProjectError("AUTHENTICATION_REQUIRED");return {projects:[],next:""};});view=render(<ProjectPage/>);await screen.findByRole("button",{name:"重试原操作"});fireEvent.click(screen.getByRole("button",{name:"重试原操作"}));await waitFor(()=>expect(screen.getByRole("button",{name:"重试原操作"})).toBeEnabled());expect(sessionStorage.getItem("project-center:intent:user-a:org-a")).not.toBeNull();view.unmount();
 f.request.mockImplementation(async(_scope,_path,_schema,_signal,command)=>command?{id,revision:1,replayed:true}:{projects:[],next:""});render(<ProjectPage/>);await screen.findByRole("button",{name:"重试原操作"});fireEvent.click(screen.getByRole("button",{name:"重试原操作"}));await waitFor(()=>expect(sessionStorage.getItem("project-center:intent:user-a:org-a")).toBeNull());expect(f.push).toHaveBeenCalledWith("/workbench/ai/projects/"+id);expect(f.request.mock.calls.filter(c=>c[4]).every(c=>c[4].key===id)).toBe(true);
});
