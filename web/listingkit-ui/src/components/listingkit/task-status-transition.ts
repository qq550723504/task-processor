import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function shouldAutoOpenWorkspace(task?: ListingKitTaskResult | null) {
  return task?.status === "completed";
}
