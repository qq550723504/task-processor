import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingPricingRuleSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    name: z.string(),
    ruleCode: z.string(),
    description: z.string().optional(),
    remark: z.string().optional(),
    storeId: z.number().optional(),
    categoryId: z.number().optional(),
    priceMin: z.number(),
    priceMax: z.number(),
    ruleType: z.string(),
    ruleValue: z.number(),
    fixedValue: z.number().optional(),
    acceptCondition: z.string().optional(),
    rejectCondition: z.string().optional(),
    status: z.number(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const pricingRulePageSchema = z
  .object({
    items: z.array(listingPricingRuleSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingPricingRule = z.infer<typeof listingPricingRuleSchema>;
export type ListingPricingRulePage = z.infer<typeof pricingRulePageSchema>;

export type ListingPricingRuleQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  name?: string;
  ruleCode?: string;
  storeId?: number;
  categoryId?: number;
  ruleType?: string;
  status?: string;
};

export type ListingPricingRuleInput = {
  name: string;
  ruleCode: string;
  description?: string;
  remark?: string;
  storeId?: number;
  categoryId?: number;
  priceMin?: number;
  priceMax?: number;
  ruleType: string;
  ruleValue?: number;
  fixedValue?: number;
  acceptCondition?: string;
  rejectCondition?: string;
  status?: number;
};

export function parsePricingRulePageResponse(
  payload: unknown,
): ListingPricingRulePage {
  return parseApiResponseShape(
    payload,
    pricingRulePageSchema,
    "ListingKit API returned an unexpected pricing rule page response",
  );
}

export function parsePricingRuleResponse(payload: unknown): ListingPricingRule {
  return parseApiResponseShape(
    payload,
    listingPricingRuleSchema,
    "ListingKit API returned an unexpected pricing rule response",
  );
}

export async function getListingPricingRules(
  query: ListingPricingRuleQuery = {},
): Promise<ListingPricingRulePage> {
  const payload = await apiRequest<unknown>("/admin/pricing-rules", { query });
  return parsePricingRulePageResponse(payload);
}

export async function createListingPricingRule(
  input: ListingPricingRuleInput,
): Promise<ListingPricingRule> {
  const payload = await apiRequest<unknown>("/admin/pricing-rules", {
    method: "POST",
    body: input,
  });
  return parsePricingRuleResponse(payload);
}

export async function updateListingPricingRuleStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingPricingRule> {
  const payload = await apiRequest<unknown>(`/admin/pricing-rules/${id}/status`, {
    method: "PATCH",
    body: { status, remark },
  });
  return parsePricingRuleResponse(payload);
}

export async function deleteListingPricingRule(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/pricing-rules/${id}`, { method: "DELETE" });
}
