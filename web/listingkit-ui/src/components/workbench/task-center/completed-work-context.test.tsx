import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { OrganizationSwitcher } from "@/components/workbench/organization-switcher";
import { completedWorkFixture } from "@/test/fixtures/completed-work";
import { CompletedWorkPageContent } from "./completed-work-page";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("actual provider/switcher and #340 client isolate an old organization HTTP response", async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const context = { user: { id: "reader" }, homeOrganizationId: "200", effectiveOrganizationId: "200", selectionRequired: false, organizations: [{ id: "200", name: "企业甲", roles: [] }, { id: "100", name: "企业乙", roles: [] }] };
  let release!: (response: Response) => void;
  const late = new Promise<Response>((resolve) => { release = resolve; });
  const requests: RequestInit[] = [];
  vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
    if (url === "/api/workbench/context") return Response.json(context);
    if (url === "/api/workbench/context/effective-organization") return Response.json({ ...context, effectiveOrganizationId: "100" });
    requests.push(init!);
    return new Headers(init?.headers).get("X-Expected-Organization-ID") === "200" ? late : Response.json({ ...completedWorkFixture(), items: [] });
  }));
  const view = render(<QueryClientProvider client={client}><WorkbenchContextProvider><OrganizationSwitcher /><CompletedWorkPageContent available /></WorkbenchContextProvider></QueryClientProvider>);
  await waitFor(() => expect(requests).toHaveLength(1));
  await userEvent.selectOptions(screen.getByLabelText("当前企业"), "100");
  expect(await screen.findByText("当前授权范围内暂无本地资料准备记录")).toBeVisible();
  expect(requests[0].signal?.aborted).toBe(true);
  expect(new Headers(requests[1].headers).get("X-Expected-Organization-ID")).toBe("100");
  await act(async () => release(Response.json(completedWorkFixture())));
  expect(screen.queryByText("synthetic-product-1")).not.toBeInTheDocument();
  expect(screen.getByLabelText("当前企业")).toHaveValue("100");
  view.unmount(); client.clear();
});
