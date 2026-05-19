import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

const optionalBooleanSchema = z.boolean().optional();

export const listingStoreSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    name: z.string(),
    username: z.string(),
    password: z.string().optional(),
    loginUrl: z.string().optional(),
    shopType: z.string(),
    region: z.string().optional(),
    platform: z.string(),
    dailyLimit: z.number().optional(),
    dailyLimitType: z.string().optional(),
    fixedStockCount: z.number().optional(),
    skuGenerateStrategy: z.string().optional(),
    prefix: z.string().optional(),
    suffix: z.string().optional(),
    proxy: z.string().optional(),
    enableAutoListing: optionalBooleanSchema,
    enableAutoLogin: optionalBooleanSchema,
    enableDraft: optionalBooleanSchema,
    enableAutoPrice: optionalBooleanSchema,
    enableRebargain: optionalBooleanSchema,
    temuPriceRejectStrategy: z.string().optional(),
    priceType: z.string().optional(),
    remark: z.string().optional(),
    status: z.number().optional(),
    validFrom: z.string().optional(),
    validUntil: z.string().optional(),
    expired: z.boolean().optional(),
    dedicatedQueueEnabled: optionalBooleanSchema,
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const storePageSchema = z
  .object({
    items: z.array(listingStoreSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

const simpleStorePageSchema = z
  .object({
    items: z.array(
      z
        .object({
          id: z.number(),
          name: z.string(),
          platform: z.string().optional(),
          region: z.string().optional(),
        })
        .passthrough(),
    ),
  })
  .passthrough();

export type ListingStore = z.infer<typeof listingStoreSchema>;
export type ListingStorePage = z.infer<typeof storePageSchema>;
export type SimpleListingStore = z.infer<
  typeof simpleStorePageSchema
>["items"][number];

export type ListingStoreQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  name?: string;
  username?: string;
  shopType?: string;
  region?: string;
  platform?: string;
  skuGenerateStrategy?: string;
  enableAutoListing?: boolean;
  enableAutoLogin?: boolean;
  enableDraft?: boolean;
  enableAutoPrice?: boolean;
  enableRebargain?: boolean;
  priceType?: string;
  status?: number;
  expired?: boolean;
};

export type ListingStoreInput = {
  name: string;
  username: string;
  password?: string;
  loginUrl?: string;
  shopType: string;
  region?: string;
  platform: string;
  dailyLimit?: number;
  dailyLimitType?: string;
  fixedStockCount?: number;
  skuGenerateStrategy?: string;
  prefix?: string;
  suffix?: string;
  proxy?: string;
  enableAutoListing?: boolean;
  enableAutoLogin?: boolean;
  enableDraft?: boolean;
  enableAutoPrice?: boolean;
  enableRebargain?: boolean;
  temuPriceRejectStrategy?: string;
  priceType?: string;
  remark?: string;
  status?: number;
};

export function parseStorePageResponse(payload: unknown): ListingStorePage {
  return parseApiResponseShape(
    payload,
    storePageSchema,
    "ListingKit API returned an unexpected store page response",
  );
}

export function parseStoreResponse(payload: unknown): ListingStore {
  return parseApiResponseShape(
    payload,
    listingStoreSchema,
    "ListingKit API returned an unexpected store response",
  );
}

export function parseSimpleStoreResponse(payload: unknown): SimpleListingStore[] {
  return parseApiResponseShape(
    payload,
    simpleStorePageSchema,
    "ListingKit API returned an unexpected simple store response",
  ).items;
}

export async function getListingStores(
  query: ListingStoreQuery = {},
): Promise<ListingStorePage> {
  const payload = await apiRequest<unknown>("/admin/stores", { query });
  return parseStorePageResponse(payload);
}

export async function getSimpleListingStores(): Promise<SimpleListingStore[]> {
  const payload = await apiRequest<unknown>("/admin/stores/simple");
  return parseSimpleStoreResponse(payload);
}

export async function createListingStore(
  input: ListingStoreInput,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>("/admin/stores", {
    method: "POST",
    body: input,
  });
  return parseStoreResponse(payload);
}

export async function updateListingStore(
  id: number,
  input: ListingStoreInput,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>(`/admin/stores/${id}`, {
    method: "PUT",
    body: input,
  });
  return parseStoreResponse(payload);
}

export async function updateListingStoreStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>(`/admin/stores/${id}/status`, {
    method: "PATCH",
    body: { status, remark },
  });
  return parseStoreResponse(payload);
}

export async function deleteListingStore(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/stores/${id}`, { method: "DELETE" });
}

export async function getDeletedListingStores(): Promise<ListingStore[]> {
  const payload = await apiRequest<unknown>("/admin/stores/deleted");
  return parseApiResponseShape(
    payload,
    z.array(listingStoreSchema),
    "ListingKit API returned an unexpected deleted store response",
  );
}

export async function restoreListingStore(id: number): Promise<ListingStore> {
  const payload = await apiRequest<unknown>(`/admin/stores/${id}/restore`, {
    method: "PUT",
  });
  return parseStoreResponse(payload);
}

export async function permanentlyDeleteListingStore(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/stores/${id}/permanent`, {
    method: "DELETE",
  });
}

export async function extendListingStoreValidity(
  id: number,
  days: number,
): Promise<ListingStore> {
  const payload = await apiRequest<unknown>(
    `/admin/stores/${id}/extend-validity?days=${encodeURIComponent(String(days))}`,
    { method: "PUT" },
  );
  return parseStoreResponse(payload);
}
