import { apiRequest } from "@/lib/api/client";
import type { SheinSettings } from "@/lib/types/listingkit";

export function getSheinSettings() {
  return apiRequest<SheinSettings>("/settings/shein");
}

export function updateSheinSettings(body: SheinSettings) {
  return apiRequest<SheinSettings>("/settings/shein", {
    method: "PUT",
    body,
  });
}
