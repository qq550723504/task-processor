import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingImportTaskSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    storeId: z.number().optional(),
    platform: z.string(),
    targetPlatform: z.string().optional(),
    sourcePlatform: z.string().optional(),
    region: z.string().optional(),
    categoryId: z.number().optional(),
    productId: z.string(),
    status: z.number(),
    errorMessage: z.string().optional(),
    error_message: z.string().optional(),
    reasonCode: z.string().optional(),
    reason_code: z.string().optional(),
    stage: z.string().optional(),
    retryCount: z.number().optional(),
    retry_count: z.number().optional(),
    maxRetryCount: z.number().optional(),
    max_retry_count: z.number().optional(),
    remark: z.string().optional(),
    priority: z.number().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const importTaskPageSchema = z
  .object({
    items: z.array(listingImportTaskSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

const batchCreateImportTaskResponseSchema = z
  .object({
    createdCount: z.number(),
    items: z.array(listingImportTaskSchema),
  })
  .passthrough();

export type ListingImportTask = z.infer<typeof listingImportTaskSchema>;
export type ListingImportTaskPage = z.infer<typeof importTaskPageSchema>;
export type BatchCreateListingImportTaskResponse = z.infer<
  typeof batchCreateImportTaskResponseSchema
>;

export type ListingImportTaskQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  storeId?: number;
  platform?: string;
  region?: string;
  categoryId?: number;
  productId?: string;
  status?: number;
};

export type BatchCreateListingImportTaskInput = {
  storeId: number;
  categoryId: number;
  platform: string;
  targetPlatform?: string;
  region?: string;
  priority?: number;
  productIds: string[];
  remark?: string;
};

export function parseImportTaskPageResponse(
  payload: unknown,
): ListingImportTaskPage {
  return parseApiResponseShape(
    payload,
    importTaskPageSchema,
    "ListingKit API returned an unexpected import task page response",
  );
}

export function parseBatchCreateImportTaskResponse(
  payload: unknown,
): BatchCreateListingImportTaskResponse {
  return parseApiResponseShape(
    payload,
    batchCreateImportTaskResponseSchema,
    "ListingKit API returned an unexpected import task batch response",
  );
}

export async function getListingImportTasks(
  query: ListingImportTaskQuery = {},
): Promise<ListingImportTaskPage> {
  const payload = await apiRequest<unknown>("/admin/import-tasks", { query });
  return parseImportTaskPageResponse(payload);
}

export async function batchCreateListingImportTasks(
  input: BatchCreateListingImportTaskInput,
): Promise<BatchCreateListingImportTaskResponse> {
  const payload = await apiRequest<unknown>("/admin/import-tasks/batch", {
    method: "POST",
    body: input,
  });
  return parseBatchCreateImportTaskResponse(payload);
}

export async function deleteListingImportTask(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/import-tasks/${id}`, { method: "DELETE" });
}
