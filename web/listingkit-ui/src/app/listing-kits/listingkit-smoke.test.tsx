import { describe, expect, it, vi } from "vitest";

const push = vi.fn();

vi.mock("@/auth", () => ({
  auth: vi.fn(async () => null),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

import { selectListingKitMockPayload } from "@/app/api/listing-kits/proxy-mock";

describe("ListingKit lightweight smoke", () => {
  it("keeps the local mock selector wired for ListingKit task routes", () => {
    const payload = selectListingKitMockPayload({
      method: "GET",
      path: ["tasks", "task-1", "preview"],
      bundle: {
        action: { action_key: "noop" },
        createTask: { task_id: "task-1" },
        dispatch: { dispatch_kind: "review_session" },
        preview: {
          task_id: "task-1",
          status: "completed",
          needs_review: false,
          created_at: "2026-04-19T00:00:00Z",
        },
        queue: { task_id: "task-1", page: 1, page_size: 20, total: 0 },
        reviewPreview: { task_id: "task-1" },
        reviewSession: { task_id: "task-1" },
        taskResult: { task_id: "task-1", status: "completed" },
      },
    });

    expect(payload).toEqual({
      task_id: "task-1",
      status: "completed",
      needs_review: false,
      created_at: "2026-04-19T00:00:00Z",
    });
  });
});
