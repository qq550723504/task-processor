import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

const dispatchEventWindowSchema = z
  .object({
    from: z.string(),
    to: z.string(),
  })
  .passthrough();

export const dispatchEventReasonCountSchema = z
  .object({
    reasonCode: z.string(),
    action: z.string(),
    count: z.number(),
  })
  .passthrough();

export const dispatchEventStoreBlockerSchema = z
  .object({
    tenantId: z.number(),
    storeId: z.number(),
    reasonCode: z.string(),
    count: z.number(),
    dailyLimit: z.number(),
    maxQueued: z.number(),
    maxProcessing: z.number(),
    maxCompletedToday: z.number(),
    ownerNode: z.string().optional(),
  })
  .passthrough();

export const dispatchEventSummarySchema = z
  .object({
    window: dispatchEventWindowSchema,
    total: z.number(),
    dispatched: z.number(),
    skipped: z.number(),
    failed: z.number(),
    reasonCounts: z.array(dispatchEventReasonCountSchema),
    storeBlockers: z.array(dispatchEventStoreBlockerSchema),
  })
  .passthrough();

export const dispatchEventItemSchema = z
  .object({
    id: z.number(),
    createdAt: z.string(),
    taskId: z.number(),
    tenantId: z.number(),
    storeId: z.number(),
    platform: z.string().optional(),
    action: z.string(),
    reasonCode: z.string().optional(),
    stage: z.string().optional(),
    capacity: z.number(),
    queued: z.number(),
    processing: z.number(),
    completedToday: z.number(),
    dailyLimit: z.number(),
    ownerNode: z.string().optional(),
  })
  .passthrough();

const dispatchEventPageSchema = z
  .object({
    items: z.array(dispatchEventItemSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number().optional(),
    pageSize: z.number().optional(),
    limit: z.number().optional(),
    offset: z.number(),
  })
  .passthrough()
  .transform((page) => ({
    ...page,
    pageSize: page.pageSize ?? page.page_size ?? page.limit ?? 50,
  }));

export type DispatchEventReasonCount = z.infer<
  typeof dispatchEventReasonCountSchema
>;
export type DispatchEventStoreBlocker = z.infer<
  typeof dispatchEventStoreBlockerSchema
>;
export type DispatchEventSummary = z.infer<typeof dispatchEventSummarySchema>;
export type DispatchEventItem = z.infer<typeof dispatchEventItemSchema>;
export type DispatchEventPage = z.infer<typeof dispatchEventPageSchema>;

export type DispatchEventQuery = QueueQuery & {
  platform?: string;
  tenantId?: string;
  storeId?: string;
  action?: string;
  reasonCode?: string;
  from?: string;
  to?: string;
  page?: number;
  page_size?: number;
};

export function parseDispatchEventSummaryResponse(
  payload: unknown,
): DispatchEventSummary {
  return parseApiResponseShape(
    payload,
    dispatchEventSummarySchema,
    "ListingKit API returned an unexpected dispatch event summary response",
  );
}

export function parseDispatchEventPageResponse(
  payload: unknown,
): DispatchEventPage {
  return parseApiResponseShape(
    payload,
    dispatchEventPageSchema,
    "ListingKit API returned an unexpected dispatch event page response",
  );
}

export async function getListingDispatchEventSummary(
  query: DispatchEventQuery = {},
): Promise<DispatchEventSummary> {
  const payload = await apiRequest<unknown>("/admin/dispatch-events/summary", {
    query: compactDispatchEventQuery(query),
  });
  return parseDispatchEventSummaryResponse(payload);
}

export async function listListingDispatchEvents(
  query: DispatchEventQuery = {},
): Promise<DispatchEventPage> {
  const payload = await apiRequest<unknown>("/admin/dispatch-events", {
    query: compactDispatchEventQuery(query),
  });
  return parseDispatchEventPageResponse(payload);
}

function compactDispatchEventQuery(query: DispatchEventQuery) {
  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => {
      if (value === undefined || value === null) {
        return false;
      }
      if (typeof value === "string") {
        return value.trim() !== "";
      }
      return true;
    }),
  );
}
