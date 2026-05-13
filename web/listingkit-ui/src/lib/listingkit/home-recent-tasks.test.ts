import { describe, expect, it } from "vitest";

import {
  pickContinueTask,
  sortRecentTasksForHomepage,
} from "@/lib/listingkit/home-recent-tasks";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

function makeTask(
  overrides: Partial<ListingKitTaskListItem>,
): ListingKitTaskListItem {
  return {
    task_id: "task-1",
    status: "completed",
    platforms: ["shein"],
    title: "Task",
    image_count: 0,
    created_at: "2026-04-30T10:00:00+08:00",
    updated_at: "2026-04-30T10:00:00+08:00",
    ...overrides,
  };
}

describe("home-recent-tasks", () => {
  it("prefers the newest actionable SHEIN task for continue", () => {
    const tasks = [
      makeTask({
        task_id: "done-amazon",
        platforms: ["amazon"],
        status: "completed",
        updated_at: "2026-04-30T09:00:00+08:00",
      }),
      makeTask({
        task_id: "needs-review-shein",
        platforms: ["shein"],
        status: "needs_review",
        updated_at: "2026-04-30T11:00:00+08:00",
      }),
      makeTask({
        task_id: "processing-temu",
        platforms: ["temu"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("needs-review-shein");
  });

  it("falls back to newest incomplete task when no actionable SHEIN task exists", () => {
    const tasks = [
      makeTask({
        task_id: "processing-amazon",
        platforms: ["amazon"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
      makeTask({
        task_id: "done-shein",
        platforms: ["shein"],
        status: "completed",
        updated_at: "2026-04-30T13:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("processing-amazon");
  });

  it("prefers a completed SHEIN task with resumable workflow state over a non-SHEIN actionable task", () => {
    const tasks = [
      makeTask({
        task_id: "processing-amazon",
        platforms: ["amazon"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
      makeTask({
        task_id: "resume-shein-draft",
        platforms: ["shein"],
        status: "completed",
        shein_workflow_status: "draft_saved",
        updated_at: "2026-04-30T11:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("resume-shein-draft");
  });

  it("treats shein work queue as resumable even without legacy workflow status", () => {
    const tasks = [
      makeTask({
        task_id: "processing-amazon",
        platforms: ["amazon"],
        status: "processing",
        updated_at: "2026-04-30T12:00:00+08:00",
      }),
      makeTask({
        task_id: "resume-shein-repair",
        platforms: ["shein"],
        status: "completed",
        shein_work_queue: "repair_queue",
        updated_at: "2026-04-30T11:00:00+08:00",
      }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("resume-shein-repair");
  });

  it("returns newest overall task when all tasks are completed", () => {
    const tasks = [
      makeTask({ task_id: "older", updated_at: "2026-04-30T09:00:00+08:00" }),
      makeTask({ task_id: "newer", updated_at: "2026-04-30T10:00:00+08:00" }),
    ];

    expect(pickContinueTask(tasks)?.task_id).toBe("newer");
  });

  it("caps recent tasks to the newest three after sorting", () => {
    const tasks = ["1", "2", "3", "4"].map((id, index) =>
      makeTask({
        task_id: id,
        updated_at: `2026-04-30T1${index}:00:00+08:00`,
      }),
    );

    expect(sortRecentTasksForHomepage(tasks).map((task) => task.task_id)).toEqual([
      "4",
      "3",
      "2",
    ]);
  });
});
