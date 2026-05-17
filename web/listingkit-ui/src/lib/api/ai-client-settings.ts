import { getListingKitSettings, updateListingKitSettings } from "@/lib/api/listingkit-settings";
import type { AIClientSettings } from "@/lib/types/listingkit";

export function getAIClientSettings(scope = "tenant", clientName = "default") {
  return getListingKitSettings<AIClientSettings>("ai", {
    scope,
    client_name: clientName,
  });
}

export function updateAIClientSettings(body: AIClientSettings) {
  return updateListingKitSettings<AIClientSettings>("ai", body);
}
