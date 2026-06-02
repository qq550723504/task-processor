import { describe, expect, it } from "vitest";

import { buildTaskWorkspaceHref } from "@/lib/listingkit/task-workspace-href";

describe("buildTaskWorkspaceHref", () => {
  it("prefers shein workspace for resumable shein tasks even when shein is not the first platform", () => {
    expect(
      buildTaskWorkspaceHref({
        task_id: "task-1",
        platforms: ["pod", "shein"],
        shein_workflow_status: "pending_confirmation",
      }),
    ).toBe("/listing-kits/task-1/workspace?platform=shein");
  });

  it("falls back to the first platform when the task is not resumable in shein", () => {
    expect(
      buildTaskWorkspaceHref({
        task_id: "task-2",
        platforms: ["pod", "shein"],
        shein_workflow_status: "submitted",
      }),
    ).toBe("/listing-kits/task-2/workspace?platform=pod");
  });

  it("returns the base workspace path when no platform is available", () => {
    expect(
      buildTaskWorkspaceHref({
        task_id: "task-3",
      }),
    ).toBe("/listing-kits/task-3/workspace");
  });
});
