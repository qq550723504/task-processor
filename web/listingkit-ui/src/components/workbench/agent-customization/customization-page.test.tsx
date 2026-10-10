import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { webcrypto } from "node:crypto";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { CustomizationPage } from "./customization-page";
import { freezeCustomization, forgetCustomization, restoreCustomization } from "./pending";
const ctx = vi.hoisted(() => ({ user: { id: "member" }, effectiveOrganization: { id: "org-a", name: "企业A" }, permissions: ["workbench.agent.read", "workbench.agent.use"], roles: ["listingkit_operator"], isLoading: false, isSwitching: false, error: null, blockingError: null, selectionRequired: false }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ctx }));
vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.ComponentProps<"a">) => <a href={href} {...props}>{children}</a> }));
beforeEach(() => vi.stubGlobal("crypto", webcrypto));
afterEach(() => { cleanup(); sessionStorage.clear(); vi.unstubAllGlobals(); ctx.effectiveOrganization = { id: "org-a", name: "企业A" }; });
const id = "11111111-1111-4111-8111-111111111111";
const at = "2026-10-09T00:00:00Z";
it("records offline confirmation before development without rewriting the proposal", async () => {
    const request = { id, organizationId: "org-a", createdBy: "member", input: { name: "需求", scenario: "场景", direction: "OTHER", description: "说明", contactName: "联系人", contactMethod: "test", consent: true }, stage: "PROPOSED", version: "3", proposal: "方案与报价", offlineConfirmation: "", consentVersion: "agent-customization-contact-v1", attachments: [], createdAt: at, updatedAt: at };
    const fetch = vi.fn().mockImplementation(async (_url: string, init: RequestInit) => {
        if (init.method === "POST") {
            const key = new Headers(init.headers).get("Idempotency-Key");
            return Response.json({ requestId: id, key, version: "4", stage: "DEVELOPING", at });
        }
        return Response.json(String(_url).endsWith(id) ? { request, events: [], nextEventVersion: "" } : { items: [request], nextCursor: "" });
    });
    vi.stubGlobal("fetch", fetch);
    render(<CustomizationPage mode="admin"/>);
    fireEvent.click(await screen.findByRole("button", { name: "查看需求" }));
    await screen.findByText("记录本次跟进");
    fireEvent.change(screen.getByLabelText("本次阶段"), { target: { value: "DEVELOPING" } });
    fireEvent.change(screen.getByLabelText("跟进说明 *"), { target: { value: "开始开发" } });
    fireEvent.change(screen.getByLabelText(/线下确认记录（/), { target: { value: "客户在线下确认方案与费用" } });
    fireEvent.click(screen.getByRole("button", { name: "保存跟进记录" }));
    await waitFor(() => expect(fetch.mock.calls.some(([, init]) => init.method === "POST")).toBe(true));
    const call = fetch.mock.calls.find(([, init]) => init.method === "POST")!;
    expect(JSON.parse(String(call[1].body))).toEqual({ stage: "DEVELOPING", note: "开始开发", offlineConfirmation: "客户在线下确认方案与费用" });
    expect(new Headers(call[1].headers).get("If-Match")).toBe('"3"');
});
function fill() { for (const [label, value] of [["需求名称 *", "标题需求"], ["主要使用场景 *", "商品维护"], ["希望智能体完成什么？*", "形成标题建议"], ["联系人 *", "测试联系人"], ["手机或微信 *", "test-contact"]])
    fireEvent.change(screen.getByLabelText(label), { target: { value } }); fireEvent.click(screen.getByLabelText("其他")); }
