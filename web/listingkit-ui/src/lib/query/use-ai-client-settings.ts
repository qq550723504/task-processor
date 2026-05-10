"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  getAIClientSettings,
  updateAIClientSettings,
} from "@/lib/api/ai-client-settings";
import type { AIClientSettings } from "@/lib/types/listingkit";

const aiClientSettingsKey = (scope: string, clientName: string, userId = "") =>
  ["listingkit", "ai-client-settings", scope, clientName, userId] as const;

export function useAIClientSettings(
  scope = "tenant",
  clientName = "default",
  userId = "",
) {
  return useQuery({
    queryKey: aiClientSettingsKey(scope, clientName, userId),
    queryFn: () => getAIClientSettings(scope, clientName, userId),
  });
}

export function useUpdateAIClientSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: AIClientSettings) => updateAIClientSettings(request),
    onSuccess: async (settings, request) => {
      await client.invalidateQueries({
        queryKey: aiClientSettingsKey(
          settings.scope ?? "tenant",
          settings.client_name ?? "default",
          request.user_id ?? "",
        ),
      });
    },
  });
}
