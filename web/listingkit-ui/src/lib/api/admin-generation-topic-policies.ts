import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingGenerationTopicPolicySchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    platform: z.string(),
    topicKey: z.string(),
    remark: z.string().optional(),
    status: z.number(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const generationTopicPolicyPageSchema = z
  .object({
    items: z.array(listingGenerationTopicPolicySchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingGenerationTopicPolicy = z.infer<
  typeof listingGenerationTopicPolicySchema
>;
export type ListingGenerationTopicPolicyPage = z.infer<
  typeof generationTopicPolicyPageSchema
>;

export type ListingGenerationTopicPolicyQuery = Omit<QueueQuery, "status"> & {
  page?: number;
  page_size?: number;
  platform?: string;
  topic_key?: string;
  status?: string;
  remark?: string;
};

export type ListingGenerationTopicPolicyInput = {
  platform: string;
  topicKey: string;
  remark?: string;
  status?: number;
};

export function parseGenerationTopicPolicyPageResponse(
  payload: unknown,
): ListingGenerationTopicPolicyPage {
  return parseApiResponseShape(
    payload,
    generationTopicPolicyPageSchema,
    "ListingKit API returned an unexpected generation topic policy page response",
  );
}

export function parseGenerationTopicPolicyResponse(
  payload: unknown,
): ListingGenerationTopicPolicy {
  return parseApiResponseShape(
    payload,
    listingGenerationTopicPolicySchema,
    "ListingKit API returned an unexpected generation topic policy response",
  );
}

export async function getListingGenerationTopicPolicies(
  query: ListingGenerationTopicPolicyQuery = {},
): Promise<ListingGenerationTopicPolicyPage> {
  const payload = await apiRequest<unknown>("/admin/generation-topic-policies", {
    query,
  });
  return parseGenerationTopicPolicyPageResponse(payload);
}

export async function createListingGenerationTopicPolicy(
  input: ListingGenerationTopicPolicyInput,
): Promise<ListingGenerationTopicPolicy> {
  const payload = await apiRequest<unknown>("/admin/generation-topic-policies", {
    method: "POST",
    body: input,
  });
  return parseGenerationTopicPolicyResponse(payload);
}

export async function updateListingGenerationTopicPolicyStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingGenerationTopicPolicy> {
  const payload = await apiRequest<unknown>(
    `/admin/generation-topic-policies/${id}/status`,
    {
      method: "PATCH",
      body: { status, remark },
    },
  );
  return parseGenerationTopicPolicyResponse(payload);
}

export async function deleteListingGenerationTopicPolicy(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/generation-topic-policies/${id}`, {
    method: "DELETE",
  });
}
