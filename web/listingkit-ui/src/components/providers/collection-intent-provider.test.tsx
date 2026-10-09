import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { WorkbenchContextProvider, useWorkbenchContext } from "./workbench-context-provider";
import type { CollectionIntent } from "@/lib/api/product-collection";

const key = "550e8400-e29b-41d4-a716-446655440000";
const intent: CollectionIntent = { userId: "actor-a", organizationId: "org-a", key,
  command: { action: "import_products", name: "保留的导入", products: [{ title: "商品", description: "原文", images: [] }] } };
function Probe() {
  const context = useWorkbenchContext();
  return <><button onClick={() => context.setPendingCollectionIntent(intent)}>retain</button>
    <output>{context.pendingCollectionIntent ? JSON.stringify(context.pendingCollectionIntent) : "empty"}</output></>;
}
function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><WorkbenchContextProvider><Probe /></WorkbenchContextProvider></QueryClientProvider>);
}
afterEach(() => { cleanup(); localStorage.clear(); sessionStorage.clear(); vi.unstubAllGlobals(); });
it("hydrates the original scoped import key and payload after the tab is closed", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ user: { id: "actor-a" }, homeOrganizationId: "org-a", effectiveOrganizationId: "org-a", selectionRequired: false,
    organizations: [{ id: "org-a", name: "试用企业", roles: [], permissions: [] }] })));
  const first = mount();
  await userEvent.click(screen.getByRole("button", { name: "retain" }));
  await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(key));
  first.unmount(); sessionStorage.clear();
  mount();
  await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(JSON.stringify(intent)));
});
