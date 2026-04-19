"use client";

import { useMutation } from "@tanstack/react-query";

import { uploadListingKitImages } from "@/lib/api/upload-images";

export function useUploadImages() {
  return useMutation({
    mutationFn: (files: File[]) => uploadListingKitImages(files),
  });
}
