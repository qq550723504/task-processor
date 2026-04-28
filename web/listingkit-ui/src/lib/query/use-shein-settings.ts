"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  getSheinSettings,
  updateSheinSettings,
} from "@/lib/api/shein-settings";
import type { SheinSettings } from "@/lib/types/listingkit";

const sheinSettingsKey = ["listingkit", "shein-settings"] as const;

export function useSheinSettings() {
  return useQuery({
    queryKey: sheinSettingsKey,
    queryFn: getSheinSettings,
  });
}

export function useUpdateSheinSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: SheinSettings) => updateSheinSettings(request),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: sheinSettingsKey });
    },
  });
}
