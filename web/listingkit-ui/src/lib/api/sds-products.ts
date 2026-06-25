import type {
  SDSCategory,
  SDSProductDetail,
  SDSProductListResponse,
  SDSShipmentArea,
} from "@/lib/types/sds";
import {
  parseSDSCategoriesResponse,
  parseSDSProductDetailResponse,
  parseSDSProductListResponse,
  parseSDSShipmentAreasResponse,
} from "@/lib/api/sds-products-schema";

type ListQuery = {
  keyword?: string;
  page?: number;
  size?: number;
  shipmentArea?: string;
  categoryId?: number;
  onSaleStatus?: number;
  hotSellStatus?: number;
  sortField?: string;
  sortType?: string;
  weightBand?: string;
  cycleBand?: string;
  preciseSearch?: boolean;
};

function buildSearch(query?: ListQuery) {
  const params = new URLSearchParams();
  if (query?.keyword?.trim()) {
    params.set("keyword", query.keyword.trim());
  }
  if (query?.page) {
    params.set("page", String(query.page));
  }
  if (query?.size) {
    params.set("size", String(query.size));
  }
  if (query?.shipmentArea?.trim()) {
    params.set("shipmentArea", query.shipmentArea.trim());
  }
  if (query?.categoryId) {
    params.set("categoryId", String(query.categoryId));
  }
  if (query?.onSaleStatus !== undefined) {
    params.set("onSaleStatus", String(query.onSaleStatus));
  }
  if (query?.hotSellStatus !== undefined) {
    params.set("hotSellStatus", String(query.hotSellStatus));
  }
  if (query?.sortField?.trim()) {
    params.set("sortField", query.sortField.trim());
  }
  if (query?.sortType?.trim()) {
    params.set("sortType", query.sortType.trim());
  }
  if (query?.weightBand?.trim()) {
    params.set("weightBand", query.weightBand.trim());
  }
  if (query?.cycleBand?.trim()) {
    params.set("cycleBand", query.cycleBand.trim());
  }
  if (query?.preciseSearch !== undefined) {
    params.set("preciseSearch", query.preciseSearch ? "1" : "0");
  }
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

export async function getSDSProducts(
  query?: ListQuery,
): Promise<SDSProductListResponse> {
  const response = await fetch(`/api/sds/products${buildSearch(query)}`, {
    method: "GET",
    cache: "no-store",
  });
  return parseSDSProductListResponse(response);
}

export async function getSDSProductDetail(
  productId: number,
): Promise<SDSProductDetail> {
  const response = await fetch(`/api/sds/products/${productId}`, {
    method: "GET",
    cache: "no-store",
  });
  return parseSDSProductDetailResponse(response);
}

export async function getSDSShipmentAreas(): Promise<SDSShipmentArea[]> {
  const response = await fetch("/api/sds/shipment-areas", {
    method: "GET",
    cache: "no-store",
  });
  return parseSDSShipmentAreasResponse(response);
}

export async function getSDSCategories(
  shipmentArea: string,
): Promise<SDSCategory[]> {
  const response = await fetch(`/api/sds/categories?shipmentArea=${encodeURIComponent(shipmentArea)}`, {
    method: "GET",
    cache: "no-store",
  });
  return parseSDSCategoriesResponse(response);
}
