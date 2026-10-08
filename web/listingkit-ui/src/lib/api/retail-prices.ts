import { z } from "zod";
import { resourceOfferSchema, type ResourceOffer } from "./resource-purchase";

const retailPriceCatalog = z.object({
  schema_version: z.literal("retail-price-catalog-v1"),
  store_period_days: z.literal(30),
  items: z.array(resourceOfferSchema).max(100),
}).strict().refine(value => new Set(value.items.map(item => item.offer_id)).size === value.items.length);

export type RetailPriceCatalog = z.infer<typeof retailPriceCatalog>;
export function parseRetailPriceCatalog(value: unknown): RetailPriceCatalog | null {
  const result = retailPriceCatalog.safeParse(value);
  return result.success ? result.data : null;
}

// Multiple current offers require selection in the authenticated catalog.
// The public page must not silently advertise one as the universal price.
export function singleRetailOffer(catalog: RetailPriceCatalog | null, resource: ResourceOffer["resource_type"]) {
  const matches = catalog?.items.filter(item => item.resource_type === resource) ?? [];
  return matches.length === 1 ? matches[0] : null;
}

export function formatRetailMoney(minor: string) {
  const n = BigInt(minor), remainder = n % BigInt(100);
  return `¥${(n / BigInt(100)).toLocaleString("zh-CN")}${remainder ? `.${remainder.toString().padStart(2, "0")}` : ""}`;
}
