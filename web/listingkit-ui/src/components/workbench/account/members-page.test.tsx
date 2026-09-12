import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { createRequire } from "node:module";
import { afterEach, expect, it, vi } from "vitest";
import { MembersPage } from "./members-page";

// Reuse the axe engine already installed by the repository's browser test dependency.
const load = createRequire(import.meta.url);
const axe = load(load.resolve("axe-core", {paths:[load.resolve("@axe-core/playwright")]})) as {run:(node:HTMLElement,options:unknown)=>Promise<{violations:unknown[]}>};

const state = vi.hoisted(() => ({ switching: false, org: "org" }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "actor" }, effectiveOrganization: { id: state.org, name: "当前企业" }, roles: ["listingkit_admin"], isSwitching: state.switching, isLoading: false, selectionRequired: false, error: null, blockingError: null }) }));
const result = { schemaVersion: "membership-v1", userId: "actor", organizationId: "org", canManage: false, assignableRoles: [], total: 1, items: [{ id: "grant", userId: "member", projectId: "project", organizationId: "org", displayName: "成员甲", loginName: "a@example.com", roles: ["listingkit_viewer"], state: "active", createdAt: "2026-09-12T00:00:00Z", changedAt: "2026-09-12T00:00:00Z", observedVersion: "a".repeat(64), canChangeRole: false, canRemove: false }] };
const clients: QueryClient[] = [];
afterEach(() => { cleanup(); clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); state.switching = false; state.org = "org"; sessionStorage.clear(); });
function page(client = new QueryClient()) { clients.push(client); return <QueryClientProvider client={client}><MembersPage expectedUserId="actor" /></QueryClientProvider>; }
it("uses backend capability instead of context role names", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json(result))));
  render(page()); expect(await screen.findByText("成员甲")).toBeVisible();
  expect(screen.queryByRole("button", { name: "邀请成员" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "移除成员" })).not.toBeInTheDocument();
});
it("starts a role edit with the member's current role", async () => {
  const data = {...result,canManage:true,assignableRoles:["listingkit_viewer","listingkit_operator"],items:[{...result.items[0],roles:["listingkit_operator"],canChangeRole:true}]};
  vi.stubGlobal("fetch",vi.fn().mockImplementation(()=>Promise.resolve(Response.json(data))));
  render(page());
  await userEvent.setup().click(await screen.findByRole("button",{name:"查看详情"}));
  expect(await screen.findByRole("combobox",{name:"新的成员角色"})).toHaveValue("listingkit_operator");
});
it("keeps a missing receipt pending until the original key yields a durable rejection", async () => {
  const key = "ea0390e6-6fd0-4834-8e9c-277caf59c122";
  sessionStorage.setItem('membership.pending:["actor","org"]',JSON.stringify({key,kind:"role",target:"grant",input:{role:"listingkit_operator",expectedVersion:"a".repeat(64)}}));
  const calls = vi.fn().mockImplementation((url) => Promise.resolve(
    String(url).endsWith("/verify") ? Response.json({schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id:key,kind:"role",step:"update_authorization",status:"rejected",targetUserId:"member",authorizationId:"grant",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null}) :
    String(url).includes("member-operations") ? Response.json({code:"MEMBER_NOT_FOUND",message:"",requestId:"",fieldErrors:[]},{status:404}) : Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer","listingkit_operator"]})
  ));
  vi.stubGlobal("fetch",calls); render(page());
  await waitFor(()=>expect(screen.getByRole("button",{name:"核实原操作"})).toBeEnabled());
  expect(screen.queryByRole("button",{name:"关闭回执"})).not.toBeInTheDocument();
  expect(screen.getByRole("button",{name:"邀请成员"})).toBeDisabled();
  await userEvent.setup().click(screen.getByRole("button",{name:"核实原操作"}));
  await userEvent.setup().click(await screen.findByRole("button",{name:"关闭回执"}));
  await waitFor(()=>expect(screen.getByRole("button",{name:"邀请成员"})).toBeEnabled());
  expect(calls.mock.calls.filter(([,init])=>init.method==="POST").map(([url])=>String(url))).toEqual([`/api/account/member-operations/${key}/verify`]);
});
it("clears the member directory immediately during an org switch", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json(result))));
  const tree = page(); const view = render(tree); expect(await screen.findByText("成员甲")).toBeVisible();
  state.switching = true; view.rerender(page());
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument();
});
it("sends an invitation after StrictMode effect replay and keeps original-key recovery usable", async () => {
  const calls = vi.fn().mockImplementation((_url, init) => Promise.resolve(init.method === "POST" ? Response.json({code:"DEPENDENCY_UNAVAILABLE",message:"",requestId:"",fieldErrors:[]},{status:503}) : Response.json({...result,canManage:true,assignableRoles:["listingkit_viewer"]})));
  vi.stubGlobal("fetch", calls);
  render(<StrictMode>{page()}</StrictMode>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button",{name:"邀请成员"}));
  await user.type(screen.getByRole("textbox",{name:"邮箱"}),"fixture@example.com");
  await user.type(screen.getByRole("textbox",{name:"名字"}),"Test");
  await user.type(screen.getByRole("textbox",{name:"姓氏"}),"Member");
  await user.click(screen.getByRole("button",{name:"确认邀请"}));
  await waitFor(() => expect(calls.mock.calls.some(([,init]) => init.method === "POST")).toBe(true));
  expect(await screen.findByRole("button",{name:"继续原操作"})).toBeEnabled();
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
