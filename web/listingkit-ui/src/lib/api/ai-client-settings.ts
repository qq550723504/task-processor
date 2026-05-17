import { getListingKitSettings, updateListingKitSettings } from "@/lib/api/listingkit-settings";
import type { AIClientSettings } from "@/lib/types/listingkit";

export function getAIClientSettings(
  scope = "tenant",
  clientName = "default",
  userId = "",
) {
  return getListingKitSettings<AIClientSettings>("ai", {
    scope,
    client_name: clientName,
    user_id: userId || undefined,
  });
}

export function updateAIClientSettings(body: AIClientSettings) {
  const { user_id: userId, ...payload } = body;
  return updateListingKitSettings<AIClientSettings>("ai", payload, {
    user_id: userId || undefined,
  });
}
