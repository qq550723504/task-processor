import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { RolePermissions } from "./role-permissions";
const api = vi.hoisted(() => ({ read: vi.fn(), create: vi.fn(), save: vi.fn() }));
vi.mock("@/lib/api/enterprise-roles", async original => ({ ...(await original<typeof import("@/lib/api/enterprise-roles")>()), getEnterpriseRoles: api.read, createEnterpriseRole: api.create, saveEnterpriseRole: api.save }));
const key="sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_01";
const role={id:key,name:"客服",modules:["members"],version:1,system:false};
const data={schemaVersion:"enterprise-roles-v1",userId:"actor",organizationId:"org",items:[{id:"listingkit_admin",name:"管理员",modules:["members"],version:0,system:true},role],catalog:[{id:"members",group:"我的账户",label:"成员与权限",available:true,permissions:[]},{id:"reports",group:"AI工作台",label:"我的报告",available:false,permissions:[]}],canManage:true,canCreate:true,remainingSlots:63};
beforeEach(()=>{Object.defineProperty(HTMLDialogElement.prototype,"showModal",{configurable:true,value:function(this:HTMLDialogElement){this.open=true}});Object.defineProperty(HTMLDialogElement.prototype,"close",{configurable:true,value:function(this:HTMLDialogElement){this.open=false}});});
afterEach(()=>{cleanup();sessionStorage.clear();vi.clearAllMocks();});
function mount(){api.read.mockResolvedValue(data);const client=new QueryClient({defaultOptions:{queries:{retry:false}}});return render(<QueryClientProvider client={client}><RolePermissions scope={{expectedUserId:"actor",expectedOrganizationId:"org"}} onChanged={vi.fn()} onAuthorityFailure={vi.fn()} /></QueryClientProvider>);}
it("saves only available modules with the displayed role revision",async()=>{
 api.save.mockResolvedValue({role:{...role,modules:[],version:2}});mount();const user=userEvent.setup();
 await user.click(await screen.findByRole("button",{name:"客服"}));await user.click(screen.getByText("我的账户",{selector:"strong"}));await user.click(screen.getByRole("button",{name:"移除成员与权限"}));
 await user.click(screen.getByRole("button",{name:"保存"}));await waitFor(()=>expect(api.save).toHaveBeenCalledOnce());expect(api.save.mock.calls[0][2]).toBe(key);expect(api.save.mock.calls[0][3]).toEqual({modules:[],expectedVersion:1});
});
it("retains the exact creation key and payload after a lost response",async()=>{
 api.create.mockRejectedValueOnce(new Error("response lost")).mockResolvedValueOnce({role});mount();const user=userEvent.setup();
 await user.click(await screen.findByRole("button",{name:"＋ 新建角色"}));await user.type(screen.getByRole("textbox",{name:/角色名称/}),"客服");await user.click(screen.getByRole("button",{name:"创建"}));
 await screen.findByRole("button",{name:"核实原保存"});expect(screen.queryByRole("dialog",{name:"新建角色"})).not.toBeInTheDocument();
 await user.click(screen.getByRole("button",{name:"核实原保存"}));await waitFor(()=>expect(api.create).toHaveBeenCalledTimes(2));expect(api.create.mock.calls[1][1]).toBe(api.create.mock.calls[0][1]);expect(api.create.mock.calls[1][2]).toEqual(api.create.mock.calls[0][2]);expect(api.create.mock.calls[1][2]).toEqual({name:"客服",modules:[]});
});
