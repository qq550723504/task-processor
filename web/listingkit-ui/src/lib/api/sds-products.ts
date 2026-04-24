import type {
  SDSCategory,
  SDSProductDetail,
  SDSProductListResponse,
  SDSShipmentArea,
} from "@/lib/types/sds";

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
  const suffix = params.toString();
  return suffix ? `?${suffix}` : "";
}

export async function getSDSProducts(query?: ListQuery) {
  const response = await fetch(`/api/sds/products${buildSearch(query)}`, {
    method: "GET",
    cache: "no-store",
  });
  const payload = (await response.json()) as SDSProductListResponse;
  if (!response.ok) {
    throw new Error("Failed to load SDS products");
  }
  return payload;
}

export async function getSDSProductDetail(productId: number) {
  const response = await fetch(`/api/sds/products/${productId}`, {
    method: "GET",
    cache: "no-store",
  });
  const payload = (await response.json()) as SDSProductDetail;
  if (!response.ok) {
    throw new Error("Failed to load SDS product detail");
  }
  return payload;
}

export async function getSDSShipmentAreas() {
  const response = await fetch("/api/sds/shipment-areas", {
    method: "GET",
    cache: "no-store",
  });
  const payload = (await response.json()) as SDSShipmentArea[];
  if (!response.ok) {
    throw new Error("Failed to load SDS shipment areas");
  }
  return payload;
}

export async function getSDSCategories(shipmentArea: string) {
  const response = await fetch(`/api/sds/categories?shipmentArea=${encodeURIComponent(shipmentArea)}`, {
    method: "GET",
    cache: "no-store",
  });
  const payload = (await response.json()) as SDSCategory[];
  if (!response.ok) {
    throw new Error("Failed to load SDS categories");
  }
  return payload;
}
