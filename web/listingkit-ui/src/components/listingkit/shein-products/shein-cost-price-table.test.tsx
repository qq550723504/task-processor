import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  SheinCostPriceTable,
  type SheinCostPriceSaveTarget,
} from "@/components/listingkit/shein-products/shein-cost-price-table";

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img alt={props.alt ?? ""} {...props} />
  ),
}));

function renderCostPriceTable(options?: {
  group?: Record<string, unknown>;
  item?: Record<string, unknown>;
  items?: Array<Record<string, unknown>>;
  onSave?: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  onSyncSourceSDSProduct?: (sourceCode: string) => Promise<void>;
  shipmentArea?: string;
  sourceGroups?: Array<Record<string, unknown>>;
  syncingSourceCode?: string;
}) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={client}>
      <SheinCostPriceTable
        groups={[
          {
            group_key: "style:B3195DA6",
            group_label: "B3195DA6",
            manual_cost_price: 50,
            ...options?.group,
          },
        ]}
        items={
          options?.items ?? [
            {
              id: 8,
              skc_name: "SKC-A",
              supplier_code: "MG8006905001-B3195DA6",
              auto_cost_price: 39.1,
              effective_cost_price: 39.1,
              ...options?.item,
            },
          ]
        }
        onSave={options?.onSave ?? vi.fn()}
        onSyncSourceSDSProduct={options?.onSyncSourceSDSProduct}
        saving={false}
        shipmentArea={options?.shipmentArea}
        sourceGroups={options?.sourceGroups}
        storeId={870}
        syncingSourceCode={options?.syncingSourceCode}
      />
    </QueryClientProvider>,
  );
}

