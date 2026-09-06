import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { ProductTitleProposal, ProductTitleProposalList } from "@/lib/api/product-title-review";
import { ProductTitleReviewError } from "@/lib/api/product-title-review-client";
import { titleProposalFixture } from "@/test/product-title-review-fixture";
import { PendingTitleReviewPageContent } from "./pending-title-review-page";

const calls = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), decide: vi.fn(), apply: vi.fn(), context: {} as Record<string, unknown> }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => calls.context }));
vi.mock("@/lib/api/product-title-review-client", async (original) => ({ ...await original<object>(), fetchProductTitleProposals: calls.list, fetchProductTitleProposal: calls.get, decideProductTitleProposal: calls.decide, applyProductTitleProposal: calls.apply }));
let client: QueryClient;
const proposal = (state: ProductTitleProposal["state"] = "pending", revision = "1"): ProductTitleProposal => ({ ...titleProposalFixture(), state, revision, input: { product_key: "product", base_version: "9007199254740993" } });
const collection = (): ProductTitleProposalList => ({ schema_version: 1, coverage: "product-title-proposals-only", items: [{ proposal_id: proposal().proposal_id, product_key: "product", base_version: "9007199254740993", proposal_revision: "1", state: "pending" }], next_cursor: null });
const tree = (available = true, initialProposalId?: string) => <QueryClientProvider client={client}><PendingTitleReviewPageContent available={available} initialProposalId={initialProposalId} /></QueryClientProvider>;
const failure = (code: string, outcome: "unknown" | "rejected" = "rejected", status = 409) => new ProductTitleReviewError(status, code, { code, message: "private raw details", requestId: "", fieldErrors: [] }, outcome);
function deferred<T>() { let resolve!: (value: T) => void, reject!: (error: unknown) => void; const promise = new Promise<T>((r, j) => { resolve = r; reject = j; }); return { promise, resolve, reject }; }
beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  calls.context = { user: { id: "owner" }, effectiveOrganization: { id: "200" }, roles: ["listingkit_admin"], retry: vi.fn() };
  calls.list.mockReset().mockResolvedValue(collection()); calls.get.mockReset().mockResolvedValue(proposal());
  calls.decide.mockReset().mockResolvedValue(proposal("accepted", "2")); calls.apply.mockReset();
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value: function (this: HTMLDialogElement) { this.open = true; } });
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value: function (this: HTMLDialogElement) { this.open = false; } });
});
afterEach(() => { cleanup(); client.clear(); });
async function open() { render(tree()); await userEvent.click(await screen.findByRole("button", { name: /product.*查看详情/ })); await screen.findByText("Original title"); }

it("discovers the real collection, GETs only the selected proposal and keeps exact versions", async () => {
  await open();
  expect(calls.list).toHaveBeenCalledOnce(); expect(calls.get).toHaveBeenCalledOnce();
  expect(calls.get.mock.calls[0][0]).toMatchObject({ organizationId: "200", proposalId: proposal().proposal_id });
  expect(screen.getAllByText("9007199254740993").length).toBeGreaterThan(0);
  expect(screen.getByText("sha256:sample")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /sha256|evidence/ })).not.toBeInTheDocument();
});

it("accept never applies; only separate confirmation sends the newly accepted exact revision", async () => {
  await open(); await userEvent.click(screen.getByRole("button", { name: "接受提案" }));
  await screen.findByRole("button", { name: "应用到标准商品" });
  expect(calls.apply).not.toHaveBeenCalled();
  expect(calls.decide.mock.calls[0][0].input).toEqual({ action: "accept", expected_revision: "1" });
  await userEvent.click(screen.getByRole("button", { name: "应用到标准商品" }));
  expect(calls.apply).not.toHaveBeenCalled();
  const applied = { ...proposal("applied", "2"), apply_receipt: { proposal_id: proposal().proposal_id, revision: "2", product_version: "9007199254740994", publication_id: "real-receipt-reference", actor: "owner", at: "2026-09-07T00:00:00Z" } };
  calls.apply.mockResolvedValue(applied); calls.list.mockResolvedValue({ ...collection(), items: [] });
  await userEvent.click(screen.getByRole("button", { name: "确认应用" }));
  expect(await screen.findByText("9007199254740994")).toBeVisible();
  expect(calls.apply.mock.calls[0][0].input).toEqual({ expected_revision: "2" });
  expect(calls.apply.mock.calls[0][0].idempotencyKey).not.toBe(calls.decide.mock.calls[0][0].idempotencyKey);
  expect(screen.queryByRole("button", { name: "应用到标准商品" })).not.toBeInTheDocument();
});

