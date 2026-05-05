import type { ReviewTarget } from "@/lib/types/listingkit";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

function selectedPlatformFromTarget(target?: ReviewTarget | null) {
  const panelState = (target as ReviewTarget & {
    panel_state?: { selected_platform?: string };
  } | null)?.panel_state;
  return target?.platform ?? panelState?.selected_platform;
}

export function buildWorkspaceSearch(
  currentSearch: string,
  target?: ReviewTarget | null,
) {
  const params = sanitizedNavigationSearchParams(
    new URLSearchParams(currentSearch),
  );

  const nextValues: Record<string, string | undefined> = {
    platform: selectedPlatformFromTarget(target),
    slot: target?.slot,
    preview_capability: target?.capability,
    section_key: target?.section_key,
  };

  Object.entries(nextValues).forEach(([key, value]) => {
    if (!value) {
      params.delete(key);
      return;
    }
    params.set(key, value);
  });

  return params.toString();
}
