import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

type StoreOptionLike = Pick<ListingKitStoreProfile, "store_id" | "site" | "store"> & {
  name?: string;
  storeId?: string;
  region?: string;
};

export function formatSheinStoreOptionLabel(option: StoreOptionLike) {
  const primary =
    option.store?.name?.trim() ||
    option.name?.trim() ||
    option.store?.store_id?.trim() ||
    option.storeId?.trim() ||
    `店铺 ${option.store_id}`;
  const externalStoreId = option.store?.store_id?.trim() || option.storeId?.trim();
  const region = option.store?.region?.trim() || option.region?.trim();
  const site = option.site?.trim();
  const meta = [
    externalStoreId && externalStoreId !== primary ? externalStoreId : "",
    region,
    site && site !== region ? site : "",
  ]
    .filter(Boolean)
    .join(" / ");
  return meta ? `${primary} (${meta})` : primary;
}
