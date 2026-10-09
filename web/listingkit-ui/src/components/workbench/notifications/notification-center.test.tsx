import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { NotificationCenterPage, NotificationDetailPage } from "./notification-center";
const fixture = vi.hoisted(() => ({ context: { user: { id: "actor" } as { id: string } | null, effectiveOrganization: { id: "org-a" } as { id: string } | null, selectionRequired: false, isLoading: false, isSwitching: false, error: null as { code: string } | null, blockingError: null as { code: string } | null } }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => fixture.context }));
const id = "4841d296-ef14-4c16-8d25-a7667e534feb", snapshotID = "4f5e1184-83c7-45b0-9dd9-c61242c9fb25";
function item(category: "official" | "business" = "official", organizationId = "") {
  const source = category === "official" ? "official" : "workbench-task", type = category === "official" ? "PRODUCT" : "pending";
  const ref = btoa(JSON.stringify({ source, entityId: id, type, revision: "1", organizationId })).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  return { id: ref, category, type, title: category === "official" ? "产品能力更新" : "任务需要确认", summary: "打开原业务页面查看当前状态", paragraphs: [], occurredAt: "2026-10-08T02:00:00Z", source, organizationId, attention: category === "business", read: false, target: { kind: "home", id: "" }, href: "/workbench" };
}
function list(category: "official" | "business" = "official", organization = "") { return { schemaVersion: "notification-center-v1", items: [item(category, organization)], coverage: [{ source: category === "official" ? "official" : "workbench-task", state: "AVAILABLE", complete: true, observedAt: "2026-10-09T00:00:00Z" }], exact: true, count: 121, unread: 121, pending: category === "business" ? 121 : 0, next: "" }; }
function receipt(key: string, operation: string) { return Response.json({ key, operation, committedAt: "2026-10-09T00:00:00Z" }); }
const clients: QueryClient[] = [];
function mount(child: React.ReactNode) { const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }); clients.push(client); return render(<QueryClientProvider client={client}>{child}</QueryClientProvider>); }
beforeEach(() => { fixture.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-a" }, selectionRequired: false, isLoading: false, isSwitching: false, error: null, blockingError: null }; });
afterEach(() => { cleanup(); clients.splice(0).forEach(c => c.clear()); vi.unstubAllGlobals(); });
it("keeps official and personal messages available when the organization read fails", async () => {
  fixture.context.error = { code: "DEPENDENCY_UNAVAILABLE" }; fixture.context.user = null;
  const fetch = vi.fn().mockImplementation((url: string) => Promise.resolve(Response.json(url.includes("/official") ? list() : { ...list("business"), items: [] })));
  vi.stubGlobal("fetch", fetch); mount(<NotificationCenterPage expectedUserId="actor" />);
  expect(await screen.findByText("产品能力更新")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("tab", { name: /商家经营/ }));
  expect(await screen.findByText(/当前仅显示个人提醒/)).toBeInTheDocument();
  await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).includes("/personal?"))).toBe(true));
  expect(fetch.mock.calls.some(([url]) => String(url).includes("/business?"))).toBe(false);
});
it("all-read uses the complete category snapshot even under unread filtering", async () => {
  const fetch = vi.fn().mockImplementation((url: string, init?: RequestInit) => Promise.resolve(url.endsWith("/snapshot") ? Response.json({ id: snapshotID, fingerprint: "a".repeat(64), expiresAt: "2026-10-09T00:05:00Z", count: 121 }) : url.endsWith("/read-all") ? receipt(new Headers(init?.headers).get("Idempotency-Key")!, "read-all") : Response.json(list())));
  vi.stubGlobal("fetch", fetch); mount(<NotificationCenterPage expectedUserId="actor" />);
  await screen.findByText("产品能力更新"); fireEvent.click(screen.getByRole("checkbox", { name: /仅看未读/ }));
  await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).includes("filter=unread"))).toBe(true));
  await waitFor(() => expect(screen.getByRole("button", { name: "全部已读" })).toBeEnabled());
  fireEvent.click(screen.getByRole("button", { name: "全部已读" }));
  await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).endsWith("/read-all"))).toBe(true));
  const snapshot = fetch.mock.calls.find(([url]) => String(url).endsWith("/snapshot"))!;
  const submit = fetch.mock.calls.find(([url]) => String(url).endsWith("/read-all"))!;
  expect(snapshot[1].body).toBe("{}"); expect(JSON.parse(submit[1].body)).toEqual({ id: snapshotID, fingerprint: "a".repeat(64) });
});
it("keeps the original snapshot key after response loss and verifies before continuing", async () => {
  let snapshots = 0;
  const fetch = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (url.endsWith("/snapshot")) { snapshots++; return Promise.resolve(snapshots === 1 ? Response.json({ code: "OUTCOME_UNKNOWN" }, { status: 503 }) : Response.json({ id: snapshotID, fingerprint: "a".repeat(64), expiresAt: "2026-10-09T00:05:00Z", count: 121 })); }
    if (url.includes("/commands/")) return Promise.resolve(receipt(url.split("/").at(-1)!, "snapshot"));
    if (url.endsWith("/read-all")) return Promise.resolve(receipt(new Headers(init?.headers).get("Idempotency-Key")!, "read-all"));
    return Promise.resolve(Response.json(list()));
  });
  vi.stubGlobal("fetch", fetch); mount(<NotificationCenterPage expectedUserId="actor" />);
  await screen.findByText("产品能力更新"); fireEvent.click(screen.getByRole("button", { name: "全部已读" }));
  fireEvent.click(await screen.findByRole("button", { name: "核实原操作" }));
  await waitFor(() => expect(snapshots).toBe(2));
  const calls = fetch.mock.calls.filter(([url]) => String(url).endsWith("/snapshot"));
  expect(new Headers(calls[0][1].headers).get("Idempotency-Key")).toBe(new Headers(calls[1][1].headers).get("Idempotency-Key"));
  expect(fetch.mock.calls.filter(([url]) => String(url).endsWith("/read-all"))).toHaveLength(1);
});
it("hides previous company messages while switching context and reads the new company", async () => {
  const fetch = vi.fn().mockImplementation((url: string, init?: RequestInit) => Promise.resolve(Response.json(url.includes("/official") ? list() : list("business", new Headers(init?.headers).get("X-Expected-Organization-ID")!))));
  vi.stubGlobal("fetch", fetch); const rendered = mount(<NotificationCenterPage expectedUserId="actor" />);
  await screen.findByText("产品能力更新"); fireEvent.click(screen.getByRole("tab", { name: /商家经营/ })); await screen.findByText("任务需要确认");
  fixture.context.isSwitching = true; rendered.rerender(<QueryClientProvider client={clients[0]}><NotificationCenterPage expectedUserId="actor" /></QueryClientProvider>);
  expect(screen.queryByText("任务需要确认")).not.toBeInTheDocument();
  fixture.context.isSwitching = false; fixture.context.effectiveOrganization = { id: "org-b" };
  rendered.rerender(<QueryClientProvider client={clients[0]}><NotificationCenterPage expectedUserId="actor" /></QueryClientProvider>);
  await screen.findByText("任务需要确认"); expect(new Headers(fetch.mock.calls.at(-1)![1].headers).get("X-Expected-Organization-ID")).toBe("org-b");
});
it("opening detail only records reading, keeping pending business attention", async () => {
  const value = { ...item("business", "org-a"), paragraphs: ["请在原任务页面确认。"] };
  const fetch = vi.fn().mockImplementation((url: string, init?: RequestInit) => Promise.resolve(url.endsWith("/read") ? receipt(new Headers(init?.headers).get("Idempotency-Key")!, "read") : Response.json(value)));
  vi.stubGlobal("fetch", fetch); mount(<NotificationDetailPage expectedUserId="actor" category="business" notificationRef={value.id} />);
  await screen.findByText("仍待处理"); await waitFor(() => expect(fetch.mock.calls.filter(([url]) => String(url).endsWith("/read"))).toHaveLength(1));
  expect(fetch.mock.calls.every(([url]) => String(url).startsWith("/api/notifications/"))).toBe(true);
  expect(screen.getByRole("link", { name: "查看相关业务" })).toHaveAttribute("href", "/workbench");
});
