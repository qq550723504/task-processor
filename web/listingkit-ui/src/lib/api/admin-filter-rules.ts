import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingFilterRuleSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    name: z.string(),
    ruleCode: z.string(),
    description: z.string().optional(),
    storeId: z.number().optional(),
    categoryId: z.number().optional(),
    priceType: z.string().optional(),
    priceMin: z.number(),
    priceMax: z.number(),
    stockMin: z.number(),
    ratingMin: z.number(),
    reviewCountMin: z.number(),
    deliveryTimeMax: z.number().optional(),
    fulfillmentType: z.string().optional(),
    status: z.number(),
    remark: z.string().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const filterRulePageSchema = z
  .object({
    items: z.array(listingFilterRuleSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingFilterRule = z.infer<typeof listingFilterRuleSchema>;
export type ListingFilterRulePage = z.infer<typeof filterRulePageSchema>;

export type ListingFilterRuleQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  name?: string;
  ruleCode?: string;
  storeId?: number;
  categoryId?: number;
  priceType?: string;
  fulfillmentType?: string;
  status?: string;
};

export type ListingFilterRuleInput = {
  name: string;
  ruleCode: string;
  description?: string;
  storeId?: number;
  categoryId?: number;
  priceType?: string;
  priceMin?: number;
  priceMax?: number;
  stockMin?: number;
  ratingMin?: number;
  reviewCountMin?: number;
  deliveryTimeMax?: number;
  fulfillmentType?: string;
  status?: number;
  remark?: string;
};

export function parseFilterRulePageResponse(
  payload: unknown,
): ListingFilterRulePage {
  return parseApiResponseShape(
    payload,
    filterRulePageSchema,
    "ListingKit API returned an unexpected filter rule page response",
  );
}

export function parseFilterRuleResponse(payload: unknown): ListingFilterRule {
  return parseApiResponseShape(
    payload,
    listingFilterRuleSchema,
    "ListingKit API returned an unexpected filter rule response",
  );
}

export async function getListingFilterRules(
  query: ListingFilterRuleQuery = {},
): Promise<ListingFilterRulePage> {
  const payload = await apiRequest<unknown>("/admin/filter-rules", { query });
  return parseFilterRulePageResponse(payload);
}

export async function createListingFilterRule(
  input: ListingFilterRuleInput,
): Promise<ListingFilterRule> {
  const payload = await apiRequest<unknown>("/admin/filter-rules", {
    method: "POST",
    body: input,
  });
  return parseFilterRuleResponse(payload);
}

export async function updateListingFilterRuleStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingFilterRule> {
  const payload = await apiRequest<unknown>(`/admin/filter-rules/${id}/status`, {
    method: "PATCH",
    body: { status, remark },
  });
  return parseFilterRuleResponse(payload);
}

export async function deleteListingFilterRule(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/filter-rules/${id}`, {
    method: "DELETE",
  });
}
