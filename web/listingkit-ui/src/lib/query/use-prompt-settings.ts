"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  listPromptSettings,
  setPromptSettingStatus,
  upsertPromptSetting,
} from "@/lib/api/prompt-settings";
import type { PromptSetting } from "@/lib/types/listingkit";

const promptSettingsKey = ["listingkit", "prompt-settings"] as const;

export function usePromptSettings() {
  return useQuery({
    queryKey: promptSettingsKey,
    queryFn: listPromptSettings,
  });
}

export function useUpsertPromptSetting() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (request: PromptSetting) => upsertPromptSetting(request),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: promptSettingsKey });
    },
  });
}

export function useSetPromptSettingStatus() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      setPromptSettingStatus(key, enabled),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: promptSettingsKey });
    },
  });
}
