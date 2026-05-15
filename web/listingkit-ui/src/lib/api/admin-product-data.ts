import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingProductDataSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    source: z.string().optional(),
    importTaskId: z.number().optional(),
    rawJsonDataId: z.number().optional(),
    storeId: z.number().optional(),
    categoryId: z.number().optional(),
    platform: z.string().optional(),
    region: z.string().optional(),
    parentProductId: z.string().optional(),
    productId: z.string(),
    title: z.string().optional(),
    description: z.string().optional(),
    originalPrice: z.number().optional(),
    specialPrice: z.number().optional(),
    priceCurrency: z.string().optional(),
    stock: z.string().optional(),
    brand: z.string().optional(),
    category: z.string().optional(),
    mainImageUrl: z.string().optional(),
    imageUrls: z.unknown().optional(),
    attributes: z.unknown().optional(),
    sourceUrl: z.string().optional(),
    status: z.number(),
    platformProductId: z.string().optional(),
    platformStatus: z.string().optional(),
    shelfStatus: z.number().optional(),
    publishTime: z.string().optional(),
    shelfTime: z.string().optional(),
    lastSyncTime: z.string().optional(),
    platformData: z.unknown().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const productDataPageSchema = z
  .object({
    items: z.array(listingProductDataSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingProductData = z.infer<typeof listingProductDataSchema>;
export type ListingProductDataPage = z.infer<typeof productDataPageSchema>;

export type ListingProductDataQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  storeId?: number;
  categoryId?: number;
  platform?: string;
  region?: string;
  parentProductId?: string;
  productId?: string;
  title?: string;
  brand?: string;
  status?: number;
  platformProductId?: string;
  shelfStatus?: number;
};

export type ListingProductDataInput = {
  source?: string;
  importTaskId?: number;
  rawJsonDataId?: number;
  storeId?: number;
  categoryId?: number;
  platform?: string;
  region?: string;
  parentProductId?: string;
  productId: string;
  title?: string;
  description?: string;
  originalPrice?: number;
  specialPrice?: number;
  priceCurrency?: string;
  stock?: string;
  brand?: string;
  category?: string;
  mainImageUrl?: string;
  imageUrls?: unknown;
  attributes?: unknown;
  sourceUrl?: string;
  status?: number;
  platformProductId?: string;
  platformStatus?: string;
  shelfStatus?: number;
  publishTime?: string;
  shelfTime?: string;
  lastSyncTime?: string;
  platformData?: unknown;
};

export function parseProductDataPageResponse(
  payload: unknown,
): ListingProductDataPage {
  return parseApiResponseShape(
    payload,
    productDataPageSchema,
    "ListingKit API returned an unexpected product data page response",
  );
}

export function parseProductDataResponse(payload: unknown): ListingProductData {
  return parseApiResponseShape(
    payload,
    listingProductDataSchema,
    "ListingKit API returned an unexpected product data response",
  );
}

export async function getListingProductData(
  query: ListingProductDataQuery = {},
): Promise<ListingProductDataPage> {
  const payload = await apiRequest<unknown>("/admin/product-data", { query });
  return parseProductDataPageResponse(payload);
}

export async function getListingProductDataDetail(
  id: number,
): Promise<ListingProductData> {
  const payload = await apiRequest<unknown>(`/admin/product-data/${id}`);
  return parseProductDataResponse(payload);
}

export async function createListingProductData(
  input: ListingProductDataInput,
): Promise<ListingProductData> {
  const payload = await apiRequest<unknown>("/admin/product-data", {
    method: "POST",
    body: input,
  });
  return parseProductDataResponse(payload);
}

export async function updateListingProductData(
  id: number,
  input: ListingProductDataInput,
): Promise<ListingProductData> {
  const payload = await apiRequest<unknown>(`/admin/product-data/${id}`, {
    method: "PUT",
    body: input,
  });
  return parseProductDataResponse(payload);
}

export async function updateListingProductDataStatus(
  id: number,
  status: number,
): Promise<ListingProductData> {
  const payload = await apiRequest<unknown>(`/admin/product-data/${id}/status`, {
    method: "PATCH",
    body: { status },
  });
  return parseProductDataResponse(payload);
}

export async function deleteListingProductData(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/product-data/${id}`, { method: "DELETE" });
}
