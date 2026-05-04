import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function shouldAutoOpenWorkspace(task?: ListingKitTaskResult | null) {
  return (
    task?.status === "completed" ||
    task?.status === "needs_review" ||
    task?.status === "succeeded" ||
    task?.status === "review_ready"
  );
}
