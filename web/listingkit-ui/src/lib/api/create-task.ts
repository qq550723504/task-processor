import { apiRequest } from "@/lib/api/client";
import type {
  CreateListingKitTaskRequest,
  CreateListingKitTaskResponse,
} from "@/lib/types/listingkit";

export function createListingKitTask(body: CreateListingKitTaskRequest) {
  return apiRequest<CreateListingKitTaskResponse>("/generate", {
    method: "POST",
    body,
    timeoutMs: 3_600_000,
  });
}
