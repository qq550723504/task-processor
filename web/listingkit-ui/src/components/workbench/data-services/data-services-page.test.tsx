import { render, screen, fireEvent, waitFor, cleanup, within } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { DataServicesPage } from "./data-services-page";
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
        fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "提交定制需求" }));
        await screen.findByText("请检查输入、格式、条数和额度。");
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
