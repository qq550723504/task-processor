import { apiRequest } from "@/lib/api/client";
import type { ListingKitPreview, SheinSizeAttribute } from "@/lib/types/listingkit";

export type UpdateSheinFinalDraftRequest = {
  confirmed?: boolean;
  submit_mode?: "publish" | "save_draft";
  manual_price_overrides?: Record<string, number>;
  final_image_order?: string[];
  main_image_url?: string;
  deleted_image_urls?: string[];
  image_role_overrides?: Record<string, "main" | "gallery" | "swatch" | "size_map" | "skc">;
  size_attribute_list?: SheinSizeAttribute[];
};

export function updateSheinFinalDraft(
  taskId: string,
  body: UpdateSheinFinalDraftRequest,
) {
  return apiRequest<ListingKitPreview>(
    `/tasks/${taskId}/shein/final-draft`,
    {
      method: "PATCH",
      body,
    },
  );
}
