import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

type StoreOptionLike = Pick<ListingKitStoreProfile, "store_id" | "site" | "store">;

export function formatSheinStoreOptionLabel(option: StoreOptionLike) {
  const primary =
    option.store?.name?.trim() ||
    option.store?.store_id?.trim() ||
    `店铺 ${option.store_id}`;
  const externalStoreId = option.store?.store_id?.trim();
  const meta = [
    externalStoreId && externalStoreId !== primary ? externalStoreId : "",
    option.store?.region?.trim(),
    option.site?.trim(),
  ]
    .filter(Boolean)
    .join(" / ");
  return meta ? `${primary} (${meta})` : primary;
}
