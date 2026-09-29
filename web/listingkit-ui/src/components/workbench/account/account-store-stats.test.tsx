import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { AccountOrganization } from "@/lib/api/account";
import { OrganizationView } from "./account-views";

const organization: AccountOrganization = {
  schemaVersion: "account-v1", userId: "self", homeOrganizationId: "A",
  effectiveOrganizationId: "B", name: "企业乙", roles: ["viewer"],
  source: "zitadel_project_authorizations", readAt: "2026-09-28T00:00:00Z",
  authorizationMaxAgeSeconds: 60,
};
const clients: QueryClient[] = [];
afterEach(() => {
  cleanup(); clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals();
});
function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  return render(<QueryClientProvider client={client}><OrganizationView data={organization} /></QueryClientProvider>);
}
function unavailable() {
  return Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 });
}

it("shows the full Store owner total through the current nonempty DTO instead of the one returned item", async () => {
  const fetcher = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).startsWith("/api/workbench/stores?")) {
      return Promise.resolve(Response.json({
        items: [{
          id: "11111111-1111-4111-8111-111111111111", name: "真实目录记录",
          platform: "shein", region: "SG", externalStoreId: "",
          recordStatus: "active", serviceStatus: "pending_activation", serviceStartedAt: null,
          serviceExpiresAt: null, connectionStatus: "unavailable", version: 1,
          createdAt: "2026-09-28T00:00:00Z", updatedAt: "2026-09-28T00:00:00Z",
        }],
        pagination: { page: 1, pageSize: 1, total: 37 },
      }));
    }
    void init;
    return Promise.resolve(unavailable());
  });
  vi.stubGlobal("fetch", fetcher);
  mount();
  const metric = screen.getByText("已绑定店铺").closest("article")!;
  expect(await within(metric).findByText("37")).toBeVisible();
  expect(within(metric).queryByText("1")).not.toBeInTheDocument();
  const [url, init] = fetcher.mock.calls.find(([input]) => String(input).startsWith("/api/workbench/stores?"))!;
  expect(url).toBe("/api/workbench/stores?page=1&pageSize=1");
  expect(new Headers(init?.headers).get("X-Expected-Organization-ID")).toBe("B");
  expect(init?.signal).toBeInstanceOf(AbortSignal);
  expect(init).toMatchObject({ method: "GET", cache: "no-store", redirect: "error" });
});

it("keeps an unavailable Store owner explicit instead of displaying a false zero", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => Promise.resolve(unavailable())));
  mount();
  const metric = screen.getByText("已绑定店铺").closest("article")!;
  expect(await within(metric).findByText("暂不可用")).toBeVisible();
  expect(within(metric).queryByText("0")).not.toBeInTheDocument();
});
