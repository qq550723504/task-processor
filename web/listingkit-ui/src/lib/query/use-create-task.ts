"use client";

import { useMutation } from "@tanstack/react-query";

import { createListingKitTask } from "@/lib/api/create-task";
import type { CreateListingKitTaskRequest } from "@/lib/types/listingkit";

export function useCreateTask() {
  return useMutation({
    mutationFn: (request: CreateListingKitTaskRequest) =>
      createListingKitTask(request),
  });
}
