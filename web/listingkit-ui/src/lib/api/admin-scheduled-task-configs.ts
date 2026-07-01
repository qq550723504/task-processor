import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingScheduledTaskConfigSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    storeId: z.number(),
    platform: z.string(),
    taskType: z.string(),
    enabled: z.boolean(),
    intervalSeconds: z.number(),
    remark: z.string().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const scheduledTaskConfigPageSchema = z
  .object({
    items: z.array(listingScheduledTaskConfigSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingScheduledTaskConfig = z.infer<
  typeof listingScheduledTaskConfigSchema
>;
export type ListingScheduledTaskConfigPage = z.infer<
  typeof scheduledTaskConfigPageSchema
>;

export type ListingScheduledTaskConfigQuery = Omit<QueueQuery, "status"> & {
  page?: number;
  page_size?: number;
  storeId?: number;
  platform?: string;
  taskType?: string;
  enabled?: boolean;
};

export type ListingScheduledTaskConfigInput = {
  storeId: number;
  platform: string;
  taskType: string;
  enabled: boolean;
  intervalSeconds: number;
  remark?: string;
};

export function parseScheduledTaskConfigPageResponse(
  payload: unknown,
): ListingScheduledTaskConfigPage {
  return parseApiResponseShape(
    payload,
    scheduledTaskConfigPageSchema,
    "ListingKit API returned an unexpected scheduled task config page response",
  );
}

export function parseScheduledTaskConfigResponse(
  payload: unknown,
): ListingScheduledTaskConfig {
  return parseApiResponseShape(
    payload,
    listingScheduledTaskConfigSchema,
    "ListingKit API returned an unexpected scheduled task config response",
  );
}

export async function getListingScheduledTaskConfigs(
  query: ListingScheduledTaskConfigQuery = {},
): Promise<ListingScheduledTaskConfigPage> {
  const payload = await apiRequest<unknown>("/admin/scheduled-task-configs", {
    query,
  });
  return parseScheduledTaskConfigPageResponse(payload);
}

export async function upsertListingScheduledTaskConfig(
  input: ListingScheduledTaskConfigInput,
): Promise<ListingScheduledTaskConfig> {
  const payload = await apiRequest<unknown>("/admin/scheduled-task-configs", {
    method: "POST",
    body: input,
  });
  return parseScheduledTaskConfigResponse(payload);
}

export async function updateListingScheduledTaskConfigStatus(
  id: number,
  enabled: boolean,
  remark?: string,
): Promise<ListingScheduledTaskConfig> {
  const payload = await apiRequest<unknown>(
    `/admin/scheduled-task-configs/${id}/status`,
    {
      method: "PATCH",
      body: { enabled, remark },
    },
  );
  return parseScheduledTaskConfigResponse(payload);
}

export async function deleteListingScheduledTaskConfig(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/scheduled-task-configs/${id}`, {
    method: "DELETE",
  });
}
