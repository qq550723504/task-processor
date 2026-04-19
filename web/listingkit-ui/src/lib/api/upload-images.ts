import { apiFormRequest } from "@/lib/api/client";
import type { UploadImagesResponse } from "@/lib/types/listingkit";

export async function uploadListingKitImages(files: File[]) {
  const formData = new FormData();
  files.forEach((file) => {
    formData.append("files", file);
  });

  return apiFormRequest<UploadImagesResponse>("/uploads/images", {
    formData,
  });
}
