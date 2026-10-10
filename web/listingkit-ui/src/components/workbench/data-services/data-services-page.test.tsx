import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { DataServicesPage, CustomDetails } from "./data-services-page";
import { SpecialistPage } from "./specialist-page";
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ user: { id: "user" }, effectiveOrganization: { id: "org" }, registerOrganizationSwitchGuard: () => () => { } }) }));
vi.mock("@/components/workbench/collections/collection-page", () => ({ CollectionDialog: ({ title, children, onClose }: {
        title: string;
        children: React.ReactNode;
        onClose: () => void;
    }) => <section role="dialog" aria-label={title}><button onClick={onClose}>关闭</button>{children}</section> }));
const options = { sites: [{ code: "us", domain: "www.amazon.com", name: "美国" }], customSites: [{ code: "us", domain: "www.amazon.com", name: "美国" }], fields: ["asin", "title"], priceFen: 5, maximumRows: 200, formats: ["csv", "json", "xlsx"], acquisitionReady: true };
const id = "a233d58b-1fd3-40d7-a983-d35bbec45313";
const job = { id, commandKey: id, query: { site: "us", mode: "asin", asins: ["B000123456"], limit: 1, fields: ["asin", "title"] }, state: "ADMITTED", discovered: false, canceled: false, saved: 0, failed: 0, pending: 0, confirmedFen: 0, pendingFen: 0, createdAt: "2026-10-10T00:00:00Z", deadline: "2026-10-10T00:30:00Z" };
const json = (value: unknown, status = 200) => Response.json(value, { status });
afterEach(() => { cleanup(); sessionStorage.clear(); vi.unstubAllGlobals(); });
describe("data service product paths", () => {
    it.each(["2026-10-20T15:00:00Z", new Date(Date.now() + 3600000).toISOString()])("preserves the exact existing expiry %s when only limits change", async expiresAt => {
        const key = { id, suffix: "test", state: "ACTIVE", revision: 1, createdAt: job.createdAt, limits: { name: "fixture", expiresAt, dailyRows: 10, monthlyCostFen: 100, permissions: ["amazon.acquire", "amazon.result.read"] } };
        const mutations: unknown[] = [];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method === "POST") {
                const body = JSON.parse(String(init.body));
                mutations.push(body);
                return json({ ...key, limits: body.patch.limits, revision: 2 });
            }
            void url;
            return json({ options, keys: [key], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } });
        }));
        render(<DataServicesPage mode="api"/>);
        fireEvent.click(await screen.findByRole("button", { name: "管理密钥" }));
        fireEvent.click(screen.getByRole("button", { name: "编辑" }));
        fireEvent.change(screen.getByLabelText("每日最大成功条数"), { target: { value: "20" } });
        fireEvent.click(screen.getByRole("button", { name: "保存限制" }));
        await waitFor(() => expect(mutations).toHaveLength(1));
        expect(mutations[0]).toMatchObject({ patch: { limits: { expiresAt, dailyRows: 20 } } });
    });
    it("rejects a key name exceeding the existing 80 UTF-8 bytes before dispatch", async () => {
        vi.stubGlobal("fetch", vi.fn(async () => json({ options, keys: [], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } })));
        render(<DataServicesPage mode="api"/>);
        const create = await screen.findByRole("button", { name: "创建API密钥" });
        await waitFor(() => expect(create).toBeEnabled());
        fireEvent.click(create);
        const field = screen.getByLabelText("密钥名称");
        fireEvent.change(field, { target: { value: "中".repeat(27) } });
        expect(field).toHaveAttribute("aria-invalid", "true");
        expect(screen.getByRole("button", { name: "创建密钥" })).toBeDisabled();
        fireEvent.change(field, { target: { value: "中".repeat(26) } });
        expect(field).toHaveAttribute("aria-invalid", "false");
        expect(screen.getByRole("button", { name: "创建密钥" })).toBeEnabled();
        expect(sessionStorage.length).toBe(0);
    });
    it.each([["进度说明", 2000], ["已确认的规格", 4000], ["线下报价记录", 2000], ["线下确认记录", 2000]])("enforces specialist UTF-8 limits for %s", async (label, maximum) => {
        const request = { id, input: { name: "specialist fixture", query: job.query, purpose: "fixture", format: "json" }, state: "EVALUATING", revision: 1, specRevision: 0, deliveredRows: 0, createdAt: job.createdAt, events: [], applicant: { organizationId: "org", actorId: "user" } };
        const fetcher = vi.fn(async (url: string, init?: RequestInit) => { void init; return json(url.endsWith(id) ? request : { items: [{ id, name: request.input.name, site: "us", mode: "asin", state: "EVALUATING", createdAt: job.createdAt, applicant: request.applicant }] }); });
        vi.stubGlobal("fetch", fetcher);
        render(<SpecialistPage userId="user"/>);
        fireEvent.click(await screen.findByRole("button", { name: /specialist fixture/ }));
        const field = await screen.findByLabelText(label);
        expect(field).toHaveAttribute("maxlength", String(maximum));
        fireEvent.change(field, { target: { value: "中".repeat(Math.floor(Number(maximum) / 3) + 1) } });
        expect(field).toHaveAttribute("aria-invalid", "true");
        expect(screen.getByRole("button", { name: "保存进度与确认记录" })).toBeDisabled();
        fireEvent.change(field, { target: { value: "中".repeat(Math.floor(Number(maximum) / 3)) } });
        expect(field).toHaveAttribute("aria-invalid", "false");
        expect(screen.getByRole("button", { name: "保存进度与确认记录" })).toBeEnabled();
        expect(fetcher.mock.calls.every(([, init]) => !init || init.method !== "POST")).toBe(true);
        expect(sessionStorage.length).toBe(0);
    });
    it.each([
        ["需求名称", 200], ["用途与业务场景", 1000], ["时间范围／更新需求", 1000], ["其他说明", 4000],
    ])("enforces the accepted UTF-8 byte limit for %s", async (label, maximum) => {
        const fetcher = vi.fn(async (url: string, init?: RequestInit) => { void init; return json(url.endsWith("options") ? options : []); });
        vi.stubGlobal("fetch", fetcher);
        render(<DataServicesPage mode="market"/>);
        const start = await screen.findByRole("button", { name: "提交定制需求" });
        await waitFor(() => expect(start).toBeEnabled());
        fireEvent.click(start);
        const field = screen.getByLabelText(label);
        expect(field).toHaveAttribute("maxlength", String(maximum));
        fireEvent.change(field, { target: { value: "中".repeat(Math.floor(Number(maximum) / 3) + 1) } });
        expect(field).toHaveAttribute("aria-invalid", "true");
        expect(within(screen.getByRole("dialog")).getByRole("button", { name: "提交定制需求" })).toBeDisabled();
        fireEvent.change(field, { target: { value: "中".repeat(Math.floor(Number(maximum) / 3)) } });
        expect(field).toHaveAttribute("aria-invalid", "false");
        expect(within(screen.getByRole("dialog")).getByRole("button", { name: "提交定制需求" })).toBeEnabled();
        expect(fetcher.mock.calls.every(([, init]) => !init || init.method !== "POST")).toBe(true);
        expect(sessionStorage.length).toBe(0);
    });
    it.each([
        ["provider_challenged", "Amazon 页面要求验证，本次获取已停止。"],
        ["provider_unsupported", "Amazon 页面结构暂不支持，本次获取已停止。"],
        ["provider_rejected", "来源页面未能取得有效数据"],
    ])("shows the persisted item failure reason %s", async (reason, label) => {
        const failed = { ...job, state: "FAILED", discovered: true, failed: 1 };
        vi.stubGlobal("fetch", vi.fn(async (url: string) => {
            if (url.endsWith("/results"))
                return json({ items: [{ id, state: "FAILED", reason }] });
            if (url.endsWith(id))
                return json(failed);
            return json({ options, keys: [], jobs: [failed], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 1, succeededJobs: 0, successRate: 0, window: "UTC calendar month; completed jobs" } });
        }));
        render(<DataServicesPage mode="api"/>);
        fireEvent.click(await screen.findByRole("button", { name: "查看" }));
        expect(await screen.findByText(`失败 · ${label}`)).toBeInTheDocument();
        expect(screen.queryByText("失败 · 来源未完成")).not.toBeInTheDocument();
    });
    it("shows the original customization criteria for the customer and specialist", () => {
        render(<CustomDetails request={{ id, input: { name: "fixture", query: { site: "us", mode: "keyword", keyword: "cordless drill", categoryNode: "12345", asins: ["B000123456"], limit: 5, fields: ["title", "price"] }, purpose: "选品分析", timeRange: "最近三个月", format: "json", notes: "只需品牌官方店" }, state: "SUBMITTED", revision: 1, specRevision: 0, deliveredRows: 0, createdAt: "2026-10-10T00:00:00Z", events: [] }}/>);
        for (const value of ["cordless drill", "12345", "B000123456", "title、price", "最近三个月", "只需品牌官方店"])
            expect(screen.getByText(value)).toBeInTheDocument();
    });
    it("reports invalid customization input before storing or dispatching a command", async () => {
        const fetcher = vi.fn(async (url: string, init?: RequestInit) => { void init; return json(url.endsWith("options") ? options : []); });
        vi.stubGlobal("fetch", fetcher);
        render(<DataServicesPage mode="market"/>);
        const start = await screen.findByRole("button", { name: "提交定制需求" });
        await waitFor(() => expect(start).toBeEnabled());
        fireEvent.click(start);
        fireEvent.change(screen.getByLabelText("需求名称"), { target: { value: "数".repeat(70) } });
        fireEvent.change(screen.getByLabelText(/ASIN 或当前站点/), { target: { value: "B000123456" } });
        fireEvent.change(screen.getByLabelText("用途与业务场景"), { target: { value: "选品分析" } });
        expect(within(screen.getByRole("dialog")).getByRole("button", { name: "提交定制需求" })).toBeDisabled();
        expect(screen.getByText("超出字节上限，请缩短内容。")).toBeInTheDocument();
        expect(fetcher.mock.calls.every(([, init]) => !init || (init as RequestInit).method !== "POST")).toBe(true);
        expect(sessionStorage.length).toBe(0);
    });
    it("recovers UNKNOWN with the original body and UUID after form inputs change", async () => {
        const mutations: {
            body: unknown;
            key: string | null;
        }[] = [];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method === "POST") {
                mutations.push({ body: JSON.parse(String(init.body)), key: new Headers(init.headers).get("Idempotency-Key") });
                if (mutations.length === 1)
                    throw Error("lost response");
                return json(job, 202);
            }
            if (url.endsWith("options"))
                return json(options);
            if (url.includes("by-command"))
                return json({ error: { code: "DATA_NOT_FOUND" } }, 404);
            if (url.endsWith("/results"))
                return json({ items: [] });
            if (url.endsWith(id))
                return json(job);
            return json([]);
        }));
        render(<DataServicesPage mode="market"/>);
        const start = await screen.findByRole("button", { name: /开始抓取/ });
        await waitFor(() => expect(start).toBeEnabled());
        fireEvent.click(start);
        fireEvent.change(screen.getByLabelText(/ASIN 或当前站点/), { target: { value: "B000123456" } });
        fireEvent.change(screen.getByLabelText("最大数据条数"), { target: { value: "1" } });
        fireEvent.click(screen.getByRole("button", { name: "确认并开始抓取" }));
        await screen.findByText(/结果待核实，请继续核实原请求/);
        fireEvent.change(screen.getByLabelText("最大数据条数"), { target: { value: "20" } });
        fireEvent.click(screen.getByRole("button", { name: "核实／继续原请求" }));
        await waitFor(() => expect(mutations).toHaveLength(2));
        expect(mutations[1]).toEqual(mutations[0]);
        expect(mutations[1].body).toMatchObject({ maximumRows: 1, maximumCostFen: 5 });
        await waitFor(() => expect(sessionStorage.length).toBe(0));
    });
    it("shows null success sample and separates confirmed fees from pending", async () => {
        vi.stubGlobal("fetch", vi.fn(async () => json({ options, keys: [], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 5, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC calendar month; completed jobs" } })));
        render(<DataServicesPage mode="api"/>);
        await screen.findByText("暂无样本");
        expect(screen.getByText("已保存待确认 ¥0.05")).toBeInTheDocument();
        expect(screen.queryByText("1,142")).not.toBeInTheDocument();
    });
});
