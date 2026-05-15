import { apiRequest } from "@/lib/api/client";
import type { PromptSetting, PromptSettingsResponse } from "@/lib/types/listingkit";

export function listPromptSettings() {
  return apiRequest<PromptSettingsResponse>("/settings/prompts");
}

export function upsertPromptSetting(body: PromptSetting) {
  return apiRequest<PromptSetting>("/settings/prompts", {
    method: "PUT",
    body,
  });
}

export function setPromptSettingStatus(key: string, enabled: boolean) {
  return apiRequest<{ key: string; enabled: boolean }>(
    `/settings/prompts/${encodeURIComponent(key)}/status`,
    {
      method: "PATCH",
      body: { enabled },
    },
  );
}
