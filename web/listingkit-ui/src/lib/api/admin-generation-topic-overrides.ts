import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import { z } from "zod";

const generationTopicDefinitionSchema = z
  .object({
    promptDirectives: z.array(z.string()).default([]),
    lexiconByLanguage: z.record(z.string(), z.array(z.string())).default({}),
  })
  .passthrough();

export const listingGenerationTopicOverrideSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    platform: z.string(),
    topicKey: z.string(),
    additionalPromptDirectives: z.array(z.string()).default([]),
    additionalLexiconByLanguage: z.record(z.string(), z.array(z.string())).default({}),
    remark: z.string().optional(),
    status: z.number(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const generationTopicOverrideViewSchema = z
  .object({
    id: z.number().optional(),
    status: z.number(),
    remark: z.string().optional(),
    additionalPromptDirectives: z.array(z.string()).default([]),
    additionalLexiconByLanguage: z.record(z.string(), z.array(z.string())).default({}),
  })
  .passthrough();

export const generationTopicCatalogItemSchema = z
  .object({
    key: z.string(),
    priority: z.number(),
    promptDirectives: z.array(z.string()).default([]),
    lexiconByLanguage: z.record(z.string(), z.array(z.string())).default({}),
    tenantOverride: generationTopicOverrideViewSchema.nullable(),
    effectiveDefinition: generationTopicDefinitionSchema,
  })
  .passthrough();

const generationTopicCatalogPageSchema = z
  .object({
    items: z.array(generationTopicCatalogItemSchema),
  })
  .passthrough();

export type ListingGenerationTopicCatalogItem = z.infer<
  typeof generationTopicCatalogItemSchema
>;
export type ListingGenerationTopicCatalogPage = z.infer<
  typeof generationTopicCatalogPageSchema
>;
export type ListingGenerationTopicOverride = z.infer<
  typeof listingGenerationTopicOverrideSchema
>;

export type ListingGenerationTopicCatalogQuery = {
  platform?: string;
};

export type ListingGenerationTopicOverrideInput = {
  platform: string;
  topicKey: string;
  additionalPromptDirectives?: string[];
  additionalLexiconByLanguage?: Record<string, string[]>;
  remark?: string;
  status?: number;
};

export function parseGenerationTopicCatalogResponse(
  payload: unknown,
): ListingGenerationTopicCatalogPage {
  return parseApiResponseShape(
    payload,
    generationTopicCatalogPageSchema,
    "ListingKit API returned an unexpected generation topic catalog response",
  );
}

export function parseGenerationTopicOverrideResponse(
  payload: unknown,
): ListingGenerationTopicOverride {
  return parseApiResponseShape(
    payload,
    listingGenerationTopicOverrideSchema,
    "ListingKit API returned an unexpected generation topic override response",
  );
}

export async function getListingGenerationTopicCatalog(
  query: ListingGenerationTopicCatalogQuery = {},
): Promise<ListingGenerationTopicCatalogPage> {
  const payload = await apiRequest<unknown>("/admin/generation-topic-catalog", {
    query,
  });
  return parseGenerationTopicCatalogResponse(payload);
}

export async function createListingGenerationTopicOverride(
  input: ListingGenerationTopicOverrideInput,
): Promise<ListingGenerationTopicOverride> {
  const payload = await apiRequest<unknown>("/admin/generation-topic-overrides", {
    method: "POST",
    body: input,
  });
  return parseGenerationTopicOverrideResponse(payload);
}

export async function updateListingGenerationTopicOverride(
  id: number,
  input: ListingGenerationTopicOverrideInput,
): Promise<ListingGenerationTopicOverride> {
  const payload = await apiRequest<unknown>(
    `/admin/generation-topic-overrides/${id}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseGenerationTopicOverrideResponse(payload);
}

export async function updateListingGenerationTopicOverrideStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingGenerationTopicOverride> {
  const payload = await apiRequest<unknown>(
    `/admin/generation-topic-overrides/${id}/status`,
    {
      method: "PATCH",
      body: { status, remark },
    },
  );
  return parseGenerationTopicOverrideResponse(payload);
}

export async function deleteListingGenerationTopicOverride(
  id: number,
): Promise<void> {
  await apiRequest<unknown>(`/admin/generation-topic-overrides/${id}`, {
    method: "DELETE",
  });
}
