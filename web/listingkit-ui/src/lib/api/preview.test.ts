import { describe, expect, it, vi } from "vitest";

import { getListingKitPreview } from "@/lib/api/preview";

vi.mock("@/lib/api/client", () => ({
  apiRequest: vi.fn().mockResolvedValue({
    task_id: "task-1",
    status: "completed",
    selected_platform: "shein",
  }),
}));

describe("getListingKitPreview", () => {
  it("keeps runtime Zod validation around the generated preview contract", async () => {
    await expect(getListingKitPreview("task-1")).resolves.toMatchObject({
      task_id: "task-1",
      selected_platform: "shein",
    });
  });
});
