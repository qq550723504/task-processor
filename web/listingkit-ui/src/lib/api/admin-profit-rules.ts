import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

export const listingProfitRuleSchema = z
  .object({
    id: z.number(),
    tenantId: z.number().optional(),
    name: z.string(),
    ruleCode: z.string(),
    description: z.string().optional(),
    storeId: z.number().optional(),
    categoryId: z.number().optional(),
    salePriceMultiplier: z.number(),
    discountPriceMultiplier: z.number(),
    status: z.number(),
    remark: z.string().optional(),
    createTime: z.string().optional(),
    updateTime: z.string().optional(),
  })
  .passthrough();

const profitRulePageSchema = z
  .object({
    items: z.array(listingProfitRuleSchema),
    total: z.number(),
    page: z.number(),
    page_size: z.number(),
  })
  .passthrough();

export type ListingProfitRule = z.infer<typeof listingProfitRuleSchema>;
export type ListingProfitRulePage = z.infer<typeof profitRulePageSchema>;

export type ListingProfitRuleQuery = QueueQuery & {
  page?: number;
  page_size?: number;
  name?: string;
  ruleCode?: string;
  storeId?: number;
  categoryId?: number;
  status?: string;
};

export type ListingProfitRuleInput = {
  name: string;
  ruleCode: string;
  description?: string;
  storeId?: number;
  categoryId?: number;
  salePriceMultiplier?: number;
  discountPriceMultiplier?: number;
  status?: number;
  remark?: string;
};

export function parseProfitRulePageResponse(
  payload: unknown,
): ListingProfitRulePage {
  return parseApiResponseShape(
    payload,
    profitRulePageSchema,
    "ListingKit API returned an unexpected profit rule page response",
  );
}

export function parseProfitRuleResponse(payload: unknown): ListingProfitRule {
  return parseApiResponseShape(
    payload,
    listingProfitRuleSchema,
    "ListingKit API returned an unexpected profit rule response",
  );
}

export async function getListingProfitRules(
  query: ListingProfitRuleQuery = {},
): Promise<ListingProfitRulePage> {
  const payload = await apiRequest<unknown>("/admin/profit-rules", { query });
  return parseProfitRulePageResponse(payload);
}

export async function createListingProfitRule(
  input: ListingProfitRuleInput,
): Promise<ListingProfitRule> {
  const payload = await apiRequest<unknown>("/admin/profit-rules", {
    method: "POST",
    body: input,
  });
  return parseProfitRuleResponse(payload);
}

export async function updateListingProfitRuleStatus(
  id: number,
  status: number,
  remark?: string,
): Promise<ListingProfitRule> {
  const payload = await apiRequest<unknown>(`/admin/profit-rules/${id}/status`, {
    method: "PATCH",
    body: { status, remark },
  });
  return parseProfitRuleResponse(payload);
}

export async function deleteListingProfitRule(id: number): Promise<void> {
  await apiRequest<unknown>(`/admin/profit-rules/${id}`, { method: "DELETE" });
}
