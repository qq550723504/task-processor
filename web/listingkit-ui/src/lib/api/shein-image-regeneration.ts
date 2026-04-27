import { apiRequest } from "@/lib/api/client";
import type { ListingKitPreview } from "@/lib/types/listingkit";

export type RegenerateSheinDataImageRequest = {
  image_url: string;
  label?: string;
  role?: string;
  prompt: string;
};

export type RegenerateSheinDataImageResponse = {
  preview?: ListingKitPreview;
  image?: {
    id?: string;
    image_url?: string;
    revised_prompt?: string;
    role?: string;
    role_label?: string;
  };
  replaced_url?: string;
};

export function regenerateSheinDataImage(
  taskId: string,
  body: RegenerateSheinDataImageRequest,
) {
  return apiRequest<RegenerateSheinDataImageResponse>(
    `/tasks/${taskId}/shein-images/regenerate`,
    {
      method: "POST",
      body,
      timeoutMs: 180000,
    },
  );
}