function mockSourceMetadataFetch(
  items: Array<Record<string, unknown>>,
  expectedSourceCodes = "MG8006905001",
) {
  const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
    const url = String(input);
    if (
      url ===
      `/api/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=${expectedSourceCodes}`
    ) {
      return Promise.resolve(
        new Response(JSON.stringify({ items }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    }
    return Promise.resolve(
      new Response(JSON.stringify({ items: [], totalCount: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("SheinCostPriceTable", () => {
  it("loads POD SDS source information through the batched metadata endpoint", async () => {
    const fetchMock = mockSourceMetadataFetch([
      {
        source_code: "MG8006905001",
        title: "批量 SDS 方形挂钟",
        variant_sku: "MG8006905001",
        price: 16.6,
        variant_label: "白色 / 25x25cm",
        image_url: "https://cdn.sdspod.com/mockup/clock.jpg",
      },
    ]);

    renderCostPriceTable();

    expect(await screen.findByText("批量 SDS 方形挂钟")).toBeInTheDocument();
    expect(screen.getByAltText("批量 SDS 方形挂钟 首图")).toHaveAttribute(
      "src",
      "https://cdn.sdspod.com/mockup/clock.jpg",
    );
    expect(screen.getByText("POD/SDS: MG8006905001")).toBeInTheDocument();
    expect(screen.getByText("变体 25x25cm")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥16.60")).toBeInTheDocument();
    expect(screen.getByText("发货地 US")).toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(([input]) => String(input).startsWith("/api/sds/products")),
    ).toBe(false);
  });

  it("opens a larger preview when the POD SDS source image is clicked", async () => {
    mockSourceMetadataFetch([
      {
        source_code: "MG8006905001",
        title: "批量 SDS 方形挂钟",
        variant_sku: "MG8006905001",
        image_url: "https://cdn.sdspod.com/mockup/clock.jpg",
      },
    ]);

    renderCostPriceTable();

    const imageButton = await screen.findByRole("button", {
      name: "查看批量 SDS 方形挂钟首图",
    });
    const hoverPreview = screen.getByAltText("批量 SDS 方形挂钟 悬浮预览");
    expect(imageButton).toHaveClass("cursor-zoom-in");
    expect(hoverPreview.parentElement).toHaveClass("group-hover:block");

    fireEvent.click(imageButton);

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 图片预览")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "批量 SDS 方形挂钟首图" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("matches metadata by source, variant, or product SKU", async () => {
    mockSourceMetadataFetch([
      {
        source_code: "",
        title: "产品 SKU 匹配标题",
        product_sku: "MG8006905001",
        variant_sku: "",
        price: 18.8,
      },
    ]);

    renderCostPriceTable();

    expect(await screen.findByText("产品 SKU 匹配标题")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥18.80")).toBeInTheDocument();
  });

  it("groups products by POD SDS source code even when SHEIN style suffix differs", async () => {
    mockSourceMetadataFetch([
      {
        source_code: "XB0608021001",
        title: "皮革多功能钱包",
      },
    ], "XB0608021001");

    renderCostPriceTable({
      group: {
        group_key: "style:DA578653",
        group_label: "DA578653",
        manual_cost_price: 43.3,
      },
      items: [
        {
          id: 8,
          skc_name: "sg260604223794143925005",
          supplier_code: "XB0608021001-DA578653",
        },
        {
          id: 9,
          skc_name: "sg260603162031320517713",
          supplier_code: "XB0608021001-DE93508C",
        },
      ],
    });

    expect(await screen.findByText("XB0608021001 · 2 个商品")).toBeInTheDocument();
    expect(screen.queryByText("DA578653 · 1 个商品")).not.toBeInTheDocument();
    expect(screen.queryByText("DE93508C · 1 个商品")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("43.3")).toBeInTheDocument();
  });

  it("does not prefill or display automatic cost when no manual cost is maintained", async () => {
    mockSourceMetadataFetch([]);

    renderCostPriceTable({
      group: { manual_cost_price: null },
      shipmentArea: "US",
    });

    const input = screen.getByLabelText("成本价 MG8006905001");
    expect(input).toHaveValue("");
    expect(screen.queryByText(/自动\/当前成本/)).not.toBeInTheDocument();
    expect(screen.queryByText("POD 价 ¥39.10")).not.toBeInTheDocument();
    expect(await screen.findByText("发货地 US")).toBeInTheDocument();
  });

  it("falls back to source code and store shipment area when source metadata is unavailable", async () => {
    mockSourceMetadataFetch([]);

    renderCostPriceTable({ shipmentArea: "US" });

    expect(await screen.findByText("来源 POD/SDS 商品")).toBeInTheDocument();
    expect(screen.getByText("POD/SDS: MG8006905001")).toBeInTheDocument();
    expect(screen.getByText("发货地 US")).toBeInTheDocument();
    expect(screen.queryByText("POD 价 ¥39.10")).not.toBeInTheDocument();
  });

  it("requests all visible source SDS codes in one metadata call", async () => {
    const fetchMock = mockSourceMetadataFetch([], "MG8006905001,MG8006905002");

    renderCostPriceTable({
      items: [
        {
          id: 8,
          skc_name: "SKC-A",
          supplier_code: "MG8006905001-B3195DA6",
        },
        {
          id: 9,
          skc_name: "SKC-B",
          supplier_code: "MG8006905002-B3195DA6",
        },
      ],
    });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=MG8006905001%2CMG8006905002",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("saves manual group cost from the input", async () => {
    mockSourceMetadataFetch([]);
    const onSave = vi.fn().mockResolvedValue(undefined);

    renderCostPriceTable({ onSave });

    fireEvent.change(screen.getByLabelText("成本价 MG8006905001"), {
      target: { value: "45.6" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存成本价" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        {
          groupKey: "source:MG8006905001",
          groupLabel: "MG8006905001",
          productId: undefined,
        },
        45.6,
      );
    });
  });

  it("syncs one source product from the cost card action", async () => {
    mockSourceMetadataFetch([]);
    const onSyncSourceSDSProduct = vi.fn().mockResolvedValue(undefined);

    renderCostPriceTable({ onSyncSourceSDSProduct });

    fireEvent.click(await screen.findByRole("button", { name: "同步该产品 MG8006905001" }));

    await waitFor(() => {
      expect(onSyncSourceSDSProduct).toHaveBeenCalledWith("MG8006905001");
    });
  });

  it("edits each source product variant once in the child table", async () => {
    mockSourceMetadataFetch([
      {
        source_code: "XB0603003001",
        title: "帆布拉绳收纳袋双面印刷",
        product_sku: "XB0603003001",
        variant_sku: "XB0603003001",
        variant_label: "white / 12*18cm",
        price: 8.81,
      },
      {
        source_code: "XB0603003002",
        title: "帆布拉绳收纳袋双面印刷",
        product_sku: "XB0603003001",
        variant_sku: "XB0603003002",
        variant_label: "white / 20*25cm",
        price: 9.64,
      },
    ], "XB0603003001");
    const onSave = vi.fn().mockResolvedValue(undefined);

    renderCostPriceTable({
      onSave,
      sourceGroups: [
        {
          group_key: "source:XB0603003001",
          group_label: "XB0603003001",
          source_code: "XB0603003001",
          sku_codes: ["SKU-BLACK-S", "SKU-BLACK-M"],
          sku_groups: [
            {
              group_key: "source:XB0603003001:variant:WHITE",
              group_label: "XB0603003001 / White",
              source_code: "XB0603003001",
              sku_code: "White",
              variant_label: "White",
              sku_codes: ["SKU-WHITE"],
              product_count: 1,
              manual_cost_price: 31.2,
            },
            {
              group_key: "source:XB0603003001:variant:MULTI",
              group_label: "XB0603003001 / 多色",
              source_code: "XB0603003001",
              sku_code: "多色",
              variant_label: "多色",
              sku_codes: ["SKU-COLOR-A", "SKU-COLOR-B"],
              product_count: 10,
              manual_cost_price: 29.8,
            },
            {
              group_key: "source:XB0603003001:variant:WHITE-CN",
              group_label: "XB0603003001 / 白色",
              source_code: "XB0603003001",
              sku_code: "白色",
              variant_label: "白色",
              sku_codes: ["SKU-CN-WHITE"],
              product_count: 7,
              manual_cost_price: 30.1,
            },
          ],
          product_count: 18,
          products: [
            {
              id: 18,
              skc_name: "sh260603194059486654294",
              supplier_code: "XB0603003001-181EB5DF",
              sale_name: "多色",
              site_snapshot: `{"sku_codes":["SKU-BLACK-S","SKU-BLACK-M","SKU-BLACK-L"]}`,
            },
          ],
          manual_cost_price: 31.2,
        },
      ],
    });

    expect(await screen.findByText("XB0603003001 · 18 个商品")).toBeInTheDocument();
    const detailTable = screen.getByRole("table", { name: "XB0603003001 明细" });
    expect(detailTable).toBeInTheDocument();
    expect(await within(detailTable).findByText("12*18cm")).toBeInTheDocument();
    expect(within(detailTable).getAllByRole("textbox")).toHaveLength(2);
    expect(within(detailTable).getByText("20*25cm")).toBeInTheDocument();
    expect(within(detailTable).queryByText("white / 12*18cm")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("white / 20*25cm")).not.toBeInTheDocument();
    expect(detailTable).toHaveTextContent("POD/SDS: XB0603003001 · POD 价 ¥8.81");
    expect(detailTable).toHaveTextContent("POD/SDS: XB0603003002 · POD 价 ¥9.64");
    expect(within(detailTable).getAllByText("18 个商品")).toHaveLength(2);
    expect(within(detailTable).queryByText("White")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("多色")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("白色")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("sh260603194059486654294 / sh260603194059486654294")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("SKU-COLOR-A / SKU-COLOR-B")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("成本价 XB0603003001")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("成本价 XB0603003001 / XB0603003001"), {
      target: { value: "34.5" },
    });
    fireEvent.click(within(detailTable).getAllByRole("button", { name: "保存" })[0]);

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(
        {
          groupKey: "source:XB0603003001:variant:XB0603003001",
          groupLabel: "XB0603003001 / XB0603003001",
        },
        34.5,
      );
    });
  });

  it("hides SHEIN color groups when SDS size variants are available", async () => {
    mockSourceMetadataFetch([
      {
        source_code: "XB0603003001",
        title: "帆布拉绳收纳袋双面印刷",
        product_sku: "XB0603003001",
        variant_sku: "XB0603003001",
        variant_label: "white / 12*18cm",
        price: 8.81,
      },
    ], "XB0603003001");

    renderCostPriceTable({
      sourceGroups: [
        {
          group_key: "source:XB0603003001",
          group_label: "XB0603003001",
          source_code: "XB0603003001",
          sku_groups: [
            {
              group_key: "source:XB0603003001:variant:12*18CM",
              group_label: "XB0603003001 / 12*18cm",
              source_code: "XB0603003001",
              sku_code: "12*18cm",
              variant_label: "12*18cm",
              sku_codes: ["SKU-SIZE-12"],
              product_count: 6,
            },
            {
              group_key: "source:XB0603003001:variant:20*25CM",
              group_label: "XB0603003001 / 20*25cm",
              source_code: "XB0603003001",
              sku_code: "20*25cm",
              variant_label: "20*25cm",
              sku_codes: ["SKU-SIZE-20"],
              product_count: 6,
            },
            {
              group_key: "source:XB0603003001:variant:WHITE",
              group_label: "XB0603003001 / White",
              source_code: "XB0603003001",
              sku_code: "White",
              variant_label: "White",
              sku_codes: ["SKU-WHITE"],
              product_count: 1,
            },
            {
              group_key: "source:XB0603003001:variant:MULTI",
              group_label: "XB0603003001 / 多色",
              source_code: "XB0603003001",
              sku_code: "多色",
              variant_label: "多色",
              sku_codes: ["SKU-MULTI"],
              product_count: 9,
            },
            {
              group_key: "source:XB0603003001:variant:WHITE-CN",
              group_label: "XB0603003001 / 白色",
              source_code: "XB0603003001",
              sku_code: "白色",
              variant_label: "白色",
              sku_codes: ["SKU-WHITE-CN"],
              product_count: 2,
            },
          ],
          product_count: 18,
          products: [
            {
              id: 18,
              skc_name: "sh260603194059486654294",
              supplier_code: "XB0603003001-181EB5DF",
            },
          ],
        },
      ],
    });

    const detailTable = await screen.findByRole("table", { name: "XB0603003001 明细" });

    expect(within(detailTable).getAllByRole("textbox")).toHaveLength(2);
    expect(within(detailTable).getByText("12*18cm")).toBeInTheDocument();
    expect(within(detailTable).getByText("20*25cm")).toBeInTheDocument();
    expect(within(detailTable).queryByText("White")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("多色")).not.toBeInTheDocument();
    expect(within(detailTable).queryByText("白色")).not.toBeInTheDocument();
  });
});
