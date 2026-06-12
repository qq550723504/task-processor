import { z } from "zod";

import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  ListingKitStoreProfile,
} from "@/lib/types/listingkit";

const storeOptionSchema = z
  .object({
    id: z.number(),
    store_id: z.string().optional(),
    name: z.string().optional(),
    platform: z.string().optional(),
    region: z.string().optional(),
  })
  .passthrough();

const pricingSchema = z
  .object({
    source_currency: z.string().optional(),
    target_currency: z.string().optional(),
    exchange_rate: z.number().optional(),
    markup_multiplier: z.number().optional(),
    minimum_price: z.number().optional(),
    round_to: z.number().optional(),
    price_ending: z.number().optional(),
  })
  .passthrough();

const matchRuleSchema = z
  .object({
    kind: z.string().optional(),
    values: z.array(z.string()).optional(),
  })
  .passthrough();

const storeProfileSchema = z
  .object({
    id: z.number().optional(),
    tenant_id: z.number().optional(),
    store_id: z.number(),
    enabled: z.boolean().optional(),
    priority: z.number().optional(),
    is_fallback: z.boolean().optional(),
    site: z.string().optional(),
    warehouse_code: z.string().optional(),
    default_stock: z.number().optional(),
    default_submit_mode: z.enum(["publish", "save_draft"]).optional(),
    pricing: pricingSchema.optional(),
    match_rules: z.array(matchRuleSchema).optional(),
    updated_at: z.string().optional(),
    store: storeOptionSchema.optional(),
  })
  .passthrough();

const storeProfileListSchema = z
  .object({
    items: z.array(storeProfileSchema),
  })
  .passthrough();

export function parseStoreProfilesResponse(
  payload: unknown,
): ListingKitStoreProfile[] {
  return parseApiResponseShape(
    payload,
    storeProfileListSchema,
    "ListingKit API returned an unexpected store profile response",
  ).items;
}

export async function getStoreProfiles(): Promise<ListingKitStoreProfile[]> {
  const payload = await apiRequest<unknown>("/store-profiles");
  return parseStoreProfilesResponse(payload);
}

export async function createStoreProfile(
  input: ListingKitStoreProfile,
): Promise<ListingKitStoreProfile> {
  const payload = await apiRequest<unknown>("/store-profiles", {
    method: "POST",
    body: input,
  });
  return parseApiResponseShape(
    payload,
    storeProfileSchema,
    "ListingKit API returned an unexpected store profile response",
  );
}

export async function updateStoreProfile(
  input: ListingKitStoreProfile,
): Promise<ListingKitStoreProfile> {
  const payload = await apiRequest<unknown>("/store-profiles", {
    method: "POST",
    body: input,
  });
  return parseApiResponseShape(
    payload,
    storeProfileSchema,
    "ListingKit API returned an unexpected store profile response",
  );
}

export async function deleteStoreProfile(id: number): Promise<void> {
  await apiRequest(`/store-profiles/${id}`, { method: "DELETE" });
}
