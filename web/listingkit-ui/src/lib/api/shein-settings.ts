import { getListingKitSettings, updateListingKitSettings } from "@/lib/api/listingkit-settings";
import type { SheinSettings } from "@/lib/types/listingkit";

export function getSheinSettings() {
  return getListingKitSettings<SheinSettings>("shein");
}

export function updateSheinSettings(body: SheinSettings) {
  return updateListingKitSettings<SheinSettings>("shein", body);
}
