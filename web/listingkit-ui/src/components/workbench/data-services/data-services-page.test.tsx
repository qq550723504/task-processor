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
    it.each([
        [79, 0, 100, false], [60, 20, 100, true], [80, 0, 100, true], [0, 80, 100, true], [90, 10, 100, true],
        [60, 20, 101, false], [0, 8, 10, true],
    ])("warns at 80 percent from actual per-key occupied budget %s + %s / %s", async (consumed, reserved, limit, warning) => {
        const key = { id, suffix: "test", state: "ACTIVE", revision: 1, createdAt: job.createdAt, limits: { name: "实际限额密钥", expiresAt: new Date(Date.now() + 3600000).toISOString(), dailyRows: 10, monthlyCostFen: Number(limit), permissions: ["amazon.acquire", "amazon.result.read"] } };
        const overview = { options, keys: [key], jobs: [], keyQuotas: [{ keyId: id, dayConsumedRows: 0, dayReservedRows: 0, monthConsumedFen: Number(consumed), monthReservedFen: Number(reserved) }], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } };
        vi.stubGlobal("fetch", vi.fn(async (url: string) => json(url.endsWith("/keys") ? [key] : overview)));
        render(<DataServicesPage mode="api"/>);
        await screen.findByRole("progressbar", { name: "该密钥月预算占用" });
        const checkWarning = () => {
            const alert = screen.queryByRole("alert");
            if (warning) {
                expect(alert).toHaveTextContent("实际限额密钥");
                expect(alert).toHaveTextContent("80%");
                expect(alert).toHaveTextContent("计量与预留");
                expect(alert).not.toHaveTextContent("通知已发送");
            } else expect(alert).not.toBeInTheDocument();
        };
        checkWarning();
        fireEvent.click(screen.getByRole("button", { name: "用量与费用" }));
        await screen.findByText("月预算已计量", { exact: false });
        checkWarning();
    });
    it("changes the warning with the selected key and never invents missing quota", async () => {
        const keys = [id, "b233d58b-1fd3-40d7-a983-d35bbec45313", "c233d58b-1fd3-40d7-a983-d35bbec45313"].map((keyId, i) => ({ id: keyId, suffix: "test", state: "ACTIVE", revision: 1, createdAt: job.createdAt, limits: { name: ["高占用密钥", "低占用密钥", "未读到额度密钥"][i], expiresAt: new Date(Date.now() + 3600000).toISOString(), dailyRows: 10, monthlyCostFen: [100, 200, 10][i], permissions: ["amazon.acquire", "amazon.result.read"] } }));
        const overview = { options, keys, jobs: [], keyQuotas: keys.slice(0, 2).map(key => ({ keyId: key.id, dayConsumedRows: 0, dayReservedRows: 0, monthConsumedFen: 60, monthReservedFen: 20 })), usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } };
        vi.stubGlobal("fetch", vi.fn(async (url: string) => json(url.endsWith("/keys") ? keys : overview)));
        render(<DataServicesPage mode="api"/>);
        const selector = await screen.findByRole("combobox", { name: "额度所属密钥" });
        expect(screen.getByRole("alert")).toHaveTextContent("高占用密钥");
        fireEvent.change(selector, { target: { value: keys[1].id } });
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        fireEvent.change(selector, { target: { value: keys[2].id } });
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        fireEvent.change(screen.getByRole("combobox", { name: "额度所属密钥" }), { target: { value: keys[0].id } });
        expect(screen.getByRole("alert")).toHaveTextContent("高占用密钥");
        fireEvent.click(screen.getByRole("button", { name: "用量与费用" }));
        expect(screen.getAllByRole("alert")).toHaveLength(1);
        expect(screen.getByRole("alert")).toHaveTextContent("高占用密钥");
    });
    it.each([
        ["amazon/jobs", 403], ["amazon/jobs", 503], ["custom", 403], ["custom", 503],
    ])("submits both market flows independently of %s history failure %s", async (historyPath, status) => {
        const mutations: { path: string; body: unknown; key: string | null }[] = [];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method === "POST") {
                const body = JSON.parse(String(init.body));
                const key = new Headers(init.headers).get("Idempotency-Key");
                mutations.push({ path: url, body, key });
                expect(new Headers(init.headers).get("X-Expected-User-ID")).toBe("user");
                expect(new Headers(init.headers).get("X-Expected-Organization-ID")).toBe("org");
                return url.endsWith("custom")
                    ? json({ id, input: body, state: "SUBMITTED", revision: 1, specRevision: 0, deliveredRows: 0, createdAt: job.createdAt, events: [] })
                    : json({ ...job, commandKey: key }, 202);
            }
            if (url.endsWith("/options")) return json(options);
            if (url.endsWith(`/${historyPath}`)) return json({ error: { code: status === 403 ? "FORBIDDEN" : "DATA_UNAVAILABLE" } }, Number(status));
            if (url.endsWith("/results")) return json({ items: [] });
            if (url.endsWith(id)) return json(job);
            return json([]);
        }));
        render(<DataServicesPage mode="market"/>);
        const start = screen.getByRole("button", { name: "开始抓取" });
        await waitFor(() => expect(start).toBeEnabled());
        expect(screen.getByRole("button", { name: "提交定制需求" })).toBeEnabled();
        const historyName = historyPath === "custom" ? "定制历史" : "抓取历史";
        expect(await screen.findByText(status === 403 ? `当前身份没有${historyName}读取权限。` : `${historyName}暂不可用，请稍后刷新。`)).toBeInTheDocument();
        expect(screen.queryByText("暂无记录，提交后会在这里显示真实进度。")).not.toBeInTheDocument();
        fireEvent.click(start);
        fireEvent.change(screen.getByLabelText(/ASIN 或当前站点/), { target: { value: "B000123456" } });
        fireEvent.change(screen.getByLabelText("最大数据条数"), { target: { value: "1" } });
        fireEvent.click(screen.getByRole("button", { name: "确认并开始抓取" }));
        const result = await screen.findByRole("dialog", { name: "抓取任务与结果" });
        fireEvent.click(within(result).getByRole("button", { name: "关闭" }));
        const custom = screen.getByRole("button", { name: "提交定制需求" });
        await waitFor(() => expect(custom).toBeEnabled());
        fireEvent.click(custom);
        fireEvent.change(screen.getByLabelText("需求名称"), { target: { value: "history-independent request" } });
        fireEvent.change(screen.getByLabelText("用途与业务场景"), { target: { value: "选品分析" } });
        fireEvent.change(screen.getByLabelText(/ASIN 或当前站点/), { target: { value: "B000123456" } });
        fireEvent.change(screen.getByLabelText("最大数据条数"), { target: { value: "1" } });
        fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "提交定制需求" }));
        await waitFor(() => expect(mutations).toHaveLength(2));
        expect(mutations[0]).toMatchObject({ body: { maximumRows: 1, maximumCostFen: 5, query: { asins: ["B000123456"] } } });
        expect(mutations[1]).toMatchObject({ body: { name: "history-independent request", purpose: "选品分析", query: { asins: ["B000123456"], limit: 1 } } });
        expect(mutations[0].key).toMatch(/^[a-f0-9-]{36}$/);
        expect(mutations[1].key).not.toBe(mutations[0].key);
        await waitFor(() => expect(sessionStorage.length).toBe(0));
    });
    it("keeps market submission closed when options fail despite readable histories", async () => {
        vi.stubGlobal("fetch", vi.fn(async (url: string) => url.endsWith("/options") ? json({ error: { code: "FORBIDDEN" } }, 403) : json([])));
        render(<DataServicesPage mode="market"/>);
        await screen.findByText("当前身份或权限已失效，请确认登录及企业。");
        expect(screen.getByRole("button", { name: "开始抓取" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "提交定制需求" })).toBeDisabled();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    it.each([403, 503])("keeps key management usable when optional overview returns %s", async status => {
        const expiresAt = new Date(Date.now() + 3600000).toISOString();
        let key = { id, suffix: "test", state: "ACTIVE", revision: 1, createdAt: job.createdAt, limits: { name: "manage fixture", expiresAt, dailyRows: 10, monthlyCostFen: 100, permissions: ["amazon.acquire", "amazon.result.read"] } };
        const mutations: unknown[] = [];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method === "POST") {
                const body = JSON.parse(String(init.body));
                mutations.push(body);
                key = { ...key, state: body.patch.state ?? key.state, limits: body.patch.limits ?? key.limits, revision: key.revision + 1 };
                return json(key);
            }
            if (url.endsWith("/keys")) return json(key.state === "REVOKED" ? [] : [key]);
            if (url.endsWith("/overview")) return json({ error: { code: status === 403 ? "FORBIDDEN" : "DATA_UNAVAILABLE" } }, status);
            return json([]);
        }));
        render(<DataServicesPage mode="api"/>);
        const create = screen.getByRole("button", { name: "创建API密钥" });
        await waitFor(() => expect(create).toBeEnabled());
        fireEvent.click(create);
        expect(screen.getByRole("dialog", { name: "创建 API 密钥" })).toBeInTheDocument();
        fireEvent.click(screen.getByRole("button", { name: "关闭" }));
        fireEvent.click(screen.getByRole("button", { name: "API 密钥" }));
        expect(await screen.findByText("manage fixture")).toBeInTheDocument();
        expect(screen.getByText(/用量、费用与抓取配置暂未读取；密钥管理仍可使用。/)).toBeInTheDocument();
        expect(screen.queryByText("今日保存数据")).not.toBeInTheDocument();
        expect(screen.queryByText("暂无样本")).not.toBeInTheDocument();
        fireEvent.click(screen.getByRole("button", { name: "编辑" }));
        fireEvent.change(screen.getByLabelText("每日最大成功条数"), { target: { value: "20" } });
        fireEvent.click(screen.getByRole("button", { name: "保存限制" }));
        await waitFor(() => expect(mutations).toHaveLength(1));
        expect(mutations[0]).toMatchObject({ expectedRevision: 1, patch: { limits: { expiresAt, dailyRows: 20 } } });
        await screen.findByText("每日 20 条 · 月预算 ¥1.00");
        const action = status === 403 ? "撤销" : "禁用";
        fireEvent.click(await screen.findByRole("button", { name: action }));
        fireEvent.click(screen.getByRole("button", { name: status === 403 ? "确认已撤销" : "确认禁用" }));
        await waitFor(() => expect(mutations).toHaveLength(2));
        expect(mutations[1]).toMatchObject({ expectedRevision: 2, patch: { state: status === 403 ? "REVOKED" : "DISABLED" } });
        await waitFor(() => expect(sessionStorage.length).toBe(0));
    });
    it("does not grant key controls from overview when the independent key read fails", async () => {
        vi.stubGlobal("fetch", vi.fn(async (url: string) => url.endsWith("/keys")
            ? json({ error: { code: "FORBIDDEN" } }, 403)
            : json({ options, keys: [], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } })));
        render(<DataServicesPage mode="api"/>);
        await screen.findByText("当前身份或权限已失效，请确认登录及企业。");
        expect(screen.getByRole("button", { name: "创建API密钥" })).toBeDisabled();
        fireEvent.click(screen.getByRole("button", { name: "API 密钥" }));
        expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    it.each(["2026-10-20T15:00:00Z", new Date(Date.now() + 3600000).toISOString()])("preserves the exact existing expiry %s when only limits change", async expiresAt => {
        const key = { id, suffix: "test", state: "ACTIVE", revision: 1, createdAt: job.createdAt, limits: { name: "fixture", expiresAt, dailyRows: 10, monthlyCostFen: 100, permissions: ["amazon.acquire", "amazon.result.read"] } };
        const mutations: unknown[] = [];
        vi.stubGlobal("fetch", vi.fn(async (url: string, init?: RequestInit) => {
            if (init?.method === "POST") {
                const body = JSON.parse(String(init.body));
                mutations.push(body);
                return json({ ...key, limits: body.patch.limits, revision: 2 });
            }
            if (url.endsWith("/keys")) return json([key]);
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
        vi.stubGlobal("fetch", vi.fn(async (url: string) => json(url.endsWith("/keys") ? [] : { options, keys: [], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 0, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC month" } })));
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
            if (url.endsWith("/keys")) return json([]);
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
        vi.stubGlobal("fetch", vi.fn(async (url: string) => json(url.endsWith("/keys") ? [] : { options, keys: [], jobs: [], keyQuotas: [], usage: { dayRows: 0, monthConfirmedFen: 0, monthPendingFen: 5, finishedJobs: 0, succeededJobs: 0, successRate: null, window: "UTC calendar month; completed jobs" } })));
        render(<DataServicesPage mode="api"/>);
        await screen.findByText("暂无样本");
        expect(screen.getByText("已保存待确认 ¥0.05")).toBeInTheDocument();
        expect(screen.queryByText("1,142")).not.toBeInTheDocument();
    });
});
