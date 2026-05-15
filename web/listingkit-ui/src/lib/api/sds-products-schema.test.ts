import { describe, expect, it } from "vitest";

import {
  parseSDSCategoriesResponse,
  parseSDSProductDetailResponse,
  parseSDSProductListResponse,
  parseSDSShipmentAreasResponse,
} from "@/lib/api/sds-products-schema";

describe("SDS product response schemas", () => {
  it("parses product list pagination and item identity", async () => {
    const response = jsonResponse({
      totalCount: 1,
      page: 1,
      size: 20,
      items: [{ id: 100, name: "T-shirt" }],
    });

    await expect(parseSDSProductListResponse(response)).resolves.toMatchObject({
      totalCount: 1,
      items: [{ id: 100, name: "T-shirt" }],
    });
  });

  it("rejects product lists with invalid item ids", async () => {
    const response = jsonResponse({
      items: [{ id: "not-a-number", name: "T-shirt" }],
    });

    await expect(parseSDSProductListResponse(response)).rejects.toMatchObject({
      message: "SDS API returned an unexpected product list response",
    });
  });

  it("parses product detail identity", async () => {
    await expect(
      parseSDSProductDetailResponse(
        jsonResponse({ id: 100, name: "T-shirt", subproducts: { items: [] } }),
      ),
    ).resolves.toMatchObject({ id: 100, name: "T-shirt" });
  });

  it("parses shipment areas and categories", async () => {
    await expect(
      parseSDSShipmentAreasResponse(
        jsonResponse([{ value: "US", label: "United States", totalCount: 10 }]),
      ),
    ).resolves.toHaveLength(1);

    await expect(
      parseSDSCategoriesResponse(
        jsonResponse([{ id: 1, name: "Apparel", count: 10 }]),
      ),
    ).resolves.toHaveLength(1);
  });

  it("turns invalid JSON and error responses into clear errors", async () => {
    await expect(
      parseSDSProductListResponse(
        new Response("<html>bad</html>", {
          status: 502,
          headers: { "content-type": "text/html" },
        }),
      ),
    ).rejects.toThrow("SDS API returned invalid JSON: 502");

    await expect(
      parseSDSProductListResponse(
        jsonResponse({ message: "SDS token expired" }, { status: 401 }),
      ),
    ).rejects.toThrow("SDS token expired");
  });
});

function jsonResponse(payload: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "content-type": "application/json" },
    ...init,
  });
}