it("editing accepted data hides Apply and saves a new pending revision without approval", async () => {
  calls.get.mockResolvedValue(proposal("accepted", "8")); calls.decide.mockResolvedValue({ ...proposal("pending", "9"), after: "人工标题" });
  await open(); await userEvent.click(screen.getByRole("button", { name: "编辑标题" }));
  expect(screen.queryByRole("button", { name: "应用到标准商品" })).not.toBeInTheDocument();
  await userEvent.clear(screen.getByRole("textbox", { name: "编辑标题" })); await userEvent.type(screen.getByRole("textbox", { name: "编辑标题" }), "人工标题");
  await userEvent.click(screen.getByRole("button", { name: "保存为待审核提案" }));
  await screen.findByRole("button", { name: "接受提案" });
  expect(calls.decide.mock.calls[0][0].input).toEqual({ action: "edit", expected_revision: "8", title: "人工标题" });
  expect(calls.apply).not.toHaveBeenCalled();
});

it("double click dispatches once; unknown writes stay unknown after GET without receipt and replay only explicitly", async () => {
  const late = deferred<ProductTitleProposal>(); calls.decide.mockReturnValueOnce(late.promise).mockResolvedValue(proposal("accepted", "2"));
  await open(); await userEvent.dblClick(screen.getByRole("button", { name: "接受提案" }));
  expect(calls.decide).toHaveBeenCalledOnce();
  await act(async () => late.reject(failure("RESULT_UNVERIFIED", "unknown", 502)));
  expect(await screen.findByText("结果待核实")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "重新读取当前状态" }));
  await screen.findByText("Original title");
  expect(screen.getByText("结果待核实")).toBeVisible(); expect(calls.decide).toHaveBeenCalledOnce();
  calls.get.mockResolvedValue(proposal("accepted", "2"));
  await userEvent.click(screen.getByRole("button", { name: "核实本次操作" }));
  await screen.findByRole("button", { name: "应用到标准商品" });
  const [first, second] = calls.decide.mock.calls.map(([input]) => input);
  expect(second).toMatchObject({ idempotencyKey: first.idempotencyKey, input: first.input, proposalId: first.proposalId, organizationId: first.organizationId });
});

it.each(["stale_product_version", "operation_conflict"])("fails closed on %s without replacing the base or replaying", async (code) => {
  calls.decide.mockRejectedValue(failure(code)); await open();
  await userEvent.click(screen.getByRole("button", { name: "接受提案" }));
  expect(await screen.findByText(/资料已变化，请重新读取/)).toBeVisible();
  expect(screen.queryByRole("button", { name: "接受提案" })).not.toBeInTheDocument();
  expect(calls.decide).toHaveBeenCalledOnce(); expect(screen.queryByText("private raw details")).not.toBeInTheDocument();
});

it.each(["organization", "user", "roles", "switching", "revoked", "logout", "unmount"])("clears a pending write and ignores late success on %s", async (kind) => {
  const late = deferred<ProductTitleProposal>(); calls.decide.mockReturnValue(late.promise);
  const view = render(tree()); await userEvent.click(await screen.findByRole("button", { name: /product.*查看详情/ }));
  await userEvent.click(await screen.findByRole("button", { name: "接受提案" }));
  const signal = calls.decide.mock.calls[0][0].signal;
  if (kind === "organization") calls.context.effectiveOrganization = { id: "100" };
  if (kind === "user") calls.context.user = { id: "other" };
  if (kind === "roles") calls.context.roles = ["listingkit_operator"];
  if (kind === "switching") calls.context.isSwitching = true;
  if (kind === "revoked") calls.context.blockingError = { code: "ORGANIZATION_ACCESS_REVOKED" };
  if (kind === "logout") calls.context.user = null;
  if (kind === "unmount") view.unmount(); else view.rerender(tree());
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve(proposal("accepted", "2")));
  expect(screen.queryByText("Original title")).not.toBeInTheDocument(); expect(calls.apply).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "应用到标准商品" })).not.toBeInTheDocument();
});

