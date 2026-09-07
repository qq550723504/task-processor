import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { OrganizationSwitcher } from "@/components/workbench/organization-switcher";
import { parseCommercialOverview } from "@/lib/api/commercial";
import { WORKBENCH_CONTEXT_QUERY_KEY } from "@/lib/api/workbench-context";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { CommercialPage } from "./commercial-page";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("uses real context/switcher and commercial client, drops late HTTP response, and clears on role refresh", async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let context = { user: { id: "reader" }, homeOrganizationId: "org-A", effectiveOrganizationId: "org-B", selectionRequired: false, organizations: [{ id: "org-B", name: "企业乙", roles: ["listingkit_admin"] }, { id: "org-C", name: "企业丙", roles: ["listingkit_admin"] }] };
  let release!: (response: Response) => void;
  const late = new Promise<Response>(resolve => { release = resolve; });
  const requests: RequestInit[] = [];
  let revoked = false;
  vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
    if (url === "/api/workbench/context") return Response.json(context);
    if (url === "/api/workbench/context/effective-organization") {
      context = { ...context, effectiveOrganizationId: "org-C" }; return Response.json(context);
    }
    expect(url).toBe("/api/workbench/commercial/overview");
    requests.push(init!);
    if (revoked) return Response.json({ code: "PERMISSION_DENIED", message: "private", requestId: "test-request", fieldErrors: [] }, { status: 403 });
    if (new Headers(init?.headers).get("X-Expected-Organization-ID") === "org-B") return late;
    return Response.json({ ...commercialOverviewFixture("org-C"), subscription: null });
  }));
  const view = render(<QueryClientProvider client={queryClient}><WorkbenchContextProvider><OrganizationSwitcher /><CommercialPage page="entitlements" /></WorkbenchContextProvider></QueryClientProvider>);
  await waitFor(() => expect(requests).toHaveLength(1));
  await userEvent.selectOptions(screen.getByLabelText("当前企业"), "org-C");
  expect(await screen.findByText("无订阅")).toBeVisible();
  expect(requests[0].signal?.aborted).toBe(true);
  expect(new Headers(requests[1].headers).get("X-Expected-Organization-ID")).toBe("org-C");
  await act(async () => release(Response.json(commercialOverviewFixture())));
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  revoked = true;
  context = { ...context, organizations: context.organizations.map(org => ({ ...org, roles: ["listingkit_viewer"] })) };
  await act(async () => { await queryClient.invalidateQueries({ queryKey: WORKBENCH_CONTEXT_QUERY_KEY }); });
  expect(await screen.findByRole("alert")).toHaveTextContent("无查看权限");
  expect(screen.queryByText("无订阅")).not.toBeInTheDocument();
  expect(screen.queryByText("private")).not.toBeInTheDocument();
  expect(requests.every(request => request.method === "GET" && request.cache === "no-store" && request.redirect === "manual")).toBe(true);
  view.unmount(); queryClient.clear();
});

it("validates component fixtures using the single actual wire parser and reauthorizes after cached context drift", async () => {
  expect(parseCommercialOverview(commercialOverviewFixture())).not.toBeNull();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const context = { user: { id: "reader" }, homeOrganizationId: "org-B", effectiveOrganizationId: "org-B", selectionRequired: false, organizations: [{ id: "org-B", name: "企业乙", roles: ["listingkit_admin"] }] };
  const fetchMock = vi.fn(async (url: string) => url === "/api/workbench/context" ? Response.json(context) : Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "hidden", requestId: "test-request", fieldErrors: [] }, { status: 403 }));
  vi.stubGlobal("fetch", fetchMock);
  queryClient.setQueryData(WORKBENCH_CONTEXT_QUERY_KEY, context);
  const view = render(<QueryClientProvider client={queryClient}><WorkbenchContextProvider><CommercialPage page="options" /></WorkbenchContextProvider></QueryClientProvider>);
  expect(await screen.findByRole("alert")).toHaveTextContent("企业访问已撤销");
  expect(screen.queryByText("基础方案 · 按需使用")).not.toBeInTheDocument();
  expect(fetchMock.mock.calls.some(([url]) => url === "/api/workbench/commercial/overview")).toBe(true);
  view.unmount(); queryClient.clear();
});