it("requires explicit consent and retains exact intent after unknown submission", async () => { const fetch = vi.fn().mockRejectedValue(new Error("lost")); vi.stubGlobal("fetch", fetch); render(<CustomizationPage mode="new"/>); fill(); expect(screen.getByLabelText(/我同意平台专员/)).not.toBeChecked(); fireEvent.click(screen.getByRole("button", { name: "提交定制需求" })); expect(fetch).not.toHaveBeenCalled(); fireEvent.click(screen.getByLabelText(/我同意平台专员/)); fireEvent.click(screen.getByRole("button", { name: "提交定制需求" })); await screen.findByText(/操作结果尚未确认/); const first = fetch.mock.calls[0][1]; fireEvent.click(screen.getByRole("button", { name: "核实同一次操作" })); await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2)); expect(fetch.mock.calls[1][1].body).toBe(first.body); expect(new Headers(fetch.mock.calls[1][1].headers).get("Idempotency-Key")).toBe(new Headers(first.headers).get("Idempotency-Key")); });
it("restores an unknown request after leaving and returning without creating a new intent", async () => {
    const fetch = vi.fn().mockRejectedValue(new Error("lost"));
    vi.stubGlobal("fetch", fetch);
    const page = render(<CustomizationPage mode="new"/>);
    fill();
    fireEvent.click(screen.getByLabelText(/我同意平台专员/));
    fireEvent.click(screen.getByRole("button", { name: "提交定制需求" }));
    await screen.findByText(/操作结果尚未确认/);
    const original = fetch.mock.calls[0][1];
    page.unmount();
    render(<CustomizationPage mode="new"/>);
    expect(screen.getByRole("button", { name: "提交定制需求" })).toBeDisabled();
    fireEvent.click(await screen.findByRole("button", { name: "核实同一次操作" }));
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(fetch.mock.calls[1][1].body).toBe(original.body);
    expect(new Headers(fetch.mock.calls[1][1].headers).get("Idempotency-Key")).toBe(new Headers(original.headers).get("Idempotency-Key"));
});
it("keeps an unresolved marker locked if large attachment bytes cannot be recovered", async () => {
    const storageKey = 'resource.pending:' + JSON.stringify(["member", "org-a", "agent-customization", "enterprise"]), key = "22222222-2222-4222-8222-222222222222";
    const body = JSON.stringify({ name: "需求", scenario: "场景", direction: "OTHER", description: "说明", contactName: "联系人", contactMethod: "test", consent: true, files: [{ name: "参考.txt", data: Buffer.from("x".repeat(150000)).toString("base64") }] });
    const record = await freezeCustomization(storageKey, { key, body, path: "" });
    expect(record.body).toBeUndefined();
    expect(restoreCustomization(storageKey, record)?.body).toBe(body);
    sessionStorage.setItem(storageKey, JSON.stringify(record));
    forgetCustomization(storageKey, key);
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    render(<CustomizationPage mode="new"/>);
    await screen.findByText(/附件载荷已无法恢复/);
    expect(screen.getByRole("button", { name: "提交定制需求" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "核实同一次操作" })).not.toBeInTheDocument();
    expect(fetch).not.toHaveBeenCalled();
});
it("releases a known 412 rejection so the specialist can read and confirm a new version", async () => {
    const request = { id, organizationId: "org-a", createdBy: "member", input: { name: "需求", scenario: "场景", direction: "OTHER", description: "说明", contactName: "联系人", contactMethod: "test", consent: true }, stage: "SUBMITTED", version: "1", proposal: "", offlineConfirmation: "", consentVersion: "agent-customization-contact-v1", attachments: [], createdAt: at, updatedAt: at };
    let posts = 0;
    const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => { if (init.method === "POST") {
        posts++;
        if (posts === 1) {
            request.stage = "EVALUATING";
            request.version = "2";
            return Response.json({ code: "CUSTOMIZATION_REVISION_MISMATCH" }, { status: 412 });
        }
        return Response.json({ requestId: id, key: new Headers(init.headers).get("Idempotency-Key"), version: "3", stage: "PROPOSED", at });
    } return Response.json(url.endsWith(id) ? { request, events: [], nextEventVersion: "" } : { items: [request], nextCursor: "" }); });
    vi.stubGlobal("fetch", fetch);
    render(<CustomizationPage mode="admin"/>);
    fireEvent.click(await screen.findByRole("button", { name: "查看需求" }));
    await screen.findByText("记录本次跟进");
    fireEvent.change(screen.getByLabelText("本次阶段"), { target: { value: "EVALUATING" } });
    fireEvent.change(screen.getByLabelText("跟进说明 *"), { target: { value: "评估" } });
    fireEvent.click(screen.getByRole("button", { name: "保存跟进记录" }));
    await screen.findByText(/进度已被更新/);
    expect(screen.getByRole("button", { name: "保存跟进记录" })).not.toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "刷新进度" }));
    await waitFor(() => expect(screen.getByLabelText("本次阶段")).toHaveValue("EVALUATING"));
    fireEvent.change(screen.getByLabelText("本次阶段"), { target: { value: "PROPOSED" } });
    fireEvent.change(screen.getByLabelText("跟进说明 *"), { target: { value: "提交方案" } });
    fireEvent.change(screen.getByLabelText(/方案与报价（/), { target: { value: "方案报价" } });
    fireEvent.click(screen.getByRole("button", { name: "保存跟进记录" }));
    await waitFor(() => expect(posts).toBe(2));
    const calls = fetch.mock.calls.filter(([, init]) => init.method === "POST");
    expect(new Headers(calls[1][1].headers).get("If-Match")).toBe('"2"');
    expect(new Headers(calls[1][1].headers).get("Idempotency-Key")).not.toBe(new Headers(calls[0][1].headers).get("Idempotency-Key"));
});
it("does not invent progress from an empty persisted list", async () => { vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ items: [], nextCursor: "" }))); render(<CustomizationPage mode="progress"/>); await screen.findByText("当前企业暂无定制需求"); expect(screen.queryByText(/98%|126|运行中/)).not.toBeInTheDocument(); });
it("drops a late enterprise A response after switching to B", async () => { let finish: (v: Response) => void = () => { }; const fetch = vi.fn().mockImplementationOnce(() => new Promise<Response>(resolve => { finish = resolve; })).mockResolvedValue(Response.json({ items: [], nextCursor: "" })); vi.stubGlobal("fetch", fetch); const page = render(<CustomizationPage mode="progress"/>); ctx.effectiveOrganization = { id: "org-b", name: "企业B" }; page.rerender(<CustomizationPage mode="progress"/>); await screen.findByText("当前企业暂无定制需求"); await act(async () => finish(Response.json({ items: [], nextCursor: "" }))); expect(screen.getByText(/企业B/)).toBeInTheDocument(); expect(screen.queryByText(/企业A/)).not.toBeInTheDocument(); });
