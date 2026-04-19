import type { ConditionalState } from "@/lib/types/listingkit";

export function nextConditional(
  current?: ConditionalState | null,
  fallbackToken?: string,
): ConditionalState | null {
  if (current?.etag || current?.delta_token) {
    return current;
  }

  if (fallbackToken) {
    return {
      delta_token: fallbackToken,
      etag: fallbackToken,
    };
  }

  return null;
}
