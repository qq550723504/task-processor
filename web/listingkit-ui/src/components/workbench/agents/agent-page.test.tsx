import { afterEach, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { AgentPage } from "./agent-page";
const scope = vi.hoisted(() => ({
  user: { id: "actor" },
  effectiveOrganization: { id: "org-a", name: "企业A" },
  roles: ["listingkit_admin"],
  permissions: [
    "listingkit.admin.read",
    "listingkit.admin.write",
    "product_sourcing.write",
    "local_agent.write",
    "listingkit.image_agent.read",
    "listingkit.image_agent.write",
    "workbench.agent.read",
    "workbench.agent.use",
    "workbench.agent.configure",
    "workbench.chat.read",
    "workbench.chat.use",
    "workbench.task.read",
    "workbench.knowledge.read",
    "workbench.knowledge.manage",
    "workbench.store.read",
    "workbench.store.create",
    "workbench.store.update",
    "workbench.store.lifecycle",
    "workbench.store.delete",
    "workbench.source_account.read",
    "workbench.source_account.manage",
    "workbench.organization_member.read",
    "workbench.organization_member.manage",
    "workbench.commercial.read",
    "workbench.commercial.purchase",
    "workbench.commercial.wallet_topup",
  ],
  isLoading: false,
  isSwitching: false,
  error: null,
  blockingError: null,
  selectionRequired: false,
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => scope,
}));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.ComponentProps<"a">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
const item = {
  agent: {
    agentId: "product.title.agent",
    activation: "NOT_ENABLED",
    revision: "",
    activationEpoch: "",
    defaultTemplate: null,
    updatedAt: "0001-01-01T00:00:00Z",
  },
  name: "商品标题优化智能体",
  description: "生成标题建议",
  definitionVersion: "v1.0.0",
  parameterSchema: "title-config-v1",
  canConfigure: true,
  canUse: false,
  canReadRuns: false,
  capabilities: [],
};
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  scope.effectiveOrganization = { id: "org-a", name: "企业A" };
  scope.isSwitching = false;
});
it("waits for a committed enable receipt and refetches the shared fact", async () => {
  let enabled = false;
  const fetch = vi.fn(async (_url: unknown, init?: RequestInit) => {
    if (init?.method === "POST") {
      enabled = true;
      return Response.json({
        commandId: "8fc227bb-b572-4138-8e2a-5f1a0be98617",
        operation: "enable",
        agentId: "product.title.agent",
        revision: "1",
        noop: false,
        committedAt: "2026-09-29T00:00:00Z",
      });
    }
    return Response.json({
      items: [
        enabled
          ? {
              ...item,
              agent: {
                ...item.agent,
                activation: "ENABLED",
                revision: "1",
                activationEpoch: "1",
              },
            }
          : item,
      ],
      nextCursor: "",
    });
  });
  vi.stubGlobal("fetch", fetch);
  render(<AgentPage mode="market" />);
  fireEvent.click(
    await screen.findByRole("button", { name: "启用到当前企业" }),
  );
  await waitFor(() => expect(screen.getByText("已启用")).toBeInTheDocument());
  const call = fetch.mock.calls.find(([, init]) => init?.method === "POST")!;
  expect(new Headers(call[1]?.headers).get("If-None-Match")).toBe("*");
});
it("drops a late protected response after switching A to B to A", async () => {
  let resolve!: (r: Response) => void;
  const old = new Promise<Response>((r) => {
    resolve = r;
  });
  const fetch = vi
    .fn()
    .mockReturnValueOnce(old)
    .mockResolvedValue(Response.json({ items: [], nextCursor: "" }));
  vi.stubGlobal("fetch", fetch);
  const view = render(<AgentPage mode="mine" />);
  scope.isSwitching = true;
  view.rerender(<AgentPage mode="mine" />);
  scope.effectiveOrganization = { id: "org-b", name: "企业B" };
  scope.isSwitching = false;
  view.rerender(<AgentPage mode="mine" />);
  scope.isSwitching = true;
  view.rerender(<AgentPage mode="mine" />);
  scope.effectiveOrganization = { id: "org-a", name: "企业A" };
  scope.isSwitching = false;
  view.rerender(<AgentPage mode="mine" />);
  await act(async () =>
    resolve(
      Response.json({
        items: [{ ...item, name: "旧企业受保护内容" }],
        nextCursor: "",
      }),
    ),
  );
  expect(screen.queryByText("旧企业受保护内容")).toBeNull();
});

it("shows the required acquisition permission when title execution cannot use evidence", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      Response.json({
        items: [
          {
            ...item,
            agent: {
              ...item.agent,
              activation: "ENABLED",
              revision: "1",
              activationEpoch: "1",
            },
            canConfigure: false,
            canUse: false,
            canReadRuns: true,
            capabilities: [
              {
                id: "text.generate",
                support: "REQUIRED",
                readiness: "AVAILABLE",
                reason: "模型已接入",
                observedAt: "2026-10-08T00:00:00Z",
              },
              {
                id: "product.source-evidence",
                support: "REQUIRED",
                readiness: "REQUIRES_AUTHORIZATION",
                reason: "标题执行需要1688采集模块权限",
                observedAt: "2026-10-08T00:00:00Z",
              },
              {
                id: "knowledge.context",
                support: "OPTIONAL",
                readiness: "UNAVAILABLE",
                reason: "未接入",
                observedAt: "2026-10-08T00:00:00Z",
              },
              {
                id: "image.generate",
                support: "NOT_SUPPORTED",
                readiness: "UNAVAILABLE",
                reason: "未提供",
                observedAt: "2026-10-08T00:00:00Z",
              },
              {
                id: "platform.write",
                support: "NOT_SUPPORTED",
                readiness: "UNAVAILABLE",
                reason: "未提供",
                observedAt: "2026-10-08T00:00:00Z",
              },
            ],
          },
        ],
        nextCursor: "",
      }),
    ),
  );
  render(<AgentPage mode="mine" />);
  expect(await screen.findByText("标题执行需要1688采集模块权限")).toBeTruthy();
  expect(screen.getByText("采集证据读取")).toBeTruthy();
});
