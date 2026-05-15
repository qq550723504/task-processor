import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingProductImportMappingSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    importTaskId: z.number(),
    storeId: z.number(),
    platform: z.string(),
    region: z.string(),
    productId: z.string(),
    parentProductId: z.string().optional(),
    sku: z.string().optional(),
    costPrice: z.number().optional(),
    platformProductId: z.string().optional(),
    platformParentProductId: z.string().optional(),
    filterRuleId: z.number().optional(),
    filterRuleRange: z.string().optional(),
    profitRuleId: z.number().optional(),
    salePriceMultiplier: z.number(),
    discountPriceMultiplier: z.number(),
    status: z.number(),
    remark: z.string().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const productImportMappingPageSchema = z
  .object({
    items: z.array(listingProductImportMappingSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingProductImportMapping = z.infer<
  typeof listingProductImportMappingSchema
>;
export type ListingProductImportMappingPage = z.infer<
  typeof productImportMappingPageSchema
>;

export type ListingProductImportMappingQuery = Omit<QueueQuery, "status"> & {
  page?: number;
  page_size?: number;
  importTaskId?: number;
  storeId?: number;
  platform?: string;
  region?: string;
  productId?: string;
  parentProductId?: string;
  sku?: string;
  platformProductId?: string;
  platformParentProductId?: string;
  status?: string;
};

export type ListingProductImportMappingInput = {
  importTaskId: number;
  storeId: number;
  platform: string;
  region: string;
  productId: string;
  parentProductId?: string;
  sku?: string;
  costPrice?: number;
  platformProductId?: string;
  platformParentProductId?: string;
  filterRuleId?: number;
  filterRuleRange?: string;
  profitRuleId?: number;
  salePriceMultiplier?: number;
  discountPriceMultiplier?: number;
  status?: number;
  remark?: string;
};

export function parseProductImportMappingPageResponse(
  payload: unknown,
): ListingProductImportMappingPage {
  return parseApiResponseShape(
    payload,
    productImportMappingPageSchema,
    "ListingKit API returned an unexpected product import mapping page response",
  );
}

export function parseProductImportMappingResponse(
  payload: unknown,
): ListingProductImportMapping {
  return parseApiResponseShape(
    payload,
    listingProductImportMappingSchema,
    "ListingKit API returned an unexpected product import mapping response",
  );
}

export async function getListingProductImportMappings(
  query: ListingProductImportMappingQuery = {},
): Promise<ListingProductImportMappingPage> {
  const payload = await apiRequest<unknown>("/admin/product-import-mappings", {
    query,
  });
  return parseProductImportMappingPageResponse(payload);
}

export async function createListingProductImportMapping(
  input: ListingProductImportMappingInput,
): Promise<ListingProductImportMapping> {
  const payload = await apiRequest<unknown>("/admin/product-import-mappings", {
    method: "POST",
    body: input,
  });
  return parseProductImportMappingResponse(payload);
}

export async function updateListingProductImportMappingStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingProductImportMapping> {
  const payload = await apiRequest<unknown>(
    `/admin/product-import-mappings/${id}/status`,
    {
      method: "PATCH",
      body: { status, remark },
    },
  );
  return parseProductImportMappingResponse(payload);
}

export async function deleteListingProductImportMapping(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/product-import-mappings/${id}`, {
    method: "DELETE",
  });
}
