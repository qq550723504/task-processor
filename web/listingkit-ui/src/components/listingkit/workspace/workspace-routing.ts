import type { ReviewTarget } from "@/lib/types/listingkit";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";
import type { ProductWorkspaceSectionKey } from "@/components/listingkit/workspace/product-workspace-model";

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

export function buildProductWorkspaceHref(
  taskId: string,
  currentSearch: string,
  section: ProductWorkspaceSectionKey,
) {
  const params = sanitizedNavigationSearchParams(
    new URLSearchParams(currentSearch),
  );
  params.delete("platform");
  params.delete("slot");
  params.delete("preview_capability");
  params.delete("section_key");

  if (section === "overview") {
    params.delete("product_section");
  } else {
    params.set("product_section", section);
  }

  const search = params.toString();
  return `/listing-kits/${taskId}/workspace${search ? `?${search}` : ""}`;
}

export function shouldSyncFocusedTargetToRoute(currentSearch: string) {
  const params = new URLSearchParams(currentSearch);
  return params.get("section_key") !== "final_review" && !params.has("product_section");
}
