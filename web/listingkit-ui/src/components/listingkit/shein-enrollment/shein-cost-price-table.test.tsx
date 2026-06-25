import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinCostPriceTable } from "@/components/listingkit/shein-enrollment/shein-cost-price-table";

function renderCostPriceTable(options?: {
  item?: Record<string, unknown>;
  shipmentArea?: string;
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
          },
        ]}
        items={[
          {
            id: 8,
            skc_name: "SKC-A",
            supplier_code: "MG8006905001-B3195DA6",
            auto_cost_price: 39.1,
            effective_cost_price: 39.1,
            ...options?.item,
          },
        ]}
        onSave={vi.fn()}
        saving={false}
        shipmentArea={options?.shipmentArea}
        storeId={870}
      />
    </QueryClientProvider>,
  );
}

describe("SheinCostPriceTable", () => {
  it("shows POD SDS source product information for grouped cost rows", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: 238915,
              name: "带刻度方形挂钟",
              sku: "MG8006905001",
              currentPrice: 18.8,
              issuingBayArea: { name: "美国直发", countryCode: "US" },
            },
          ],
          totalCount: 1,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable();

    expect(await screen.findByText("带刻度方形挂钟")).toBeInTheDocument();
    expect(screen.getByText("POD/SDS: MG8006905001")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥18.80")).toBeInTheDocument();
    expect(screen.getByText("发货地 美国直发 US")).toBeInTheDocument();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products?keyword=MG8006905001&page=1&size=1&shipmentArea=US&preciseSearch=1",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("shows price and shipment area from the matched POD SDS variant", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: 238915,
              name: "方形挂钟",
              sku: "MG8006905",
              subproducts: {
                items: [
                  {
                    id: 238916,
                    sku: "MG8006905001",
                    color_name: "白色",
                    size: "25x25cm",
                    currentPrice: 16.6,
                    issuingBayArea: { name: "美国直发", countryCode: "US" },
                  },
                ],
              },
            },
          ],
          totalCount: 1,
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable();

    expect(await screen.findByText("方形挂钟")).toBeInTheDocument();
    expect(screen.getByText("变体 白色 / 25x25cm")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥16.60")).toBeInTheDocument();
    expect(screen.getByText("发货地 美国直发 US")).toBeInTheDocument();
  });

  it("loads SDS product detail when the list response omits variant price and shipment area", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (url === "/api/sds/products/238915") {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 238915,
              name: "方形挂钟",
              sku: "MG8006905",
              subproducts: {
                items: [
                  {
                    id: 238916,
                    sku: "MG8006905001",
                    color_name: "白色",
                    size: "25x25cm",
                    currentPrice: 16.6,
                    issuingBayArea: { name: "美国直发", countryCode: "US" },
                  },
                ],
              },
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            items: [
              {
                id: 238915,
                name: "方形挂钟",
                sku: "MG8006905",
              },
            ],
            totalCount: 1,
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable();

    expect(await screen.findByText("POD 价 ¥16.60")).toBeInTheDocument();
    expect(screen.getByText("变体 白色 / 25x25cm")).toBeInTheDocument();
    expect(screen.getByText("发货地 美国直发 US")).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products/238915",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("uses precise SDS SKU search and loads parent detail for a matched child SKU", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (url === "/api/sds/products/238915") {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 238915,
              name: "方形挂钟",
              sku: "XB0610007",
              subproducts: {
                items: [
                  {
                    id: 238916,
                    parent_id: 238915,
                    sku: "MG8006905001",
                    color_name: "白色",
                    size: "25x25cm",
                    currentPrice: 16.6,
                    issuingBayArea: { name: "美国直发", countryCode: "US" },
                  },
                ],
              },
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            items: [
              {
                id: 238916,
                parent_id: 238915,
                name: "方形挂钟 子SKU",
                sku: "MG8006905001",
              },
            ],
            totalCount: 1,
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable();

    expect(await screen.findByText("POD 价 ¥16.60")).toBeInTheDocument();
    expect(screen.getByText("发货地 美国直发 US")).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products?keyword=MG8006905001&page=1&size=1&shipmentArea=US&preciseSearch=1",
        expect.objectContaining({ method: "GET" }),
      );
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products/238915",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("falls back to synced cost and store shipment area when SDS product details are unavailable", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ items: [], totalCount: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable({ shipmentArea: "US" });

    expect(await screen.findByText("POD 价 ¥39.10")).toBeInTheDocument();
    expect(screen.getByText("发货地 US")).toBeInTheDocument();
    expect(screen.getByText("来源 POD/SDS 商品")).toBeInTheDocument();
    expect(screen.queryByText("标题 SKC-A")).not.toBeInTheDocument();
  });

  it("falls back to ListingKit task SDS title when the live SDS source product is unavailable", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (
        url ===
        "/api/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=MG8006905001"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  source_code: "MG8006905001",
                  title: "历史 SDS 带刻度方形挂钟",
                  product_sku: "MG8006905",
                  variant_sku: "MG8006905001",
                  price: 16.6,
                  variant_label: "白色 25x25cm MG8006905001",
                },
              ],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable({ shipmentArea: "US" });

    expect(await screen.findByText("历史 SDS 带刻度方形挂钟")).toBeInTheDocument();
    expect(screen.queryByText("标题 历史 SDS 带刻度方形挂钟")).not.toBeInTheDocument();
    expect(screen.getByText("变体 白色 25x25cm MG8006905001")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥16.60")).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=MG8006905001",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("matches ListingKit task SDS title from source variants when top-level source SKU is empty", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (
        url ===
        "/api/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=XB0610007001"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  source_code: "XB0610007001",
                  title: "方形双层腰包 -（单图多拼可选）",
                  product_sku: "",
                  variant_sku: "XB0610007001",
                  price: 34.5,
                  variant_label: "white / 16x23cm",
                },
              ],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable({
      item: { supplier_code: "XB0610007001-01584EB6" },
      shipmentArea: "US",
    });

    expect(await screen.findByText("方形双层腰包 -（单图多拼可选）")).toBeInTheDocument();
    expect(screen.getByText("变体 white / 16x23cm")).toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥34.50")).toBeInTheDocument();
  });

  it("searches SDS source products in the store shipment area", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ items: [], totalCount: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderCostPriceTable({ shipmentArea: "DE" });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products?keyword=MG8006905001&page=1&size=1&shipmentArea=DE&preciseSearch=1",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("loads the SDS product title by parent SKU when variant SKU search has no result", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (
        url ===
        "/api/sds/products?keyword=MG8006905&page=1&size=1&shipmentArea=US&preciseSearch=1"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: 238915,
                  name: "SDS 带刻度方形挂钟",
                  sku: "MG8006905",
                  subproducts: {
                    items: [
                      {
                        id: 238916,
                        parent_id: 238915,
                        sku: "MG8006905001",
                        currentPrice: 16.6,
                        issuingBayArea: { name: "美国直发", countryCode: "US" },
                      },
                    ],
                  },
                },
              ],
              totalCount: 1,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      }
      if (
        url ===
        "/api/sds/products?keyword=MG8006905001&page=1&size=1&shipmentArea=US&preciseSearch=1"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: 238916,
                  name: "",
                  sku: "MG8006905001",
                },
              ],
              totalCount: 1,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable({
      item: {
        product_name_multi: "POD 带刻度方形挂钟",
      },
    });

    expect(await screen.findByText("SDS 带刻度方形挂钟")).toBeInTheDocument();
    expect(screen.queryByText("标题 SDS 带刻度方形挂钟")).not.toBeInTheDocument();
    expect(screen.queryByText("标题 POD 带刻度方形挂钟")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/sds/products?keyword=MG8006905&page=1&size=1&shipmentArea=US&preciseSearch=1",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("uses SDS product_name as title when the SDS response omits name", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (
        url ===
        "/api/sds/products?keyword=MG8006905&page=1&size=1&shipmentArea=US&preciseSearch=1"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: 238915,
                  product_name: "SDS product_name 标题",
                  sku: "MG8006905",
                },
              ],
              totalCount: 1,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable();

    expect(await screen.findByText("SDS product_name 标题")).toBeInTheDocument();
    expect(screen.queryByText("标题 SDS product_name 标题")).not.toBeInTheDocument();
  });

  it("uses SDS product_name_multi as title when name is null", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (
        url ===
        "/api/sds/products?keyword=MG8006905&page=1&size=1&shipmentArea=US&preciseSearch=1"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: 238915,
                  name: null,
                  product_name_multi: "SDS product_name_multi 标题",
                  sku: "MG8006905",
                },
              ],
              totalCount: 1,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable();

    expect(await screen.findByText("SDS product_name_multi 标题")).toBeInTheDocument();
    expect(screen.queryByText("标题 SDS product_name_multi 标题")).not.toBeInTheDocument();
  });

  it("keeps the list SDS title when detail response has an empty title", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = String(input);
      if (url === "/api/sds/products/238915") {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 238915,
              name: "",
              sku: "MG8006905",
              currentPrice: 18.8,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
        );
      }
      if (
        url ===
        "/api/sds/products?keyword=MG8006905&page=1&size=1&shipmentArea=US&preciseSearch=1"
      ) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: 238915,
                  product_name: "SDS 列表标题",
                  sku: "MG8006905",
                },
              ],
              totalCount: 1,
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            },
          ),
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

    renderCostPriceTable();

    expect(await screen.findByText("SDS 列表标题")).toBeInTheDocument();
    expect(screen.queryByText("标题 SDS 列表标题")).not.toBeInTheDocument();
    expect(screen.getByText("POD 价 ¥18.80")).toBeInTheDocument();
  });
});
