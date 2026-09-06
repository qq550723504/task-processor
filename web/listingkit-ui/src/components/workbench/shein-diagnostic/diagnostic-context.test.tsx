import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { OrganizationSwitcher } from "@/components/workbench/organization-switcher";
import { diagnosticFixture } from "@/test/fixtures/shein-diagnostic";
import { SheinDiagnosticPage } from "./diagnostic-page";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it.each([true, false])("retries diagnostics only after same-scope context recovery succeeds=%s", async (succeeds) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const context = {
    user: { id: "reader" }, homeOrganizationId: "200", effectiveOrganizationId: "200", selectionRequired: false,
    organizations: [{ id: "200", name: "企业甲", roles: [] }],
  };
  let release!: (response: Response) => void;
  const recovery = new Promise<Response>((resolve) => { release = resolve; });
  let contextReads = 0;
  let diagnosticReads = 0;
  vi.stubGlobal("fetch", vi.fn(async (url: string) => {
    if (url === "/api/workbench/context") return ++contextReads === 1 ? Response.json(context) : recovery;
    return ++diagnosticReads === 1
      ? Response.json({ code: "ORGANIZATION_SUSPENDED", message: "Suspended", requestId: "test", fieldErrors: [] }, { status: 403 })
      : Response.json(diagnosticFixture());
  }));
  const view = render(<QueryClientProvider client={client}><WorkbenchContextProvider><SheinDiagnosticPage recordId="c78285de-a1a8-4cc5-8aae-93ad6851da11" /></WorkbenchContextProvider></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
  expect(contextReads).toBe(2);
  expect(diagnosticReads).toBe(1);
  await act(async () => release(succeeds ? Response.json(context) : Response.json({ code: "ORGANIZATION_SUSPENDED", message: "Suspended", requestId: "test", fieldErrors: [] }, { status: 403 })));
  if (succeeds) {
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    expect(diagnosticReads).toBe(2);
  } else {
    expect(await screen.findByRole("status")).toHaveTextContent("企业或登录上下文不可用");
    expect(diagnosticReads).toBe(1);
  }
  view.unmount();
  client.clear();
});

it("actual enterprise switcher/provider and diagnostic client discard the old tenant HTTP response (synthetic HTTP fixture)", async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const context = {
    user: { id: "reader" }, homeOrganizationId: "200", effectiveOrganizationId: "200", selectionRequired: false,
    organizations: [{ id: "200", name: "企业甲", roles: [] }, { id: "100", name: "企业乙", roles: [] }],
  };
  let releaseOld!: (response: Response) => void;
  const old = new Promise<Response>((resolve) => { releaseOld = resolve; });
  const requests: RequestInit[] = [];
  const fetcher = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
    if (url === "/api/workbench/context") return Response.json(context);
    if (url === "/api/workbench/context/effective-organization") {
      expect(init?.method).toBe("PUT");
      expect(JSON.parse(String(init?.body))).toEqual({ organizationId: "100" });
      return Response.json({ ...context, effectiveOrganizationId: "100" });
    }
    requests.push(init!);
    if (new Headers(init?.headers).get("X-Expected-Organization-ID") === "200") return old;
    return Response.json(diagnosticFixture());
  });
  vi.stubGlobal("fetch", fetcher);
  const view = render(
    <QueryClientProvider client={client}>
      <WorkbenchContextProvider>
        <OrganizationSwitcher />
        <SheinDiagnosticPage recordId="c78285de-a1a8-4cc5-8aae-93ad6851da11" />
      </WorkbenchContextProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(requests).toHaveLength(1));
  await userEvent.selectOptions(screen.getByLabelText("当前企业"), "100");
  expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
  expect(requests[0].signal?.aborted).toBe(true);
  expect(new Headers(requests[1].headers).get("X-Expected-Organization-ID")).toBe("100");
  const late = diagnosticFixture();
  late.offline_checks.blockers[0].message = "旧企业私有内容";
  await act(async () => releaseOld(Response.json(late)));
  expect(screen.queryByText("旧企业私有内容")).not.toBeInTheDocument();
  expect(screen.getByLabelText("当前企业")).toHaveValue("100");
  view.unmount();
  client.clear();
});
