import { beforeEach, describe, expect, it } from "vitest";

import {
  buildAsyncJobResumeKey,
  clearAsyncJobResumeEntry,
  loadAsyncJobResumeEntry,
  saveAsyncJobResumeEntry,
} from "@/lib/api/async-job-resume";

describe("async job resume storage", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("reuses the same key for the same request payload", () => {
    const keyA = buildAsyncJobResumeKey("/studio/designs", {
      prompt: "flag",
      count: 1,
    });
    const keyB = buildAsyncJobResumeKey("/studio/designs", {
      prompt: "flag",
      count: 1,
    });

    expect(keyA).toBe(keyB);
  });

  it("stores and loads a resumable async job id", () => {
    const key = buildAsyncJobResumeKey("/studio/designs", {
      prompt: "flag",
      count: 1,
    });

    saveAsyncJobResumeEntry(key, "job-123");

    expect(loadAsyncJobResumeEntry(key)?.jobId).toBe("job-123");
  });

  it("clears a resumable async job id", () => {
    const key = buildAsyncJobResumeKey("/studio/designs", {
      prompt: "flag",
      count: 1,
    });

    saveAsyncJobResumeEntry(key, "job-123");
    clearAsyncJobResumeEntry(key);

    expect(loadAsyncJobResumeEntry(key)).toBeNull();
  });
});
