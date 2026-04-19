import type { QueueQuery } from "@/lib/types/listingkit";

export const listingKitKeys = {
  preview: (taskId: string) => ["listingkit", taskId, "preview"] as const,
  taskResult: (taskId: string) => ["listingkit", taskId, "task-result"] as const,
  queue: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "queue", query] as const,
  reviewSession: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "review-session", query] as const,
  reviewPreview: (taskId: string, query: QueueQuery) =>
    ["listingkit", taskId, "review-preview", query] as const,
};
