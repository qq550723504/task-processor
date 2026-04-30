import { describe, expect, it } from "vitest";

import { resolveSheinStudioEffectiveStep } from "@/lib/shein-studio/workbench-step";

describe("resolveSheinStudioEffectiveStep", () => {
  it("keeps explicit review and tasks steps", () => {
    expect(
      resolveSheinStudioEffectiveStep({
        activeStep: "review",
        designCount: 1,
        createdTaskCount: 1,
      }),
    ).toBe("review");
    expect(
      resolveSheinStudioEffectiveStep({
        activeStep: "tasks",
        designCount: 1,
        createdTaskCount: 1,
      }),
    ).toBe("tasks");
  });

  it("promotes generate to review when designs already exist", () => {
    expect(
      resolveSheinStudioEffectiveStep({
        activeStep: "generate",
        designCount: 1,
      }),
    ).toBe("review");
  });

  it("promotes generate to tasks when created tasks already exist", () => {
    expect(
      resolveSheinStudioEffectiveStep({
        activeStep: "generate",
        designCount: 1,
        createdTaskCount: 2,
      }),
    ).toBe("tasks");
  });
});
