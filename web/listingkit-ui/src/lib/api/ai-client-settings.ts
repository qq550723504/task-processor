import { apiRequest } from "@/lib/api/client";
import type { AIClientSettings } from "@/lib/types/listingkit";

export function getAIClientSettings(
  scope = "tenant",
  clientName = "default",
  userId = "",
) {
  return apiRequest<AIClientSettings>("/settings/ai", {
    query: {
      scope,
      client_name: clientName,
      user_id: userId || undefined,
    },
  });
}

export function updateAIClientSettings(body: AIClientSettings) {
  const { user_id: userId, ...payload } = body;
  return apiRequest<AIClientSettings>("/settings/ai", {
    method: "PUT",
    query: {
      user_id: userId || undefined,
    },
    body: payload,
  });
}
