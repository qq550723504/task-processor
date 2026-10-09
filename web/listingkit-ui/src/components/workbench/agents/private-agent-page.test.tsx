import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AgentPage } from "./agent-page";
import { PrivateAgentPage } from "./private-agent-page";
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "actor" }, effectiveOrganization: { id: "org", name: "测试企业" }, permissions: ["workbench.agent.read", "workbench.agent.use"], registerOrganizationSwitchGuard: () => () => { } }) }));
const id = "b510c346-54e7-4cbd-91b2-05f7c0149b42";
const delivery = { id, requestId: id, organizationId: "org", definition: "product.quality.check", version: "1.0.0", name: "商品资料质检", createdBy: "staff", createdAt: "2026-10-09T00:00:00Z" };
const input = { name: "盒子", material: "", dimensions: "", description: "", specifications: [] };
const run = { id, deliveryId: id, organizationId: "org", actorId: "actor", key: id, definition: "product.quality.check", version: "1.0.0", input, report: { ruleVersion: "product-quality-rules-v1", summary: "请补充资料", findings: [{ code: "MISSING_MATERIAL", field: "material", message: "尚未提供材质", suggestion: "核实真实材质后填写" }] }, createdAt: "2026-10-09T00:00:00Z" };
beforeEach(() => { sessionStorage.clear(); vi.restoreAllMocks(); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
it("keeps the private executable card available when title configuration is unavailable", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(async url => String(url).includes("agent-customization/agents") ? Response.json({ items: [delivery], nextCursor: "" }) : Response.json({ code: "DEPENDENCY_UNAVAILABLE" }, { status: 503 }));
  render(<AgentPage mode="mine" />);
  expect(await screen.findByRole("link", { name: "进入使用" })).toHaveAttribute("href", "/workbench/agents/mine/private/" + id);
  expect(await screen.findByText("智能体配置不可用")).toBeInTheDocument();
});
it("restores an unknown command after reload and reuses its original input and key", async () => {
  const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (url, init) => {
    if (init?.method === "POST") throw new Error("response lost");
    return Response.json(String(url).endsWith("/reports") ? { items: [], nextCursor: "" } : delivery);
  });
  const first = render(<PrivateAgentPage id={id} />);
  const name = await screen.findByRole("textbox", { name: "商品名称" }); fireEvent.change(name, { target: { value: "盒子" } });
  fireEvent.click(screen.getByRole("button", { name: "检查并保存报告" }));
  await screen.findByText("结果尚未确认，请核实同一次检查。");
  const original = fetch.mock.calls.find(v => v[1]?.method === "POST")!;
  first.unmount();
  fetch.mockImplementation(async (url, init) => {
    if (init?.method === "POST") return Response.json({ ...run, key: new Headers(init.headers).get("Idempotency-Key") });
    return Response.json(String(url).endsWith("/reports") ? { items: [run], nextCursor: "" } : delivery);
  });
  render(<PrivateAgentPage id={id} />);
  fireEvent.click(await screen.findByRole("button", { name: "核实同一次检查" }));
  await screen.findByRole("heading", { name: "质检报告 · 盒子" });
  const commands = fetch.mock.calls.filter(v => v[1]?.method === "POST"); expect(commands).toHaveLength(2);
  expect(commands[1][1]?.body).toBe(original[1]?.body);
  expect(new Headers(commands[1][1]?.headers).get("Idempotency-Key")).toBe(new Headers(original[1]?.headers).get("Idempotency-Key"));
  await waitFor(() => expect(screen.queryByRole("button", { name: "核实同一次检查" })).not.toBeInTheDocument());
  expect(screen.getByText("核实真实材质后填写")).toBeInTheDocument();
});
