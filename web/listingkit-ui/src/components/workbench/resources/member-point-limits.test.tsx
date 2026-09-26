import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MemberPointLimitError } from "@/lib/api/member-ai-point-limits";
import { MemberPointLimits } from "./member-point-limits";

const api = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn() }));
vi.mock("@/lib/api/member-ai-point-limits", async original => ({ ...await original<typeof import("@/lib/api/member-ai-point-limits")>(), getMemberAIPointLimits: api.read, setMemberAIPointLimit: api.write }));
const member = { organizationId: "org-1", memberId: "member-1", displayName: "成员甲", loginName: "member@example.test", roles: ["listingkit_operator"], configured: true, monthlyLimit: "25", consumed: "3", reserved: "2", remaining: "20", version: "1", monthStart: "2026-09-01T00:00:00Z", monthEnd: "2026-10-01T00:00:00Z" };
const snapshot = { schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: "org-1", resourceType: "ai_point", timezone: "UTC", members: [member] };
let client: QueryClient;
function tree(canManage = true, org = "org-1") { return <QueryClientProvider client={client}><MemberPointLimits userId="actor-1" organizationId={org} canManage={canManage} sequence={0} /></QueryClientProvider>; }
beforeEach(() => { client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); api.read.mockReset().mockResolvedValue(snapshot); api.write.mockReset().mockResolvedValue(member); });
afterEach(() => { cleanup(); client.clear(); });

it("shows actual member/month facts and saves an administrator target without a default", async () => {
  render(tree());
  expect(await screen.findByText("成员甲")).toBeVisible();
  expect(screen.getByText("25 / 月")).toBeVisible();
  expect(screen.getByText(/UTC 自然月/)).toBeVisible();
  expect(screen.getByText(/已消费 3 · 预留 2 · 剩余 20/)).toBeVisible();
  const input = screen.getByRole("textbox", { name: "成员甲 AI 点数月度上限" });
  await userEvent.clear(input); await userEvent.type(input, "40");
  await userEvent.click(screen.getByRole("button", { name: "保存上限" }));
  await waitFor(() => expect(api.write).toHaveBeenCalledTimes(1));
  expect(api.write).toHaveBeenCalledWith({ expectedUserId: "actor-1", expectedOrganizationId: "org-1" }, "member-1", "40", "1", expect.any(String));
});
it("unconfigured is not zero balance and viewers cannot edit", async () => {
  api.read.mockResolvedValue({ ...snapshot, members: [{ ...member, configured: false, monthlyLimit: "0", consumed: "0", reserved: "0", remaining: "0", version: "0" }] });
  render(tree(false));
  expect(await screen.findByText("未分配，不能消费")).toBeVisible();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "保存上限" })).not.toBeInTheDocument();
  expect(screen.queryByText("100 / 月")).not.toBeInTheDocument();
});
it("keeps unknown writes immutable and only manually replays the original command", async () => {
  api.write.mockRejectedValueOnce(new MemberPointLimitError(502, "RESULT_UNVERIFIED", "unknown")).mockResolvedValueOnce(member);
  render(tree()); await screen.findByText("成员甲");
  await userEvent.click(screen.getByRole("button", { name: "保存上限" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("结果未确认");
  expect(screen.getByRole("textbox")).toBeDisabled();
  expect(api.write).toHaveBeenCalledTimes(1);
  const original = api.write.mock.calls[0];
  await userEvent.click(screen.getByRole("button", { name: "核验原操作" }));
  await waitFor(() => expect(api.write).toHaveBeenCalledTimes(2));
  expect(api.write.mock.calls[1]).toEqual(original);
});
it("does not show stale facts after failed read or scope replacement", async () => {
  const view = render(tree()); await screen.findByText("成员甲");
  api.read.mockRejectedValue(new MemberPointLimitError(403, "FORBIDDEN"));
  await userEvent.click(screen.getByRole("button", { name: "刷新点数上限" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("无法读取");
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument();
  api.read.mockReturnValue(new Promise(() => {}));
  view.rerender(tree(true, "org-2"));
  expect(within(screen.getByRole("region", { name: "成员 AI 点数月度上限" })).getByRole("status")).toHaveTextContent("正在读取");
  expect(screen.queryByText("成员甲")).not.toBeInTheDocument();
});
