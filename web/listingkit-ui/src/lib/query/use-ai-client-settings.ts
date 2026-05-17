"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  getAIClientSettings,
  updateAIClientSettings,
} from "@/lib/api/ai-client-settings";
import { listingKitSettingsKeys } from "@/lib/query/listingkit-settings";
import type { AIClientSettings } from "@/lib/types/listingkit";

export function useAIClientSettings(scope = "tenant", clientName = "default") {
  return useQuery({
    queryKey: listingKitSettingsKeys.aiClient(scope, clientName),
    queryFn: () => getAIClientSettings(scope, clientName),
  });
}

export function useUpdateAIClientSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: AIClientSettings) => updateAIClientSettings(request),
    onSuccess: async (settings, request) => {
      await client.invalidateQueries({
        queryKey: listingKitSettingsKeys.aiClient(
          settings.scope ?? "tenant",
          settings.client_name ?? "default",
        ),
      });
    },
  });
}
