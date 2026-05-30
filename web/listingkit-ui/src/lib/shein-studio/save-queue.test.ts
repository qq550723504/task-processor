import { describe, expect, it } from "vitest";

import { enqueueSheinStudioSave, resetSheinStudioSaveQueueForTest } from "@/lib/shein-studio/save-queue";

describe("enqueueSheinStudioSave", () => {
  it("serializes saves that target the same queue key", async () => {
    resetSheinStudioSaveQueueForTest();

    const steps: string[] = [];
    let releaseFirst = () => undefined;
    const firstGate = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });

    const first = enqueueSheinStudioSave("batch:1", async () => {
      steps.push("first:start");
      await firstGate;
      steps.push("first:end");
      return "first";
    });
    const second = enqueueSheinStudioSave("batch:1", async () => {
      steps.push("second:start");
      steps.push("second:end");
      return "second";
    });

    await Promise.resolve();
    expect(steps).toEqual(["first:start"]);

    releaseFirst();

    await expect(first).resolves.toBe("first");
    await expect(second).resolves.toBe("second");
    expect(steps).toEqual([
      "first:start",
      "first:end",
      "second:start",
      "second:end",
    ]);
  });
});
