import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
const context = vi.hoisted(() => ({
  user: { id: "actor-a" },
  effectiveOrganization: { id: "org-a", name: "企业 A" },
  permissions: [
    "workbench.store.products.read",
    "workbench.store.products.sync",
  ],
  isSwitching: false,
  retry: vi.fn(),
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => context,
}));
vi.mock("@/lib/api/workbench-stores", () => ({
  listWorkbenchStores: vi
    .fn()
    .mockResolvedValue({ items: [], pagination: { total: 0 } }),
}));
import { ObservationPage } from "./observation-page";
const id = "d6f6ca0a-27e2-4c4a-b1aa-4505110ae635";
const record = {
  storeId: id,
  syncId: id,
  id: "spu-a",
  observedAt: "2026-10-09T01:00:00Z",
  product: {
    id: "spu-a",
    skcs: [
      {
        id: "skc-a",
        sellerCode: "",
        title: "Sample Shoe",
        imageUrl: "",
        site: "shein-us",
        siteStatus: null,
        skus: [
          { id: "sku-a", sellerSku: "", prices: [], costs: [], inventory: [] },
        ],
      },
    ],
  },
};
afterEach(() => {
  vi.unstubAllGlobals();
  sessionStorage.clear();
});
beforeEach(() => {
  context.permissions = [
    "workbench.store.products.read",
    "workbench.store.products.sync",
  ];
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
    configurable: true,
    value: function (this: HTMLDialogElement) {
      this.open = true;
    },
  });
  Object.defineProperty(HTMLDialogElement.prototype, "close", {
    configurable: true,
    value: function (this: HTMLDialogElement) {
      this.open = false;
    },
  });
});
function renderPage(kind: "products" | "orders" = "products") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ObservationPage kind={kind} />
    </QueryClientProvider>,
  );
  return { ...view, client };
}
function baseFetch() {
  return vi.fn().mockImplementation((path: string, init?: RequestInit) => {
    if (path.endsWith("/capabilities"))
      return Promise.resolve(
        Response.json({
          organizationId: "org-a",
          userId: "actor-a",
          data: {
            available: true,
            canSync: true,
            kind: "products",
            site: "shein-us",
            platformUrl: "https://sellerhub.shein.com/",
          },
        }),
      );
    if (init?.method === "POST")
      return Promise.reject(new Error("lost response"));
    if (path.includes("/commands/"))
      return Promise.resolve(
        Response.json({ code: "NOT_FOUND" }, { status: 404 }),
      );
    return Promise.resolve(
      Response.json({
        organizationId: "org-a",
        userId: "actor-a",
        data: {
          items: [record],
          next: "",
          summary: {
            total: 1,
            active: 0,
            offShelf: 0,
            today: 0,
            todayUnknown: 0,
            pending: 0,
            transit: 0,
            exceptional: 0,
            unknown: 1,
          },
          syncs: [],
          latest: [],
          complete: false,
        },
      }),
    );
  });
}
it("shows readonly incomplete observations without invented stock or pending review counts", async () => {
  vi.stubGlobal("fetch", baseFetch());
  renderPage();
  expect(await screen.findByText("Sample Shoe")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "店铺商品" })).toBeInTheDocument();
  expect(screen.getAllByText("平台未提供").length).toBeGreaterThan(0);
  expect(screen.getByText(/覆盖不完整/)).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "去发货" }),
  ).not.toBeInTheDocument();
});
it("reuses the captured sync key after a lost response", async () => {
  const fetch = baseFetch();
  vi.stubGlobal("fetch", fetch);
  renderPage();
  await userEvent.click(
    await screen.findByRole("button", { name: "同步商品" }),
  );
  await screen.findByRole("button", { name: "重试原同步" });
  expect(
    screen.queryByRole("button", { name: "清除此页同步记录" }),
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "重试原同步" }));
  await waitFor(() =>
    expect(
      fetch.mock.calls.filter((c) => c[1]?.method === "POST"),
    ).toHaveLength(2),
  );
  const posts = fetch.mock.calls.filter((c) => c[1]?.method === "POST");
  expect(new Headers(posts[0][1].headers).get("Idempotency-Key")).toBe(
    new Headers(posts[1][1].headers).get("Idempotency-Key"),
  );
  expect(posts[0][1].body).toEqual(posts[1][1].body);
});
it.each([
  ["NOT_FOUND", 404],
  ["PERMISSION_DENIED", 403],
])("allows explicitly discarding an inaccessible saved receipt (%s)", async (code, status) => {
  const storageKey = "store-observations:org-a:actor-a:products";
  const saved = JSON.stringify({ key: id, input: { kind: "products", stores: [] } });
  sessionStorage.setItem(storageKey, saved);
  const fetch = baseFetch();
  const original = fetch.getMockImplementation()!;
  fetch.mockImplementation((path: string, init?: RequestInit) =>
    path.includes("/commands/")
      ? Promise.resolve(Response.json({ code }, { status }))
      : original(path, init),
  );
  vi.stubGlobal("fetch", fetch);
  renderPage();
  const discard = await screen.findByRole("button", { name: "清除此页同步记录" });
  expect(sessionStorage.getItem(storageKey)).toBe(saved);
  expect(screen.getByRole("button", { name: "同步商品" })).toBeDisabled();
  await userEvent.click(discard);
  expect(sessionStorage.getItem(storageKey)).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "同步商品" }));
  await waitFor(() => expect(fetch.mock.calls.filter((c) => c[1]?.method === "POST")).toHaveLength(1));
  const post = fetch.mock.calls.find((c) => c[1]?.method === "POST")!;
  expect(new Headers(post[1].headers).get("Idempotency-Key")).not.toBe(id);
  expect(JSON.parse(sessionStorage.getItem(storageKey)!).key).toBe(new Headers(post[1].headers).get("Idempotency-Key"));
});
it("keeps the saved key when receipt lookup has a transient dependency failure", async () => {
  const storageKey = "store-observations:org-a:actor-a:products";
  const saved = JSON.stringify({ key: id, input: { kind: "products", stores: [] } });
  sessionStorage.setItem(storageKey, saved);
  const fetch = baseFetch();
  const original = fetch.getMockImplementation()!;
  fetch.mockImplementation((path: string, init?: RequestInit) =>
    path.includes("/commands/")
      ? Promise.resolve(Response.json({ code: "DEPENDENCY_UNAVAILABLE" }, { status: 503 }))
      : original(path, init),
  );
  vi.stubGlobal("fetch", fetch);
  renderPage();
  await screen.findByRole("button", { name: "重试原同步" });
  expect(screen.queryByRole("button", { name: "清除此页同步记录" })).not.toBeInTheDocument();
  expect(sessionStorage.getItem(storageKey)).toBe(saved);
  await userEvent.click(screen.getByRole("button", { name: "重试原同步" }));
  await waitFor(() => expect(fetch.mock.calls.filter((c) => c[1]?.method === "POST")).toHaveLength(1));
  const post = fetch.mock.calls.find((c) => c[1]?.method === "POST")!;
  expect(new Headers(post[1].headers).get("Idempotency-Key")).toBe(id);
});
it.each([
  ["NOT_FOUND", 404],
  ["PERMISSION_DENIED", 403],
  ["DEPENDENCY_UNAVAILABLE", 503],
])("handles a failed refetch of an already cached pending receipt (%s)", async (code, status) => {
  const storageKey = "store-observations:org-a:actor-a:products";
  const saved = JSON.stringify({ key: id, input: { kind: "products", stores: [] } });
  sessionStorage.setItem(storageKey, saved);
  const receipt = {
    id,
    input: { kind: "products", stores: [id] },
    createdAt: "2026-10-09T01:00:00Z",
    syncs: [{
      id,
      commandId: id,
      storeId: id,
      kind: "products",
      status: "pending",
      progress: { page: 1, windows: [], expectedTotal: null, seen: 0, pages: 0, incomplete: false, notes: [] },
      range: null,
      createdAt: "2026-10-09T01:00:00Z",
      observedAt: null,
      errorCode: "",
    }],
  };
  let failed = false;
  const fetch = baseFetch();
  const original = fetch.getMockImplementation()!;
  fetch.mockImplementation((path: string, init?: RequestInit) =>
    path.includes("/commands/")
      ? Promise.resolve(failed
        ? Response.json({ code }, { status })
        : Response.json({ organizationId: "org-a", userId: "actor-a", data: receipt }))
      : original(path, init),
  );
  vi.stubGlobal("fetch", fetch);
  const view = renderPage();
  await screen.findByRole("button", { name: "继续同步" });
  failed = true;
  await view.client.refetchQueries({ queryKey: ["store-observations", "org-a", "actor-a", "products", "command", id] });
  expect(view.client.getQueryData(["store-observations", "org-a", "actor-a", "products", "command", id])).toEqual(receipt);
  if (status === 503) {
    expect(screen.getByRole("button", { name: "继续同步" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "清除此页同步记录" })).not.toBeInTheDocument();
    expect(sessionStorage.getItem(storageKey)).toBe(saved);
  } else {
    const discard = await screen.findByRole("button", { name: "清除此页同步记录" });
    expect(screen.queryByRole("button", { name: "继续同步" })).not.toBeInTheDocument();
    expect(sessionStorage.getItem(storageKey)).toBe(saved);
    await userEvent.click(discard);
    expect(screen.queryByRole("button", { name: "继续同步" })).not.toBeInTheDocument();
    expect(sessionStorage.getItem(storageKey)).toBeNull();
    expect(screen.getByRole("button", { name: "同步商品" })).toBeEnabled();
  }
});
it.each([false, true])(
  "keeps orders with unknown package numbers visible and tracks only actual packages (valid=%s)",
  async (valid) => {
    context.permissions = [
      "workbench.store.orders.read",
      "workbench.store.orders.sync",
    ];
    const packages = [
      { id: "", waybill: "", carrier: "Carrier", label: "label-a" },
      ...(valid
        ? [{ id: "package-a", waybill: "", carrier: "Carrier", label: "" }]
        : []),
    ];
    const orderRecord = {
      storeId: id,
      syncId: id,
      id: "order-a",
      observedAt: "2026-10-09T01:00:00Z",
      order: {
        id: "order-a",
        site: "shein-us",
        status: 2,
        stockMode: null,
        type: null,
        tag: 1,
        reasons: [4],
        items: [],
        packages,
        amount: null,
        supplyCost: null,
        createdAt: "",
        updatedAt: "",
        issuedAt: "",
        needDeliveryAt: "",
        handoverAt: "",
        expectedCollectAt: "",
      },
    };
    const fetch = vi.fn().mockImplementation((path: string) => {
      const data = path.endsWith("/capabilities")
        ? {
            available: true,
            canSync: true,
            kind: "orders",
            site: "shein-us",
            platformUrl: "https://sellerhub.shein.com/",
          }
        : path.endsWith("/track")
          ? []
          : path.includes("/records/")
            ? orderRecord
            : {
                items: [orderRecord],
                next: "",
                summary: {
                  total: 1,
                  active: 0,
                  offShelf: 0,
                  today: 0,
                  todayUnknown: 1,
                  pending: 1,
                  transit: 0,
                  exceptional: 1,
                  unknown: 0,
                },
                syncs: [],
                latest: [],
                complete: false,
              };
      return Promise.resolve(
        Response.json({ organizationId: "org-a", userId: "actor-a", data }),
      );
    });
    vi.stubGlobal("fetch", fetch);
    renderPage("orders");
    await userEvent.click(
      await screen.findByRole("button", { name: "详情 / 物流" }),
    );
    expect(await screen.findByText(/包裹号尚未提供/)).toBeInTheDocument();
    expect(screen.queryByText("正在查询物流…")).not.toBeInTheDocument();
    if (valid) {
      expect(
        await screen.findByText("平台尚未提供物流轨迹。"),
      ).toBeInTheDocument();
      expect(screen.getByRole("combobox", { name: "选择包裹" })).toHaveValue(
        "package-a",
      );
      expect(
        fetch.mock.calls.some((c) =>
          c[0].includes("/packages/package-a/track"),
        ),
      ).toBe(true);
    } else {
      expect(
        screen.queryByRole("combobox", { name: "选择包裹" }),
      ).not.toBeInTheDocument();
      expect(fetch.mock.calls.some((c) => c[0].includes("/track"))).toBe(false);
    }
  },
);
