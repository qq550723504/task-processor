import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ReferralsPage } from "./referrals-page";

const state = vi.hoisted(() => ({
  context: { user: { id: "subject-1" } as { id: string } | null, isLoading: false, isSwitching: false, error: null as { code: string } | null, blockingError: null as { code: string } | null },
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));

const clients: QueryClient[] = [];
function mount(mode: "overview" | "complete" = "overview", expectedUserId = "subject-1", registrationAvailable = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  clients.push(client);
  const child = (subject = expectedUserId) => <QueryClientProvider client={client}><ReferralsPage mode={mode} expectedUserId={subject} registrationAvailable={registrationAvailable} /></QueryClientProvider>;
  const view = render(child());
  return { ...view, update: (subject = expectedUserId) => view.rerender(child(subject)) };
}

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  state.context = { user: { id: "subject-1" }, isLoading: false, isSwitching: false, error: null, blockingError: null };
  vi.unstubAllGlobals();
});

describe("ReferralsPage", () => {
  it("shows actual relation count and leaves unsupported earnings unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "CODE1234", codeAvailability: "available", count: 2, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } })));
    mount();
    expect(await screen.findByText("2")).toBeVisible();
    expect(screen.getByText("CODE1234")).toBeVisible();
    expect(screen.getByText("收益数据暂不可用")).toBeVisible();
    expect(screen.queryByText(/¥0|￥0|0\.00/)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "打开邀请链接" })).toHaveAttribute("href", "/referrals/register?code=CODE1234");
  });

  it("distinguishes not-created from a real zero count and creates only on explicit click", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(Response.json({ code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } }))
      .mockResolvedValueOnce(Response.json({ code: "CODE1234" }))
      .mockResolvedValueOnce(Response.json({ code: "CODE1234", codeAvailability: "available", count: 0, generatedAt: "2026-09-13T10:01:00Z", earnings: { availability: "unavailable", amount: null } }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount();
    expect(await screen.findByText("尚未创建推广码")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "创建推广码" }));
    expect(await screen.findByText("CODE1234")).toBeVisible();
    expect(fetch.mock.calls[1][1].method).toBe("POST");
  });

  it("does not treat an organization switch as loss of personal referral access", async () => {
    state.context.isSwitching = true;
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } })));
    mount();
    expect(await screen.findByText("CODE1234")).toBeVisible();
    expect(screen.queryByText(/企业.*不可用|请选择当前企业/)).not.toBeInTheDocument();
  });

  it("reads personal facts when enterprise context is unavailable", async () => {
    state.context.user = null;
    state.context.blockingError = { code: "DEPENDENCY_UNAVAILABLE" };
    const fetch = vi.fn().mockResolvedValue(Response.json({ code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } }));
    vi.stubGlobal("fetch", fetch);
    mount();
    expect(await screen.findByText("CODE1234")).toBeVisible();
    expect(fetch).toHaveBeenCalledOnce();
  });

  it("keeps personal count readable but disables an unconfigured invitation entry", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "CODE1234", codeAvailability: "available", count: 2, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } })));
    mount("overview", "subject-1", false);
    expect(await screen.findByText("2")).toBeVisible();
    expect(screen.queryByRole("link", { name: "打开邀请链接" })).not.toBeInTheDocument();
    expect(screen.getByText("注册入口暂不可用")).toBeVisible();
  });

  it("clears an old subject result and prevents a late response from returning", async () => {
    let resolve!: (value: Response) => void;
    const fetch = vi.fn(() => new Promise<Response>((done) => { resolve = done; }));
    vi.stubGlobal("fetch", fetch);
    const view = mount();
    state.context.user = { id: "subject-2" };
    view.update();
    expect(screen.getByText("登录身份已变化")).toBeVisible();
    resolve(Response.json({ code: "OLD-CODE", codeAvailability: "available", count: 99, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } }));
    await waitFor(() => expect(screen.queryByText("OLD-CODE")).not.toBeInTheDocument());
  });

  it("clears an in-flight result when shared context reports authentication loss", async () => {
    let resolve!: (value: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((done) => { resolve = done; })));
    const view = mount();
    state.context.user = null;
    state.context.error = { code: "AUTHENTICATION_REQUIRED" };
    view.update();
    expect(screen.getByText("登录身份已变化")).toBeVisible();
    resolve(Response.json({ code: "OLD-CODE", codeAvailability: "available", count: 99, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } }));
    await waitFor(() => expect(screen.queryByText("OLD-CODE")).not.toBeInTheDocument());
  });

  it("completes only after an explicit click and then re-reads the durable projection", async () => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    const fetch = vi.fn().mockResolvedValueOnce(Response.json(projection)).mockResolvedValueOnce(Response.json({ status: "complete", intentID: "intent-1", boundAt: "2026-09-13T10:02:00Z" })).mockResolvedValueOnce(Response.json({ ...projection, count: 1 }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount("complete");
    expect(await screen.findByRole("button", { name: "完成推广关系" })).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "完成推广关系" }));
    expect(await screen.findByText("推广关系已确认")).toBeVisible();
    expect(fetch.mock.calls[1][0]).toBe("/api/account/referrals/complete");
  });

  it("keeps a successful receipt when the projection refresh is unavailable", async () => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    const fetch = vi.fn().mockResolvedValueOnce(Response.json(projection)).mockResolvedValueOnce(Response.json({ status: "complete", intentID: "intent-1", boundAt: "2026-09-13T10:02:00Z" })).mockResolvedValueOnce(Response.json({ error: "referral_unavailable" }, { status: 503 }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount("complete");
    await user.click(await screen.findByRole("button", { name: "完成推广关系" }));
    expect(await screen.findByText("推广关系已确认")).toBeVisible();
    expect(screen.getByText("推广汇总暂不可用")).toBeVisible();
  });

  it.each([[401, "AUTHENTICATION_REQUIRED"], [409, "IDENTITY_CONTEXT_CHANGED"]])("clears a receipt when refreshed identity fails with %s", async (status, code) => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json(projection)).mockResolvedValueOnce(Response.json({ status: "complete", intentID: "intent-1", boundAt: "2026-09-13T10:02:00Z" })).mockResolvedValueOnce(Response.json({ code }, { status })));
    const user = userEvent.setup();
    mount("complete");
    await user.click(await screen.findByRole("button", { name: "完成推广关系" }));
    expect(await screen.findByText("登录身份已变化")).toBeVisible();
    expect(screen.queryByText("推广关系已确认")).not.toBeInTheDocument();
  });

  it("does not claim an unknown write produced no fact", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "referral_outcome_unknown" }, { status: 503 })));
    mount();
    expect(await screen.findByText("暂时无法确认操作结果")).toBeVisible();
    expect(screen.queryByText("没有生成或推测推广事实。请稍后重试原操作。")).not.toBeInTheDocument();
  });
});
