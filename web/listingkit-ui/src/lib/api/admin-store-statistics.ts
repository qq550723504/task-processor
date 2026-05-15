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

const storeStatisticsResponseSchema = z.array(listingStoreStatisticsSchema);

export type ListingStoreStatistics = z.infer<
  typeof listingStoreStatisticsSchema
>;

export type ListingStoreStatisticsQuery = QueueQuery & {
  date?: string;
};

export function parseStoreStatisticsResponse(
  payload: unknown,
): ListingStoreStatistics[] {
  return parseApiResponseShape(
    payload,
    storeStatisticsResponseSchema,
    "ListingKit API returned an unexpected store statistics response",
  );
}

export async function getListingStoreStatistics(
  query: ListingStoreStatisticsQuery = {},
): Promise<ListingStoreStatistics[]> {
  const payload = await apiRequest<unknown>("/admin/store-statistics", {
    query,
  });
  return parseStoreStatisticsResponse(payload);
}
