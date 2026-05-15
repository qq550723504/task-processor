import { render, screen } from "@testing-library/react";

import { ListingKitHomeTaskCard } from "@/components/listingkit/home/listingkit-home-task-card";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

function makeTask(
  overrides: Partial<ListingKitTaskListItem> = {},
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

describe("ListingKitHomeTaskCard", () => {
  it("renders the preferred task title and contextual workspace link", () => {
    render(
      <ListingKitHomeTaskCard
        task={makeTask({
          task_id: "task-resume",
          product_name: "Botanical clock",
          platforms: ["shein"],
        })}
      />,
    );

    expect(screen.getByText("Botanical clock")).toBeInTheDocument();
    expect(screen.getByText("SHEIN")).toBeInTheDocument();
    expect(screen.getByText("已完成")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "继续处理 Botanical clock" }),
    ).toHaveAttribute("href", "/listing-kits/task-resume/workspace?platform=shein");
  });

  it("shows the SHEIN workflow badge when present", () => {
    render(
      <ListingKitHomeTaskCard
        task={makeTask({
          status: "completed",
          shein_workflow_status: "draft_saved",
        })}
      />,
    );

    expect(screen.getByText("草稿已保存")).toBeInTheDocument();
  });

  it("shows the SHEIN remote submission status badge when present", () => {
    render(
      <ListingKitHomeTaskCard
        task={makeTask({
          shein_submission_remote_status: "pending",
        })}
      />,
    );

    expect(screen.getAllByText("待 SHEIN 确认").length).toBeGreaterThan(0);
  });

  it("shows shein work queues, action queues, and compact next-step guidance", () => {
    render(
      <ListingKitHomeTaskCard
        taxonomy={{
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
          ],
        }}
        task={makeTask({
          shein_work_queue: "repair_queue",
          shein_action_queue: "final_review_queue",
          shein_status_overview: {
            headline: "SHEIN 资料包暂不能直接提交",
            primary_action: "最终确认",
          },
        })}
      />,
    );

    expect(screen.getByText("修复队列")).toBeInTheDocument();
    expect(screen.getAllByText("最终确认").length).toBeGreaterThan(0);
    expect(screen.getByText("下一步")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "继续处理 Task" })).toBeInTheDocument();
  });

  it("uses the SHEIN overview headline as the compact next step fallback", () => {
    render(
      <ListingKitHomeTaskCard
        taxonomy={{
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
          ],
        }}
        task={makeTask({
          shein_work_queue: "repair_queue",
          shein_action_queue: "final_review_queue",
          shein_status_overview: {
            headline: "SHEIN 资料包暂不能直接提交",
          },
        })}
      />,
    );

    expect(screen.getByText("修复队列")).toBeInTheDocument();
    expect(screen.getAllByText("最终确认").length).toBeGreaterThan(0);
    expect(screen.getByText("SHEIN 资料包暂不能直接提交")).toBeInTheDocument();
    expect(screen.getByText("下一步")).toBeInTheDocument();
  });

  it("uses the SHEIN workspace query for resumable mixed-platform tasks", () => {
    render(
      <ListingKitHomeTaskCard
        task={makeTask({
          task_id: "mixed-platform-task",
          title: "Mixed platform task",
          platforms: ["amazon", "shein"],
          shein_workflow_status: "draft_saved",
        })}
      />,
    );

    expect(
      screen.getByRole("link", { name: "继续处理 Mixed platform task" }),
    ).toHaveAttribute(
      "href",
      "/listing-kits/mixed-platform-task/workspace?platform=shein",
    );
  });

  it("omits the platform query when the task has no platform metadata", () => {
    render(
      <ListingKitHomeTaskCard
        task={makeTask({
          task_id: "task-without-platform",
          platforms: undefined,
          title: "Fallback task",
        })}
      />,
    );

    expect(
      screen.getByRole("link", { name: "继续处理 Fallback task" }),
    ).toHaveAttribute("href", "/listing-kits/task-without-platform/workspace");
    expect(screen.getByText("LISTINGKIT")).toBeInTheDocument();
  });
});
