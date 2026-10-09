import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StoreOfficialConnection } from "./store-official-connection";
import type { WorkbenchStore } from "@/lib/api/workbench-stores";
const state = vi.hoisted(() => ({
  read: vi.fn(),
  begin: vi.fn(),
	remember: vi.fn(),
	applications:vi.fn(),
}));
vi.mock("@/lib/api/store-connection", async (original) => ({
  ...(await original<typeof import("@/lib/api/store-connection")>()),
  getStoreConnection: state.read,
  beginStoreConnection: state.begin,
	rememberStoreAuthorization: state.remember,
	getStoreApplications:state.applications,
}));
vi.mock("./store-service-actions", () => ({ StoreServiceActions: () => null }));
const store = {
  id: "11111111-1111-4111-8111-11111111111a",
  version: 2,
  recordStatus: "active",
} as WorkbenchStore;
const client = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});
function tree(org = "org-1") {
  return (
    <QueryClientProvider client={client}>
      <StoreOfficialConnection
        store={store}
        scope={{ expectedUserId: "actor", expectedOrganizationId: org }}
        canWrite
        administrator
        onChanged={vi.fn()}
      />
    </QueryClientProvider>
  );
}
beforeEach(() => {
  client.clear();
  state.remember.mockReset();
	state.begin.mockReset();
	state.applications.mockResolvedValue([{appId:"self-app",revision:"v1~self_operated",type:"self_operated"}]);
  state.read.mockResolvedValue({
    connectionStatus: "disconnected",
    attemptId: null,
  });
});
afterEach(cleanup);
it.each(["switch", "unmount"])(
  "does not persist or navigate a late begin result after %s",
  async (mode) => {
    let resolve!: (value: unknown) => void;
    state.begin.mockImplementationOnce(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
    const view = render(tree());
    await screen.findByText("未连接");
    await userEvent.click(screen.getByRole("button", { name: "前往官方授权" }));
    if (mode === "switch") view.rerender(tree("org-2"));
    else view.unmount();
    await act(async () => {
      resolve({
        attemptId: "22222222-2222-4222-8222-22222222222b",
        expiresAt: "2026-09-29T00:00:00Z",
        authorizationUrl: "https://open.sheincorp.com/authorization",
      });
    });
    expect(state.remember).not.toHaveBeenCalled();
  },
);
it("requires a configured application choice and sends the chosen managed application",async()=>{
	state.applications.mockResolvedValue([
		{appId:"self-app",revision:"v1~self_operated",type:"self_operated"},
		{appId:"semi-app",revision:"v1~semi_managed",type:"semi_managed"},
		{appId:"full-app",revision:"v1~fully_managed",type:"fully_managed"},
	]);
	state.begin.mockRejectedValue(new Error("controlled fixture"));
	render(tree());
	const choices=await screen.findByRole("combobox",{name:"官方应用类型"});
	await screen.findByRole("option",{name:"全托管 · full-app"});
	expect(screen.getByRole("button",{name:"前往官方授权"})).toBeDisabled();
	await userEvent.selectOptions(choices,"full-app");
	await userEvent.click(screen.getByRole("button",{name:"前往官方授权"}));
	expect(state.begin).toHaveBeenCalledWith({expectedUserId:"actor",expectedOrganizationId:"org-1"},store.id,store.version,expect.any(String),"full-app");
});
