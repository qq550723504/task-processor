import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ReferralsPage, type ReferralView } from "./referrals-page";

const state = vi.hoisted(() => ({
  context: { user: { id: "subject-1" } as { id: string } | null, isLoading: false, isSwitching: false, error: null as { code: string } | null, blockingError: null as { code: string } | null },
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));

const clients: QueryClient[] = [];
function mount(mode: "overview" | "complete" = "overview", expectedUserId = "subject-1", registrationAvailable = true, view?: ReferralView) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  clients.push(client);
  const child = (subject = expectedUserId) => <QueryClientProvider client={client}><ReferralsPage mode={mode} view={view} expectedUserId={subject} registrationAvailable={registrationAvailable} /></QueryClientProvider>;
  const rendered = render(child());
  return { ...rendered, update: (subject = expectedUserId) => rendered.rerender(child(subject)) };
}

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
  state.context = { user: { id: "subject-1" }, isLoading: false, isSwitching: false, error: null, blockingError: null };
  vi.unstubAllGlobals();
});

describe("ReferralsPage", () => {
  it("reads promotion rules from the backend contract", async () => {
    const rules = { schemaVersion: "referral-rules-v1", currency: "CNY", commissionRateBps: 1000, settlementPeriodDays: 14, minimumWithdrawalMinor: "10000", withdrawalReview: "manual", earningsBasis: "canonical_settled_payment_refund_chargeback", source: "referral_economics_contract" };
    const fetch = vi.fn().mockResolvedValue(Response.json(rules));
    vi.stubGlobal("fetch", fetch);
    mount("overview", "subject-1", true, "rules");
    expect(await screen.findByText("10%，按人民币最小货币单位计算。")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe("/api/account/referral-rules");
  });

  it("uses the dedicated backend projection for the earnings page", async () => {
    const earnings = { schemaVersion: "referral-earnings-v1", referrer: "subject-1", currency: "CNY", pendingMinor: "2000", availableMinor: "10000", reservedMinor: "0", adjustmentMinor: "-500", version: "3", updatedAt: "2026-09-13T10:00:00Z", source: "referral_earnings_projection", entryLimit: 100, entries: [{ entryId: "entry-1", referrer: "subject-1", currency: "CNY", paymentId: "payment-1", entryType: "COMMISSION", amountMinor: "10000", referenceId: "payment-1", occurredAt: "2026-09-13T10:00:00Z" }] };
    const fetch = vi.fn().mockResolvedValue(Response.json(earnings));
    vi.stubGlobal("fetch", fetch);
    mount("overview", "subject-1", true, "earnings");
    expect(await screen.findByText("¥95.00")).toBeVisible();
    expect(screen.getByText(/Projection 版本：3/)).toBeVisible();
    expect(screen.getByText(/COMMISSION · ¥100.00 · 业务单据 payment-1/)).toBeVisible();
    expect(screen.getByText("逐笔列表仅展示最新最多 100 条记录；汇总以完整收益 projection 为准。")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe("/api/account/referral-earnings");
  });

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
      .mockResolvedValueOnce(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }))
      .mockResolvedValueOnce(Response.json({ code: "CODE1234" }))
      .mockResolvedValueOnce(Response.json({ code: "CODE1234", codeAvailability: "available", count: 0, generatedAt: "2026-09-13T10:01:00Z", earnings: { availability: "unavailable", amount: null } }))
      .mockResolvedValueOnce(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }));
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount();
    expect(await screen.findByText("尚未创建推广码")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(2);
    await user.click(screen.getByRole("button", { name: "创建推广码" }));
    expect(await screen.findByText("CODE1234")).toBeVisible();
    expect(fetch.mock.calls[2][1].method).toBe("POST");
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
    const fetch = vi.fn((input: string) => String(input).includes("referral-withdrawals")
      ? Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }))
      : Promise.resolve(Response.json({ code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } })));
    vi.stubGlobal("fetch", fetch);
    mount();
    expect(await screen.findByText("CODE1234")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("keeps personal count readable but disables an unconfigured invitation entry", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "CODE1234", codeAvailability: "available", count: 2, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } })));
    mount("overview", "subject-1", false);
    expect(await screen.findByText("2")).toBeVisible();
    expect(screen.queryByRole("link", { name: "打开邀请链接" })).not.toBeInTheDocument();
    expect(screen.getByText("注册入口暂不可用")).toBeVisible();
  });

  it("renders unavailable earnings instead of a disabled payout-method spinner", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    const fetch = vi.fn((input: string) => String(input).includes("referral-withdrawals")
      ? Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }))
      : Promise.resolve(Response.json(projection)));
    vi.stubGlobal("fetch", fetch);
    mount("overview", "subject-1", true, "withdrawals");
    expect(await screen.findByText("收益数据暂不可用，暂不能读取收款方式或提交提现申请。请刷新后重试。")).toBeVisible();
    expect(screen.queryByText("正在读取已验证收款方式…")).not.toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("allows a user to cancel only the requested withdrawal", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "available", currency: "CNY", pendingMinor: "0", availableMinor: "12000", reservedMinor: "0", adjustmentMinor: "0", version: "1", updatedAt: "2026-09-13T10:00:00Z" } };
    const requested = { schemaVersion: "referral-withdrawal-v1", id: "withdrawal-1", currency: "CNY", method: "ALIPAY", payoutMethodId: "method-1", amountMinor: "10000", status: "REQUESTED", payoutReference: "", version: "2", createdAt: "2026-09-13T10:01:00Z", updatedAt: "2026-09-13T10:01:00Z" };
    const canceled = { ...requested, status: "CANCELED", version: "3", updatedAt: "2026-09-13T10:02:00Z" };
    const fetch = vi.fn((input: string, init?: RequestInit) => {
      const path = String(input);
      if (path === "/api/account/referrals") return Promise.resolve(Response.json(projection));
      if (path === "/api/account/referral-withdrawals" && !init?.method) return Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }));
      if (path === "/api/account/referral-payout-methods") return Promise.resolve(Response.json({ schemaVersion: "payout-methods-v1", methods: [{ methodId: "method-1", type: "ALIPAY", displayName: "支付宝", maskedDestination: "***1234", version: "1" }] }));
      if (path === "/api/account/referral-withdrawals" && init?.method === "POST") return Promise.resolve(Response.json(requested));
      if (path === "/api/account/referral-withdrawals/withdrawal-1/cancel" && init?.method === "POST") return Promise.resolve(Response.json(canceled));
      throw new Error(`unexpected fetch: ${path}`);
    });
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount();
    await user.type(await screen.findByLabelText("金额（分）"), "10000");
    await user.click(screen.getByRole("button", { name: "申请提现" }));
    expect(await screen.findByRole("button", { name: "取消提现申请" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "取消提现申请" }));
    expect(await screen.findByText("提现状态：CANCELED。")).toBeVisible();
    expect(fetch).toHaveBeenCalledWith("/api/account/referral-withdrawals/withdrawal-1/cancel", expect.objectContaining({ method: "POST" }));
  });

  it("replays an unknown withdrawal write with its original idempotency key", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "available", currency: "CNY", pendingMinor: "0", availableMinor: "12000", reservedMinor: "0", adjustmentMinor: "0", version: "1", updatedAt: "2026-09-13T10:00:00Z" } };
    const withdrawalResult = { schemaVersion: "referral-withdrawal-v1", id: "withdrawal-1", currency: "CNY", method: "ALIPAY", payoutMethodId: "method-1", amountMinor: "10000", status: "REQUESTED", payoutReference: "", version: "2", createdAt: "2026-09-13T10:01:00Z", updatedAt: "2026-09-13T10:01:00Z" };
    let withdrawalWrites = 0;
    const fetch = vi.fn((input: string, init?: RequestInit) => {
      const path = String(input);
      if (path === "/api/account/referrals") return Promise.resolve(Response.json(projection));
      if (path === "/api/account/referral-withdrawals" && !init?.method) return Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }));
      if (path === "/api/account/referral-payout-methods") return Promise.resolve(Response.json({ schemaVersion: "payout-methods-v1", methods: [{ methodId: "method-1", type: "ALIPAY", displayName: "支付宝", maskedDestination: "***1234", version: "1" }] }));
      if (path === "/api/account/referral-withdrawals" && init?.method === "POST") {
        withdrawalWrites += 1;
        return withdrawalWrites === 1 ? Promise.resolve(Response.json({ code: "RESULT_UNVERIFIED", message: "unknown", requestId: "", fieldErrors: [], outcome: "unknown" }, { status: 504 })) : Promise.resolve(Response.json(withdrawalResult));
      }
      throw new Error(`unexpected fetch: ${path}`);
    });
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount("overview", "subject-1", true, "withdrawals");
    await user.type(await screen.findByLabelText("金额（分）"), "10000");
    await user.click(screen.getByRole("button", { name: "申请提现" }));
    expect((await screen.findAllByText("操作结果待核实"))[0]).toBeVisible();
    expect(screen.getByRole("button", { name: "申请提现" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "刷新提现状态" }));
    await waitFor(() => expect(screen.queryByText("操作结果待核实")).not.toBeInTheDocument());
    const writes = fetch.mock.calls.filter(([input, init]) => input === "/api/account/referral-withdrawals" && init?.method === "POST");
    expect(writes).toHaveLength(2);
    expect(new Headers(writes[0]?.[1]?.headers).get("Idempotency-Key")).toBe(new Headers(writes[1]?.[1]?.headers).get("Idempotency-Key"));
    expect(fetch.mock.calls.filter(([input]) => input === "/api/account/referrals").length).toBe(2);
  });

  it("replays an unknown payout-method write with its original idempotency key", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "available", currency: "CNY", pendingMinor: "0", availableMinor: "12000", reservedMinor: "0", adjustmentMinor: "0", version: "1", updatedAt: "2026-09-13T10:00:00Z" } };
    const method = { schemaVersion: "payout-method-v1", methodId: "method-1", type: "ALIPAY", displayName: "支付宝账户", maskedDestination: "***1234", version: "1" };
    const listedMethod = { methodId: method.methodId, type: method.type, displayName: method.displayName, maskedDestination: method.maskedDestination, version: method.version };
    let payoutWrites = 0;
    const fetch = vi.fn((input: string, init?: RequestInit) => {
      const path = String(input);
      if (path === "/api/account/referrals") return Promise.resolve(Response.json(projection));
      if (path === "/api/account/referral-withdrawals") return Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: [] }));
      if (path === "/api/account/referral-payout-methods" && init?.method === "POST") {
        payoutWrites += 1;
        return payoutWrites === 1
          ? Promise.resolve(Response.json({ code: "RESULT_UNVERIFIED", message: "unknown", requestId: "", fieldErrors: [], outcome: "unknown" }, { status: 504 }))
          : Promise.resolve(Response.json(method, { status: 201 }));
      }
      if (path === "/api/account/referral-payout-methods") return Promise.resolve(Response.json({ schemaVersion: "payout-methods-v1", methods: payoutWrites >= 2 ? [listedMethod] : [] }));
      throw new Error(`unexpected fetch: ${path}`);
    });
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount("overview", "subject-1", true, "withdrawals");
    await user.type(await screen.findByLabelText("名称"), "支付宝账户");
    await user.type(screen.getByLabelText("账号或收款地址"), "buyer@example.test");
    await user.click(screen.getByRole("button", { name: "登记收款方式" }));
    expect((await screen.findAllByText("操作结果待核实"))[0]).toBeVisible();
    await user.click(screen.getByRole("button", { name: "刷新提现状态" }));
    await waitFor(() => expect(screen.queryByText("操作结果待核实")).not.toBeInTheDocument());
    const writes = fetch.mock.calls.filter(([input, init]) => input === "/api/account/referral-payout-methods" && init?.method === "POST");
    expect(writes).toHaveLength(2);
    expect(new Headers(writes[0]?.[1]?.headers).get("Idempotency-Key")).toBe(new Headers(writes[1]?.[1]?.headers).get("Idempotency-Key"));
    expect(screen.getByText("支付宝账户 · ***1234")).toBeVisible();
  });

  it("replays an unknown cancellation with its original idempotency key", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "available", currency: "CNY", pendingMinor: "0", availableMinor: "12000", reservedMinor: "0", adjustmentMinor: "0", version: "1", updatedAt: "2026-09-13T10:00:00Z" } };
    const requested = { schemaVersion: "referral-withdrawal-v1", id: "withdrawal-1", currency: "CNY", method: "ALIPAY", payoutMethodId: "method-1", amountMinor: "10000", status: "REQUESTED", payoutReference: "", version: "2", createdAt: "2026-09-13T10:01:00Z", updatedAt: "2026-09-13T10:01:00Z" };
    const canceled = { id: requested.id, currency: requested.currency, method: requested.method, payoutMethodId: requested.payoutMethodId, amountMinor: requested.amountMinor, status: "CANCELED", payoutReference: requested.payoutReference, version: "3", createdAt: requested.createdAt, updatedAt: "2026-09-13T10:02:00Z" };
    const canceledResult = { schemaVersion: "referral-withdrawal-v1", ...canceled };
    let cancelWrites = 0;
    const fetch = vi.fn((input: string, init?: RequestInit) => {
      const path = String(input);
      if (path === "/api/account/referrals") return Promise.resolve(Response.json(projection));
      if (path === "/api/account/referral-payout-methods") return Promise.resolve(Response.json({ schemaVersion: "payout-methods-v1", methods: [{ methodId: "method-1", type: "ALIPAY", displayName: "支付宝", maskedDestination: "***1234", version: "1" }] }));
      if (path === "/api/account/referral-withdrawals" && init?.method === "POST") return Promise.resolve(Response.json(requested));
      if (path === "/api/account/referral-withdrawals/withdrawal-1/cancel") {
        cancelWrites += 1;
        return cancelWrites === 1 ? Promise.resolve(Response.json({ code: "RESULT_UNVERIFIED", message: "unknown", requestId: "", fieldErrors: [], outcome: "unknown" }, { status: 504 })) : Promise.resolve(Response.json(canceledResult));
      }
      if (path === "/api/account/referral-withdrawals") {
        return Promise.resolve(Response.json({ schemaVersion: "referral-withdrawals-v1", withdrawals: cancelWrites >= 2 ? [canceled] : [] }));
      }
      throw new Error(`unexpected fetch: ${path}`);
    });
    vi.stubGlobal("fetch", fetch);
    const user = userEvent.setup();
    mount("overview", "subject-1", true, "withdrawals");
    await user.type(await screen.findByLabelText("金额（分）"), "10000");
    await user.click(screen.getByRole("button", { name: "申请提现" }));
    await user.click(await screen.findByRole("button", { name: "取消提现申请" }));
    expect((await screen.findAllByText("操作结果待核实"))[0]).toBeVisible();
    await user.click(screen.getByRole("button", { name: "刷新提现状态" }));
    await waitFor(() => expect(screen.queryByText("操作结果待核实")).not.toBeInTheDocument());
    const writes = fetch.mock.calls.filter(([input, init]) => input === "/api/account/referral-withdrawals/withdrawal-1/cancel" && init?.method === "POST");
    expect(writes).toHaveLength(2);
    expect(new Headers(writes[0]?.[1]?.headers).get("Idempotency-Key")).toBe(new Headers(writes[1]?.[1]?.headers).get("Idempotency-Key"));
    expect(screen.getByText("提现状态：CANCELED。")).toBeVisible();
    expect(screen.queryByRole("button", { name: "取消提现申请" })).not.toBeInTheDocument();
  });

  it("shows available earnings after immutable refund adjustments", async () => {
    const projection = { code: "CODE1234", codeAvailability: "available", count: 1, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "available", currency: "CNY", pendingMinor: "0", availableMinor: "12000", reservedMinor: "0", adjustmentMinor: "-2000", version: "2", updatedAt: "2026-09-13T10:00:00Z" } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(projection)));
    mount();
    expect(await screen.findByText("¥100.00")).toBeVisible();
    expect(screen.getByText(/调整 -¥20.00/)).toBeVisible();
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
    const view = mount("complete");
    await user.click(await screen.findByRole("button", { name: "完成推广关系" }));
    expect(await screen.findByText("推广关系已确认")).toBeVisible();
    expect(screen.getByText("推广汇总暂不可用")).toBeVisible();
    state.context.user = null;
    state.context.blockingError = { code: "DEPENDENCY_UNAVAILABLE" };
    view.update();
    expect(screen.getByText("推广关系已确认")).toBeVisible();
    state.context.user = { id: "subject-1" };
    state.context.blockingError = null;
    view.update();
    expect(screen.getByText("推广关系已确认")).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(3);
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

  it("treats a disconnected create response as an unknown write outcome", async () => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json(projection)).mockRejectedValueOnce(new TypeError("connection lost")));
    const user = userEvent.setup();
    mount();
    await user.click(await screen.findByRole("button", { name: "创建推广码" }));
    expect(await screen.findByText("推广服务暂不可用")).toBeVisible();
    expect(screen.getByText("操作结果尚未确认。请先刷新相关状态核对服务端事实，再决定是否重试。")).toBeVisible();
    expect(screen.queryByText("没有生成或推测推广事实。请稍后重试原操作。")).not.toBeInTheDocument();
  });

  it("treats an invalid complete response as an unknown write outcome", async () => {
    const projection = { code: "", codeAvailability: "not_created", count: 0, generatedAt: "2026-09-13T10:00:00Z", earnings: { availability: "unavailable", amount: null } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(Response.json(projection)).mockResolvedValueOnce(Response.json({ unexpected: true })));
    const user = userEvent.setup();
    mount("complete");
    await user.click(await screen.findByRole("button", { name: "完成推广关系" }));
    expect(await screen.findByText("操作结果尚未确认。请先刷新相关状态核对服务端事实，再决定是否重试。")).toBeVisible();
    expect(screen.queryByText("没有生成或推测推广事实。请稍后重试原操作。")).not.toBeInTheDocument();
  });
});
