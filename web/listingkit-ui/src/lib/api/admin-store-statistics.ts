import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingStoreStatisticsSchema = z
  .object({
    id: z.number(),
    storeId: z.string().optional(),
    tenantId: z.number(),
    name: z.string(),
    platform: z.string().optional(),
    dailyLimit: z.number(),
    dailyLimitType: z.string().optional(),
    completedCount: z.number(),
    remainingCount: z.number(),
    holdCount: z.number(),
    queuedCount: z.number(),
    remainingQuota: z.number(),
    progressPercentage: z.number(),
    status: z.number(),
  })
  .passthrough();

export const listingStoreStatisticsSummarySchema = z
  .object({
    completed_count: z.coerce.number().int().nonnegative(),
    daily_limit: z.coerce.number().int().nonnegative(),
    remaining_count: z.coerce.number().int().nonnegative(),
    queued_count: z.coerce.number().int().nonnegative(),
    hold_count: z.coerce.number().int().nonnegative(),
  })
  .passthrough();

export const listingStoreStatisticsPageSchema = z
  .object({
    items: z.array(listingStoreStatisticsSchema),
    total: z.coerce.number().int().nonnegative(),
    page: z.coerce.number().int().positive(),
    page_size: z.coerce.number().int().positive(),
    summary: listingStoreStatisticsSummarySchema,
  })
  .passthrough();

export type ListingStoreStatistics = z.infer<
  typeof listingStoreStatisticsSchema
>;

export type ListingStoreStatisticsSummary = z.infer<
  typeof listingStoreStatisticsSummarySchema
>;

export type ListingStoreStatisticsPage = z.infer<
  typeof listingStoreStatisticsPageSchema
>;

export type ListingStoreStatisticsQuery = QueueQuery & {
  date?: string;
  page?: number;
  page_size?: number;
};

export function parseStoreStatisticsResponse(
  payload: unknown,
): ListingStoreStatisticsPage {
  return parseApiResponseShape(
    payload,
    listingStoreStatisticsPageSchema,
    "ListingKit API returned an unexpected store statistics response",
  );
}

export async function getListingStoreStatistics(
  query: ListingStoreStatisticsQuery = {},
): Promise<ListingStoreStatisticsPage> {
  return getStoreStatistics("/admin/store-statistics", query);
}

export async function getPlatformStoreStatistics(
  query: ListingStoreStatisticsQuery = {},
): Promise<ListingStoreStatisticsPage> {
  return getStoreStatistics("/platform/store-statistics", query);
}

async function getStoreStatistics(
  path: string,
  query: ListingStoreStatisticsQuery,
): Promise<ListingStoreStatisticsPage> {
  const normalizedQuery = {
    ...query,
    page: normalizePage(query.page),
    page_size: normalizePageSize(query.page_size),
  };

  const payload = await apiRequest<unknown>(path, {
    query: normalizedQuery,
  });
  return parseStoreStatisticsResponse(payload);
}

function normalizePage(value: number | undefined) {
  return value && value > 0 ? Math.trunc(value) : 1;
}

function normalizePageSize(value: number | undefined) {
  const size = value && value > 0 ? Math.trunc(value) : 20;
  return Math.min(size, 200);
}
