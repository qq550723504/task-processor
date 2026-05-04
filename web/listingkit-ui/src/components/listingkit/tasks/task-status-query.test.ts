import { describe, expect, it } from "vitest";

import { shouldPollTaskResult } from "@/components/listingkit/tasks/task-status-query";
import { shouldAutoOpenWorkspace } from "@/components/listingkit/tasks/task-status-transition";

describe("shouldPollTaskResult", () => {
  it("polls pending and processing tasks", () => {
    expect(shouldPollTaskResult("pending")).toBe(true);
    expect(shouldPollTaskResult("processing")).toBe(true);
    expect(shouldPollTaskResult("queued")).toBe(true);
    expect(shouldPollTaskResult("running")).toBe(true);
  });

  it("stops polling terminal tasks", () => {
    expect(shouldPollTaskResult("failed")).toBe(false);
    expect(shouldPollTaskResult("completed")).toBe(false);
  });

  it("auto-opens workspace for completed and review-ready terminal tasks", () => {
    expect(shouldAutoOpenWorkspace({ status: "completed" })).toBe(true);
    expect(shouldAutoOpenWorkspace({ status: "succeeded" })).toBe(true);
    expect(shouldAutoOpenWorkspace({ status: "review_ready" })).toBe(true);
    expect(shouldAutoOpenWorkspace({ status: "failed" })).toBe(false);
    expect(shouldAutoOpenWorkspace({ status: "processing" })).toBe(false);
  });
});