it("clears sensitive data on write denial and never falls back to admin", async () => {
  calls.decide.mockRejectedValue(failure("PERMISSION_DENIED", "rejected", 403)); await open();
  await userEvent.click(screen.getByRole("button", { name: "接受提案" }));
  expect(await screen.findByText("当前身份没有标题审核权限")).toBeVisible();
  expect(screen.queryByText("Original title")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /接受提案|编辑标题|应用到标准商品/ })).not.toBeInTheDocument();
});

it("operator can edit their own proposal but cannot accept, reject or apply", async () => {
  calls.context.roles = ["listingkit_operator"]; await open();
  expect(screen.getByRole("button", { name: "编辑标题" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: /接受提案|拒绝提案|应用到标准商品/ })).not.toBeInTheDocument();
});

it("unconfigured page never fetches and URL selection is a real authorized GET even outside the list", async () => {
  const view = render(tree(false)); expect(calls.list).not.toHaveBeenCalled();
  view.unmount(); calls.list.mockResolvedValue({ ...collection(), items: [] }); calls.get.mockResolvedValue(proposal("rejected", "3"));
  render(tree(true, proposal().proposal_id)); await screen.findByText("Original title");
  expect(calls.get).toHaveBeenCalledOnce();
  expect(screen.queryByRole("button", { name: "接受提案" })).not.toBeInTheDocument();
});

it("restores an initial URL selection after context loading, but never resurrects it after a scope transition", async () => {
  calls.context.isLoading = true;
  const view = render(tree(true, proposal().proposal_id));
  calls.context.isLoading = false; view.rerender(tree(true, proposal().proposal_id));
  await screen.findByText("Original title");
  calls.context.isSwitching = true; view.rerender(tree(true, proposal().proposal_id));
  calls.context.isSwitching = false; view.rerender(tree(true, proposal().proposal_id));
  await screen.findByText("选择一条标题提案");
  expect(calls.get).toHaveBeenCalledOnce();
});

it("a detail read denial clears the whole sensitive review projection", async () => {
  await open(); calls.get.mockRejectedValue(failure("PERMISSION_DENIED", "rejected", 403));
  await userEvent.click(screen.getByRole("button", { name: "重新读取当前状态" }));
  await screen.findByText("当前身份没有标题审核权限");
  expect(screen.queryByRole("button", { name: /product.*查看详情/ })).not.toBeInTheDocument();
});

it("authoritative rereads replace an accepted response and close its old revision confirmation", async () => {
  await open(); await userEvent.click(screen.getByRole("button", { name: "接受提案" }));
  await userEvent.click(await screen.findByRole("button", { name: "应用到标准商品" }));
  expect(screen.getByRole("dialog")).toBeVisible();
  calls.get.mockResolvedValue({ ...proposal("pending", "3"), after: "另一审核者修改" });
  await act(async () => client.invalidateQueries({ predicate: (q) => q.queryKey.includes("title-proposal") }));
  await screen.findByText("另一审核者修改");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "应用到标准商品" })).not.toBeInTheDocument();
  expect(calls.apply).not.toHaveBeenCalled();
});

it("stopping a dispatched write retains uncertainty even if explicit verification cannot send", async () => {
  calls.decide.mockImplementationOnce(({ signal }: { signal: AbortSignal }) => new Promise((_, reject) => {
    signal.addEventListener("abort", () => reject(failure("RESULT_UNVERIFIED", "unknown", 502)));
  })).mockRejectedValueOnce(new ProductTitleReviewError(400, "INVALID_REQUEST", { code: "INVALID_REQUEST", message: "", requestId: "", fieldErrors: [] }, "not_sent"));
  await open(); await userEvent.click(screen.getByRole("button", { name: "接受提案" }));
  await userEvent.click(screen.getByRole("button", { name: "停止等待" }));
  await screen.findByText("结果待核实");
  await userEvent.click(screen.getByRole("button", { name: "核实本次操作" }));
  expect(await screen.findByText("结果待核实")).toBeVisible();
  expect(screen.queryByRole("button", { name: "接受提案" })).not.toBeInTheDocument();
  expect(calls.decide.mock.calls[1][0].idempotencyKey).toBe(calls.decide.mock.calls[0][0].idempotencyKey);
});

