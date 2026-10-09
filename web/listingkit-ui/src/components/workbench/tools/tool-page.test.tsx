import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ToolPage } from "./tool-page";
const context = vi.hoisted(() => ({
  user: { id: "actor" },
  effectiveOrganization: { id: "org-a", name: "A企业" },
  permissions: [
    "workbench.tools.read",
    "workbench.tools.manage",
    "workbench.tools.customize",
  ],
  isLoading: false,
  isSwitching: false,
  selectionRequired: false,
  error: null,
  blockingError: null,
  retry: vi.fn(),
  registerOrganizationSwitchGuard: () => () => {},
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => context,
}));
vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
const tool = {
  id: "product-acquisition",
  version: "0.1.0",
  name: "商品采集插件",
  category: "采集",
  description: "采集1688商品资料",
  status: "AVAILABLE",
  reason: "",
  activation: null,
  localCapture: true,
  onlineCapture: true,
  download: true,
};
beforeEach(() => {
  sessionStorage.clear();
  vi.unstubAllGlobals();
  context.effectiveOrganization = { id: "org-a", name: "A企业" };
});
const mount = (mode: "official" | "mine" | "custom") =>
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ToolPage mode={mode} />
    </QueryClientProvider>,
  );
it("enables a real enterprise choice and rereads server state", async () => {
  let enabled = false;
  const fetch = vi.fn(async (_url: string, init?: RequestInit) => {
    if (init?.method === "PUT") {
      enabled = true;
      const h = new Headers(init.headers);
      return Response.json({
        commandId: h.get("Idempotency-Key"),
        operation: "activation",
        id: tool.id,
        revision: "1",
        committedAt: "2026-10-09T00:00:00Z",
      });
    }
    return Response.json({
      tools: [
        {
          ...tool,
          activation: enabled
            ? {
                toolId: tool.id,
                enabled: true,
                revision: "1",
                updatedAt: "2026-10-09T00:00:00Z",
              }
            : null,
        },
      ],
      canManage: true,
      canCustomize: true,
    });
  });
  vi.stubGlobal("fetch", fetch);
  mount("official");
  fireEvent.click(await screen.findByRole("button", { name: "立即启用" }));
  await screen.findByRole("button", { name: "已启用" });
  expect(
    fetch.mock.calls.some(
      (c) =>
        new Headers(c[1]?.headers).get("X-Expected-Organization-ID") ===
        "org-a",
    ),
  ).toBe(true);
});
it("unknown response retains its original key through remount", async () => {
  const keys: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === "PUT") {
        keys.push(new Headers(init.headers).get("Idempotency-Key")!);
        throw new Error("lost");
      }
      return Response.json({
        tools: [tool],
        canManage: true,
        canCustomize: true,
      });
    }),
  );
  const first = mount("official");
  fireEvent.click(await screen.findByRole("button", { name: "立即启用" }));
  await screen.findByRole("button", { name: "重试原操作" });
  first.unmount();
  mount("official");
  fireEvent.click(await screen.findByRole("button", { name: "重试原操作" }));
  await waitFor(() => expect(keys).toHaveLength(2));
  expect(keys[0]).toBe(keys[1]);
});
it("readonly members cannot enable the enterprise tool", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        Response.json({ tools: [tool], canManage: false, canCustomize: false }),
      ),
  );
  mount("official");
  expect(
    await screen.findByRole("button", { name: "立即启用" }),
  ).toBeDisabled();
});
it("customization submission opens its persisted demand and progress", async () => {
  const id = "aa1043df-c64c-499d-9c99-57598f7bff18",
    now = "2026-10-09T00:00:00Z";
  let demand: { kind: string; title: string; description: string } | null =
    null;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        demand = JSON.parse(String(init.body));
        return Response.json({
          commandId: new Headers(init.headers).get("Idempotency-Key"),
          operation: "create",
          id,
          revision: "1",
          committedAt: now,
        });
      }
      if (url.endsWith("/market"))
        return Response.json({
          tools: [tool],
          canManage: true,
          canCustomize: true,
        });
      const record = {
        id,
        organizationId: "org-a",
        kind: "DATA",
        title: "采购商品资料",
        description: "保存授权来源的商品资料",
        stage: "SUBMITTED",
        revision: "1",
        createdAt: now,
        updatedAt: now,
      };
      if (url.endsWith("/requests/" + id))
        return Response.json({
          request: record,
          events: [
            {
              revision: "1",
              stage: "SUBMITTED",
              note: "需求已保存，待硕米专员评估",
              occurredAt: now,
            },
          ],
        });
      const { description: _description, ...summary } = record;
      return Response.json({ items: [summary], nextCursor: "" });
    }),
  );
  mount("custom");
  const buttons = await screen.findAllByRole("button", {
    name: "提交定制需求",
  });
  await waitFor(() => expect(buttons[0]).toBeEnabled());
  fireEvent.click(buttons[0]);
  fireEvent.change(screen.getByLabelText("需求标题"), {
    target: { value: "采购商品资料" },
  });
  fireEvent.change(screen.getByLabelText("需求说明"), {
    target: { value: "保存授权来源的商品资料" },
  });
  fireEvent.click(screen.getByRole("button", { name: "保存需求，等待评估" }));
  await screen.findByText("需求已保存，待硕米专员评估");
  expect(demand).toEqual({
    kind: "DATA",
    title: "采购商品资料",
    description: "保存授权来源的商品资料",
  });
});
