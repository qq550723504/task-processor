import type { ListingKitMockBundle } from "@/app/api/listing-kits/mock-types";

export function selectListingKitMockPayload({
  bundle,
  method,
  path,
}: {
  bundle: ListingKitMockBundle;
  method: string;
  path: string[];
}) {
  const endpoint = path[path.length - 1];
  return method === "POST" && endpoint === "execute"
    ? bundle.action
    : method === "POST"
      ? bundle.dispatch
      : endpoint === "generation-queue"
        ? bundle.queue
        : endpoint === "generation-review-session"
          ? bundle.reviewSession
          : endpoint === "generation-review-preview"
            ? bundle.reviewPreview
            : path.length === 2 && path[0] === "tasks"
              ? bundle.taskResult
              : bundle.preview;
}
