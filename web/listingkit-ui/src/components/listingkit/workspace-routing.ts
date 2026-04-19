import type { ReviewTarget } from "@/lib/types/listingkit";

export function buildWorkspaceSearch(
  currentSearch: string,
  target?: ReviewTarget | null,
) {
  const params = new URLSearchParams(currentSearch);

  const nextValues: Record<string, string | undefined> = {
    platform: target?.platform,
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
