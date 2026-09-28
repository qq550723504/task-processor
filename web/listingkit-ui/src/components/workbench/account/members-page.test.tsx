import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { createRequire } from "node:module";
import { afterEach, beforeAll, expect, it, vi } from "vitest";
import { MembersPage } from "./members-page";

// Reuse the axe engine already installed by the repository's browser test dependency.
const load = createRequire(import.meta.url);
const axe = load(load.resolve("axe-core", {paths:[load.resolve("@axe-core/playwright")]})) as {run:(node:HTMLElement,options:unknown)=>Promise<{violations:unknown[]}>};

const state = vi.hoisted(() => ({ switching: false, org: "org", counts:{total:1,active:1,administrators:0,inactive:0} }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "actor" }, effectiveOrganization: { id: state.org, name: "当前企业" }, roles: ["listingkit_admin"], isSwitching: state.switching, isLoading: false, selectionRequired: false, error: null, blockingError: null }) }));
const result = { schemaVersion: "membership-v1", userId: "actor", organizationId: "org", canManage: false, assignableRoles: [], total: 1, items: [{ id: "grant", userId: "member", projectId: "project", organizationId: "org", displayName: "成员甲", loginName: "a@example.com", roles: ["listingkit_viewer"], state: "active", createdAt: "2026-09-12T00:00:00Z", changedAt: "2026-09-12T00:00:00Z", observedVersion: "a".repeat(64), canChangeRole: false, canRemove: false, permissions: [] }] };
function withAdditionalReads(original:(url:string,init?:RequestInit)=>Promise<Response>) { return vi.fn((url:RequestInfo|URL,init?:RequestInit)=>{const path=String(url);if(path==="/api/account/members/summary")return Promise.resolve(Response.json({schemaVersion:"membership-summary-v1",userId:"actor",organizationId:state.org,...state.counts,source:"zitadel_authorization_v2",readAt:"2026-09-28T00:00:00Z"}));if(path==="/api/account/member-invitations/summary")return Promise.resolve(Response.json({schemaVersion:"membership-invitation-summary-v1",userId:"actor",organizationId:state.org,pending:0}));if(path==="/api/account/member-invitations"&&init?.method==="GET")return Promise.resolve(Response.json({schemaVersion:"membership-invitations-v1",userId:"actor",organizationId:state.org,items:[],total:0,pending:0,canNotify:true}));return original(String(url),init);});}
const clients: QueryClient[] = [];
beforeAll(() => {
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value: function (this: HTMLDialogElement) { this.open = true; } });
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value: function (this: HTMLDialogElement) { this.open = false; } });
});
afterEach(() => { cleanup(); clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); vi.restoreAllMocks(); state.switching = false; state.org = "org";state.counts={total:1,active:1,administrators:0,inactive:0}; sessionStorage.clear(); });
function page(client = new QueryClient()) { vi.stubGlobal("fetch",withAdditionalReads(globalThis.fetch)); clients.push(client); return <QueryClientProvider client={client}><MembersPage expectedUserId="actor" /></QueryClientProvider>; }
it("does not report an unread retained receipt as UNKNOWN after returning to the page", async () => {
  const unknown = "4841d296-ef14-4c16-8d25-a7667e534feb", completed = "ea0390e6-6fd0-4834-8e9c-277caf59c122";
  const retained = JSON.stringify([unknown, completed].map((key, index) => ({key, kind:"role", target:`grant-${index}`, input:{role:"listingkit_operator",expectedVersion:"a".repeat(64)}})));
  sessionStorage.setItem('membership.pending:["actor","org"]', retained);
  const receipt = {schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id:unknown,kind:"role",step:"update_authorization",status:"unknown",targetUserId:"member",authorizationId:"grant",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null};
  const calls = vi.fn().mockImplementation(url => Promise.resolve(Response.json(
    String(url).endsWith(completed) ? {...receipt,id:completed,status:"acknowledged",acknowledgment:{id:"grant",at:"2026-09-27T00:00:00Z"}} :
    String(url).endsWith(unknown) ? receipt :
    String(url).includes("member-operations") ? {schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[receipt],next:""} :
    {...result,canManage:true,assignableRoles:["listingkit_viewer"]}
  )));
  vi.stubGlobal("fetch", withAdditionalReads(calls)); render(page());
  expect(await screen.findByRole("button",{name:new RegExp(`${unknown}.*待核实`)})).toBeVisible();
  const unread = screen.getByRole("button",{name:new RegExp(completed)});
  expect(unread).toHaveTextContent("尚未读取回执");
  expect(unread).not.toHaveTextContent("待核实");
  await userEvent.setup().click(unread);
  expect(await screen.findByRole("heading",{name:"操作已获服务确认"})).toBeVisible();
  expect(screen.getByRole("button",{name:new RegExp(completed)})).toHaveTextContent("已有终局回执");
  expect(screen.getByRole("button",{name:new RegExp(unknown)})).toHaveTextContent("待核实");
  expect(sessionStorage.getItem('membership.pending:["actor","org"]')).toBe(retained);
  expect(calls.mock.calls.every(([,init])=>init.method === "GET")).toBe(true);
});
it("recovers durable pending in a new tab and keeps unrelated invitation available", async () => {
  const id = "ea0390e6-6fd0-4834-8e9c-277caf59c122";
  const receipt = {schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id,kind:"invite",step:"create_user",status:"unknown",targetUserId:"original-target",authorizationId:"",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null};
  const calls = vi.fn().mockImplementation((url) => Promise.resolve(Response.json(String(url).endsWith(id) ? receipt : String(url).includes("member-operations") ? {schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[receipt],next:""} : {...result,canManage:true,assignableRoles:["listingkit_viewer"]})));
  vi.stubGlobal("fetch", withAdditionalReads(calls));
  render(page());
  expect((await screen.findAllByText(id))[0]).toBeVisible();
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());
  expect(calls.mock.calls.every(([,init])=>init.method === "GET")).toBe(true);
});
it("retains an advanced GET receipt across selection and stale pending refetches, closing only its own record",async()=>{
  const first="4841d296-ef14-4c16-8d25-a7667e534feb", second="ea0390e6-6fd0-4834-8e9c-277caf59c122";
  sessionStorage.setItem('membership.pending:["actor","org"]',JSON.stringify({key:first,kind:"role",target:"grant",input:{role:"listingkit_operator",expectedVersion:"a".repeat(64)}}));
  const receipt={schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id:first,kind:"role",step:"update_authorization",status:"unknown",targetUserId:"member",authorizationId:"grant",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null};
  const other={...receipt,id:second,targetUserId:"other",authorizationId:"other-grant"};
  let reads=0;
  const calls=vi.fn().mockImplementation(url=>Promise.resolve(Response.json(String(url).endsWith(second) ? {...other,status:++reads===1 ? "rejected" : "unknown"} : String(url).endsWith(first) ? receipt : String(url).includes("member-operations") ? {schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[receipt,other],next:""} : {...result,canManage:true,assignableRoles:["listingkit_viewer"]})));
  vi.stubGlobal("fetch", withAdditionalReads(calls));render(page());const user=userEvent.setup();
  await user.click(await screen.findByRole("button",{name:new RegExp(second)}));
  expect(await screen.findByRole("button",{name:"关闭回执"})).toBeEnabled();
  await user.click(screen.getByRole("button",{name:new RegExp(first)}));
  await user.click(screen.getByRole("button",{name:new RegExp(second)}));
  await waitFor(()=>expect(reads).toBe(2));
  expect(screen.getByRole("button",{name:"关闭回执"})).toBeEnabled();
  await user.click(screen.getByRole("button",{name:"关闭回执"}));
  await user.click(screen.getByRole("button",{name:"刷新待处理操作"}));
  await waitFor(()=>expect(screen.queryByRole("button",{name:new RegExp(second)})).not.toBeInTheDocument());
  expect(sessionStorage.getItem('membership.pending:["actor","org"]')).toContain(first);
  expect(screen.getByRole("button",{name:"继续原操作"})).toBeEnabled();
});
it("does not send a new invitation when its local recovery record cannot be saved",async()=>{
  const calls=vi.fn().mockImplementation(url=>Promise.resolve(Response.json(String(url).includes("member-operations") ? {schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[],next:""} : {...result,canManage:true,assignableRoles:["listingkit_viewer"]})));
  vi.stubGlobal("fetch", withAdditionalReads(calls));render(page());const user=userEvent.setup();
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());await user.click(screen.getByRole("button",{name:"邀请成员"}));
  await user.type(screen.getByRole("textbox",{name:"受邀邮箱"}),"new@example.com");
  vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw new Error("storage unavailable");});
  await user.click(screen.getByRole("button",{name:"发送邀请邮件"}));
  expect(calls.mock.calls.every(([,init])=>init.method === "GET")).toBe(true);
  expect(screen.queryByRole("button",{name:"继续原操作"})).not.toBeInTheDocument();
});
it("blocks a fresh invitation when its retained create key cannot be read", async () => {
  sessionStorage.setItem('invitation.create:["actor","org"]', "broken-json");
  vi.stubGlobal("fetch", withAdditionalReads(vi.fn(url => Promise.resolve(Response.json(String(url).includes("member-operations") ? {schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[],next:""} : {...result,canManage:true,assignableRoles:["listingkit_viewer"]})))));
  render(page());
  await screen.findByText("成员甲");
  expect(await screen.findByRole("button", {name: "邀请成员"})).toBeDisabled();
});
it("uses backend capability instead of context role names", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json(result))));
  render(page()); expect(await screen.findByText("成员甲")).toBeVisible();
  expect(screen.queryByRole("button", { name: "邀请成员" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "移除成员" })).not.toBeInTheDocument();
});
it("shows exact owner-backed member summary labels and the seven-column member table", async () => {
  state.counts={total:3,active:2,administrators:1,inactive:1};
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json({ ...result, total: 3, items: [
    { ...result.items[0], displayName: "管理员甲", roles: ["listingkit_admin"], state: "active", createdAt: "2026-09-12T00:00:00Z", changedAt: "2026-09-13T00:00:00Z" },
    { ...result.items[0], id: "grant-2", userId: "user-2", displayName: "成员乙", roles: ["listingkit_viewer"], state: "active", createdAt: "2026-09-11T00:00:00Z", changedAt: "2026-09-13T00:00:00Z" },
    { ...result.items[0], id: "grant-3", userId: "user-3", displayName: "成员丙", roles: ["listingkit_viewer"], state: "inactive", createdAt: "2026-09-10T00:00:00Z", changedAt: "2026-09-12T00:00:00Z" },
  ] }))));
  render(page());
  const summary = await screen.findByRole("region", { name: "成员目录摘要" });
  expect(within(summary).getByText("正式成员")).toBeVisible();
  expect(within(summary).getByText("管理员")).toBeVisible();
  expect(within(summary).getByText("邀请中")).toBeVisible();
  expect(within(summary).getByText("已停用")).toBeVisible();
  expect(await within(summary).findByText("2", { exact: true })).toBeVisible();
  expect(within(summary).getAllByText("1", { exact: true })).toHaveLength(2);
  expect(within(summary).getByText("0", { exact: true })).toBeVisible();
  const table = screen.getByRole("table", { name: "当前企业成员" });
  expect(within(table).getAllByRole("columnheader")).toHaveLength(7);
  expect(within(table).getByText("管理员甲")).toBeVisible();
  expect(within(table).getAllByText("无已授予权限")).toHaveLength(3);
});
it("uses complete provider summary even when the directory page is incomplete", async () => {
  state.counts={total:21,active:20,administrators:3,inactive:1};
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json({
    ...result,
    total: 21,
    items: [{ ...result.items[0], roles: ["listingkit_admin"], state: "inactive" }],
  }))));
  render(page());
  const summary = await screen.findByRole("region", { name: "成员目录摘要" });
  expect(within(summary).queryByText("21", { exact: true })).not.toBeInTheDocument();
  expect(await within(summary).findByText("3", { exact: true })).toBeVisible();
  expect(within(summary).getByText("20", { exact: true })).toBeVisible();
});
it("starts a role edit with the member's current role", async () => {
  const data = {...result,canManage:true,assignableRoles:["listingkit_viewer","listingkit_operator"],items:[{...result.items[0],roles:["listingkit_operator"],canChangeRole:true}]};
  vi.stubGlobal("fetch",vi.fn().mockImplementation(()=>Promise.resolve(Response.json(data))));
  render(page());
  await userEvent.setup().click(await screen.findByRole("button",{name:"查看详情"}));
  expect(await screen.findByRole("combobox",{name:"新的成员角色"})).toHaveValue("listingkit_operator");
});
it.each([
  ["mutation",401,"AUTHENTICATION_REQUIRED"], ["mutation",403,"PERMISSION_DENIED"], ["mutation",409,"ORGANIZATION_CONTEXT_CHANGED"],
  ["receipt",401,"AUTHENTICATION_REQUIRED"], ["receipt",403,"PERMISSION_DENIED"], ["receipt",409,"ORGANIZATION_CONTEXT_CHANGED"],
  ["verify",401,"AUTHENTICATION_REQUIRED"], ["verify",403,"PERMISSION_DENIED"], ["verify",409,"ORGANIZATION_CONTEXT_CHANGED"],
])("quarantines stale directory after %s authority failure %s", async (route,status,code) => {
  const key="ea0390e6-6fd0-4834-8e9c-277caf59c122";
  let deny=false;
  const calls=vi.fn().mockImplementation((url,init)=> {
    const failure=()=>Response.json({code,message:"",requestId:"",fieldErrors:[]},{status:Number(status)});
    if(String(url).includes("member-operations")) {
      if(deny && (route==="receipt" || init.method==="POST")) return Promise.resolve(failure());
      return Promise.resolve(Response.json({code:"MEMBER_NOT_FOUND",message:"",requestId:"",fieldErrors:[]},{status:404}));
    }
    if(init.method==="POST") return Promise.resolve(failure());
    return Promise.resolve(Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer","listingkit_operator"],items:[{...result.items[0],canChangeRole:true}]}));
  });
  vi.stubGlobal("fetch", withAdditionalReads(calls));render(page());const user=userEvent.setup();
  expect(await screen.findByText("成员甲")).toBeVisible();
  if(route==="mutation") {
    await user.click(screen.getByRole("button",{name:"查看详情"}));
    await user.click(await screen.findByRole("button",{name:"保存角色"}));
  } else {
    await act(async()=>{
      deny=route==="receipt";
      sessionStorage.setItem('membership.pending:["actor","org"]',JSON.stringify({key,kind:"role",target:"grant",input:{role:"listingkit_operator",expectedVersion:"a".repeat(64)}}));
      window.dispatchEvent(new Event("membership-pending"));
    });
    if(route==="verify") {deny=true;await user.click(await screen.findByRole("button",{name:"核实原操作"}));}
  }
  await waitFor(()=>expect(screen.queryByText("成员甲")).not.toBeInTheDocument());
  expect(screen.queryByRole("button",{name:"邀请成员"})).not.toBeInTheDocument();
  expect(screen.getByRole("button",{name:"继续原操作"})).toBeDisabled();
  const retained=sessionStorage.getItem('membership.pending:["actor","org"]');expect(retained).not.toBeNull();
  const sends=calls.mock.calls.filter(([,init])=>init.method==="POST").length;
  deny=false;await user.click(screen.getByRole("button",{name:"刷新成员"}));
  expect(await screen.findByText("成员甲")).toBeVisible();
  expect(sessionStorage.getItem('membership.pending:["actor","org"]')).toBe(retained);
  expect(calls.mock.calls.filter(([,init])=>init.method==="POST")).toHaveLength(sends);
});
it("keeps a missing receipt pending until the original key yields a durable rejection", async () => {
  const key = "ea0390e6-6fd0-4834-8e9c-277caf59c122";
  sessionStorage.setItem('membership.pending:["actor","org"]',JSON.stringify({key,kind:"role",target:"grant",input:{role:"listingkit_operator",expectedVersion:"a".repeat(64)}}));
  const calls = vi.fn().mockImplementation((url) => Promise.resolve(
    String(url).endsWith("/verify") ? Response.json({schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id:key,kind:"role",step:"update_authorization",status:"rejected",targetUserId:"member",authorizationId:"grant",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null}) :
    String(url).includes("member-operations") ? Response.json({code:"MEMBER_NOT_FOUND",message:"",requestId:"",fieldErrors:[]},{status:404}) : Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer","listingkit_operator"]})
  ));
  vi.stubGlobal("fetch", withAdditionalReads(calls)); render(page());
  await waitFor(()=>expect(screen.getByRole("button",{name:"核实原操作"})).toBeEnabled());
  expect(screen.queryByRole("button",{name:"关闭回执"})).not.toBeInTheDocument();
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());
  await userEvent.setup().click(screen.getByRole("button",{name:"核实原操作"}));
  await userEvent.setup().click(await screen.findByRole("button",{name:"关闭回执"}));
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());
  expect(calls.mock.calls.filter(([,init])=>init.method==="POST").map(([url])=>String(url))).toEqual([`/api/account/member-operations/${key}/verify`]);
});
it.each([[401,"AUTHENTICATION_REQUIRED"],[403,"PERMISSION_DENIED"],[409,"ORGANIZATION_CONTEXT_CHANGED"]])("hides cached directory when detail authority fails %s", async(status,code)=> {
  vi.stubGlobal("fetch",vi.fn().mockImplementation(url=>Promise.resolve(String(url).endsWith("/grant") ? Response.json({code,message:"",requestId:"",fieldErrors:[]},{status:Number(status)}) : Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer"]}))));
  render(page());const user=userEvent.setup();
  await user.click(await screen.findByRole("button",{name:"查看详情"}));
  await waitFor(()=>expect(screen.queryByText("成员甲")).not.toBeInTheDocument());
  expect(screen.queryByRole("button",{name:"邀请成员"})).not.toBeInTheDocument();
});
it("does not let a directory refresh started before authority failure restore stale capabilities", async()=> {
  const key="ea0390e6-6fd0-4834-8e9c-277caf59c122";
  let finishReceipt!:(response:Response)=>void;
  let finishDirectory!:(response:Response)=>void;
  let reads=0;
  const directory={...result,canManage:true,assignableRoles:["listingkit_viewer"]};
  vi.stubGlobal("fetch",vi.fn().mockImplementation(url=> {
    if(String(url).includes("member-operations")) return new Promise<Response>(resolve=>{finishReceipt=resolve;});
    if(++reads===2) return new Promise<Response>(resolve=>{finishDirectory=resolve;});
    return Promise.resolve(Response.json(directory));
  }));
  render(page());const user=userEvent.setup();expect(await screen.findByText("成员甲")).toBeVisible();
  await act(async()=>{sessionStorage.setItem('membership.pending:["actor","org"]',JSON.stringify({key,kind:"role",target:"grant",input:{role:"listingkit_viewer",expectedVersion:"a".repeat(64)}}));window.dispatchEvent(new Event("membership-pending"));});
  await waitFor(()=>expect(finishReceipt).toBeDefined());
  await user.click(screen.getByRole("button",{name:"刷新成员"}));
  await waitFor(()=>expect(finishDirectory).toBeDefined());
  await act(async()=>finishReceipt(Response.json({code:"PERMISSION_DENIED",message:"",requestId:"",fieldErrors:[]},{status:403})));
  await act(async()=>finishDirectory(Response.json(directory)));
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument();
  expect(screen.getByRole("button",{name:"继续原操作"})).toBeDisabled();
  await user.click(screen.getByRole("button",{name:"刷新成员"}));
  expect(await screen.findByText("成员甲")).toBeVisible();
});
it("clears the member directory immediately during an org switch", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json(result))));
  const tree = page(); const view = render(tree); expect(await screen.findByText("成员甲")).toBeVisible();
  state.switching = true; view.rerender(page());
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument();
});
it("sends an invitation after StrictMode effect replay and keeps original-key recovery usable", async () => {
  const calls = vi.fn().mockImplementation((_url, init) => Promise.resolve(init.method === "POST" ? Response.json({code:"DEPENDENCY_UNAVAILABLE",message:"",requestId:"",fieldErrors:[]},{status:503}) : Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer"]})));
  vi.stubGlobal("fetch", withAdditionalReads(calls));
  render(<StrictMode>{page()}</StrictMode>);
  const user = userEvent.setup();
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());await user.click(screen.getByRole("button",{name:"邀请成员"}));
  await user.type(screen.getByRole("textbox",{name:"受邀邮箱"}),"fixture@example.com");


  await user.click(screen.getByRole("button",{name:"发送邀请邮件"}));
  await waitFor(() => expect(calls.mock.calls.some(([,init]) => init.method === "POST")).toBe(true));
  expect(await screen.findByRole("button",{name:"核实原邀请"})).toBeEnabled();
});
it("cancels and isolates an old response on the same query client across organizations", async () => {
  let finish!: (value: Response) => void;
  let oldSignal: AbortSignal | undefined;
  vi.stubGlobal("fetch",vi.fn().mockImplementation((_url,init) => {
    if(init.headers.get("X-Expected-Organization-ID") === "org") { oldSignal=init.signal; return new Promise<Response>(resolve => { finish=resolve; }); }
    return Promise.resolve(Response.json({...result,organizationId:"new-org",items:[{...result.items[0],organizationId:"new-org",displayName:"新企业成员"}]}));
  }));
  const client=new QueryClient(); const view=render(page(client));
  await waitFor(()=>expect(oldSignal).toBeDefined());
  state.org="new-org"; view.rerender(page(client));
  expect(await screen.findByText("新企业成员")).toBeVisible(); expect(oldSignal?.aborted).toBe(true);
  await act(async()=>finish(Response.json(result)));
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument(); expect(screen.getByText("新企业成员")).toBeVisible();
});
it("has named, valid accessible controls in the member table and invitation form", async () => {
  vi.stubGlobal("fetch",vi.fn().mockImplementation(()=>Promise.resolve(Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer"]}))));
  render(<main>{page()}</main>);
  await userEvent.setup().click(await screen.findByRole("button",{name:"邀请成员"}));
  // jsdom has no rendered contrast/layout; those remain browser visual checks.
  const report=await axe.run(document.body,{rules:{"color-contrast":{enabled:false}}});
  expect(report.violations).toEqual([]);
});
