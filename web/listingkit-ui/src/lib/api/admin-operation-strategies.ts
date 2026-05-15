import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingOperationStrategySchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    storeId: z.number(),
    name: z.string(),
    platform: z.string(),
    status: z.number(),
    stockChangeThreshold: z.number().optional(),
    stockChangeAction: z.string().optional(),
    outOfStockAction: z.string().optional(),
    minProfitRate: z.number().optional(),
    lowProfitAction: z.string().optional(),
    priceUpdateMultiplier: z.number().optional(),
    fixedPriceAdjustment: z.number().optional(),
    stockUpdateRatio: z.number().optional(),
    remark: z.string().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const operationStrategyPageSchema = z
  .object({
    items: z.array(listingOperationStrategySchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingOperationStrategy = z.infer<
  typeof listingOperationStrategySchema
>;
export type ListingOperationStrategyPage = z.infer<
  typeof operationStrategyPageSchema
>;

export type ListingOperationStrategyQuery = Omit<QueueQuery, "status"> & {
  page?: number;
  page_size?: number;
  name?: string;
  platform?: string;
  storeId?: number;
  status?: string;
};

export type ListingOperationStrategyInput = {
  storeId: number;
  name: string;
  platform: string;
  status?: number;
  stockChangeThreshold?: number;
  stockChangeAction?: string;
  outOfStockAction?: string;
  minProfitRate?: number;
  lowProfitAction?: string;
  priceUpdateMultiplier?: number;
  fixedPriceAdjustment?: number;
  stockUpdateRatio?: number;
  remark?: string;
};

export function parseOperationStrategyPageResponse(
  payload: unknown,
): ListingOperationStrategyPage {
  return parseApiResponseShape(
    payload,
    operationStrategyPageSchema,
    "ListingKit API returned an unexpected operation strategy page response",
  );
}

export function parseOperationStrategyResponse(
  payload: unknown,
): ListingOperationStrategy {
  return parseApiResponseShape(
    payload,
    listingOperationStrategySchema,
    "ListingKit API returned an unexpected operation strategy response",
  );
}

export async function getListingOperationStrategies(
  query: ListingOperationStrategyQuery = {},
): Promise<ListingOperationStrategyPage> {
  const payload = await apiRequest<unknown>("/admin/operation-strategies", {
    query,
  });
  return parseOperationStrategyPageResponse(payload);
}

export async function createListingOperationStrategy(
  input: ListingOperationStrategyInput,
): Promise<ListingOperationStrategy> {
  const payload = await apiRequest<unknown>("/admin/operation-strategies", {
    method: "POST",
    body: input,
  });
  return parseOperationStrategyResponse(payload);
}

export async function updateListingOperationStrategyStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingOperationStrategy> {
  const payload = await apiRequest<unknown>(
    `/admin/operation-strategies/${id}/status`,
    {
      method: "PATCH",
      body: { status, remark },
    },
  );
  return parseOperationStrategyResponse(payload);
}

export async function deleteListingOperationStrategy(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/operation-strategies/${id}`, {
    method: "DELETE",
  });
}