it("a rejected proposal leaves the actionable list but keeps its authorized terminal detail", async () => {
  calls.decide.mockResolvedValue(proposal("rejected", "2")); await open();
  calls.list.mockResolvedValue({ ...collection(), items: [] });
  await userEvent.click(screen.getByRole("button", { name: "拒绝提案" }));
  await screen.findByText("当前授权范围内暂无待处理标题提案");
  expect(screen.getByText("Original title")).toBeVisible();
  expect(screen.getByText("已拒绝")).toBeVisible();
  expect(calls.decide.mock.calls[0][0].input).toEqual({ action: "reject", expected_revision: "1" });
  expect(calls.apply).not.toHaveBeenCalled();
});

it("pagination forwards the opaque cursor and clears details; refresh resets to the first page", async () => {
  calls.list.mockResolvedValue({ ...collection(), next_cursor: "opaque-cursor" }); await open();
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("第 2 页");
  expect(calls.list.mock.calls.at(-1)![0].cursor).toBe("opaque-cursor");
  expect(screen.queryByText("Original title")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: /刷新/ }));
  await screen.findByText("第 1 页");
  expect(calls.list.mock.calls.at(-1)![0].cursor).toBeUndefined();
});

it("a late write failure cannot replace a new organization's review page", async () => {
  const late = deferred<ProductTitleProposal>(); calls.decide.mockReturnValue(late.promise);
  const view = render(tree()); await userEvent.click(await screen.findByRole("button", { name: /product.*查看详情/ }));
  await userEvent.click(await screen.findByRole("button", { name: "接受提案" }));
  calls.context.effectiveOrganization = { id: "100" }; view.rerender(tree());
  await act(async () => late.reject(failure("PERMISSION_DENIED", "rejected", 403)));
  expect(screen.queryByText("当前身份没有标题审核权限")).not.toBeInTheDocument();
  expect(screen.queryByText("结果待核实")).not.toBeInTheDocument();
  expect(await screen.findByText("选择一条标题提案")).toBeVisible();
});

it("a late collection read failure preserves an unknown write and its exact verification intent", async () => {
  const list = deferred<ProductTitleProposalList>(); calls.list.mockReturnValue(list.promise);
  calls.decide.mockRejectedValue(failure("RESULT_UNVERIFIED", "unknown", 502));
  render(tree(true, proposal().proposal_id));
  await userEvent.click(await screen.findByRole("button", { name: "接受提案" }));
  await screen.findByText("结果待核实");
  await act(async () => list.reject(failure("DEPENDENCY_UNAVAILABLE", "rejected", 503)));
  await screen.findByText("标题提案服务暂不可用");
  expect(screen.getByText("结果待核实")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "核实本次操作" }));
  expect(calls.decide.mock.calls[1][0].idempotencyKey).toBe(calls.decide.mock.calls[0][0].idempotencyKey);
});

it("collection permission denial clears an in-flight operation and releases the page lock", async () => {
  const list = deferred<ProductTitleProposalList>(); calls.list.mockReturnValue(list.promise);
  const write = deferred<ProductTitleProposal>(); calls.decide.mockReturnValue(write.promise);
  render(tree(true, proposal().proposal_id));
  await userEvent.click(await screen.findByRole("button", { name: "接受提案" }));
  await act(async () => list.reject(failure("PERMISSION_DENIED", "rejected", 403)));
  await screen.findByText("当前身份没有标题审核权限");
  expect(calls.decide.mock.calls[0][0].signal.aborted).toBe(true);
  expect(screen.queryByText("Original title")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /刷新/ })).toBeEnabled();
  await act(async () => write.resolve(proposal("accepted", "2")));
  expect(screen.queryByRole("button", { name: "应用到标准商品" })).not.toBeInTheDocument();
});

it("shows the handed-off 413 not_sent error without an unknown-result replay", async () => {
  calls.decide.mockRejectedValue(new ProductTitleReviewError(413, "INPUT_TOO_LARGE", { code: "INPUT_TOO_LARGE", message: "private", requestId: "", fieldErrors: [] }, "not_sent"));
  await open(); await userEvent.click(screen.getByRole("button", { name: "编辑标题" }));
  await userEvent.click(screen.getByRole("button", { name: "保存为待审核提案" }));
  expect(await screen.findByText("标题或请求内容过长")).toBeVisible();
  expect(screen.queryByText("结果待核实")).not.toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: "编辑标题" })).toBeEnabled();
});
