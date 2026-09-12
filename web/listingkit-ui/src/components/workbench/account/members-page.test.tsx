import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { MembersPage } from "./members-page";

const state = vi.hoisted(() => ({ switching: false }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "actor" }, effectiveOrganization: { id: "org", name: "当前企业" }, roles: ["listingkit_admin"], isSwitching: state.switching, isLoading: false, selectionRequired: false, error: null, blockingError: null }) }));
const result = { schemaVersion: "membership-v1", userId: "actor", organizationId: "org", canManage: false, assignableRoles: [], total: 1, items: [{ id: "grant", userId: "member", projectId: "project", organizationId: "org", displayName: "成员甲", loginName: "a@example.com", roles: ["listingkit_viewer"], state: "active", createdAt: "2026-09-12T00:00:00Z", changedAt: "2026-09-12T00:00:00Z", observedVersion: "a".repeat(64), canChangeRole: false, canRemove: false }] };
const clients: QueryClient[] = [];
afterEach(() => { cleanup(); clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); state.switching = false; sessionStorage.clear(); });
function page() { const client = new QueryClient(); clients.push(client); return <QueryClientProvider client={client}><MembersPage expectedUserId="actor" /></QueryClientProvider>; }
it("uses backend capability instead of context role names", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(Response.json(result))));
  render(page()); expect(await screen.findByText("成员甲")).toBeVisible();
  expect(screen.queryByRole("button", { name: "邀请成员" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "移除成员" })).not.toBeInTheDocument();
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
