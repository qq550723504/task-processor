import { render, screen } from "@testing-library/react";

import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
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

describe("ListingKitHomeRecentWork", () => {
  it("renders a loading state while recent work is fetching", () => {
    render(<ListingKitHomeRecentWork isLoading isError={false} tasks={[]} />);

    expect(
      screen.getByRole("status", { name: "最近任务加载中" }),
    ).toBeInTheDocument();
  });

  it("prioritizes resumable SHEIN work for continue and shows only three recent tasks", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "amazon-processing",
            platforms: ["amazon"],
            status: "processing",
            title: "Amazon task",
            updated_at: "2026-04-30T11:00:00+08:00",
          }),
          makeTask({
            task_id: "resume-shein",
            status: "completed",
            shein_workflow_status: "draft_saved",
            title: "Resume me",
            updated_at: "2026-04-30T10:00:00+08:00",
          }),
          makeTask({
            task_id: "three",
            title: "Three",
            updated_at: "2026-04-30T09:00:00+08:00",
          }),
          makeTask({
            task_id: "four",
            title: "Four",
            updated_at: "2026-04-30T08:00:00+08:00",
          }),
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: "继续最近任务" })).toHaveAttribute(
      "href",
      "/listing-kits/resume-shein/workspace?platform=shein",
    );
    expect(screen.getAllByText("Resume me").length).toBeGreaterThan(0);
    expect(screen.getByText("Amazon task")).toBeInTheDocument();
    expect(screen.getByText("Three")).toBeInTheDocument();
    expect(screen.queryByText("Four")).not.toBeInTheDocument();
  });

  it("routes continue to the SHEIN workspace when a resumable SHEIN task has mixed platform order", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "mixed-platform-task",
            status: "completed",
            title: "Mixed platform task",
            platforms: ["amazon", "shein"],
            shein_workflow_status: "draft_saved",
          }),
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: "继续最近任务" })).toHaveAttribute(
      "href",
      "/listing-kits/mixed-platform-task/workspace?platform=shein",
    );
  });

  it("renders empty state when no tasks exist", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError={false} tasks={[]} />);

    expect(screen.getByText("还没有最近任务")).toBeInTheDocument();
  });

  it("renders inline error state without blocking the page when there is no stale data", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError tasks={[]} />);

    expect(screen.getByText("最近任务暂时加载失败")).toBeInTheDocument();
  });

  it("keeps rendering stale recent work while showing an inline warning", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError
        tasks={[
          makeTask({
            task_id: "resume-shein",
            status: "completed",
            shein_workflow_status: "draft_saved",
            title: "Resume me",
          }),
          makeTask({
            task_id: "stale-two",
            title: "Stale task",
            updated_at: "2026-04-30T09:00:00+08:00",
          }),
        ]}
      />,
    );

    expect(screen.getByText("最近任务暂时加载失败")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "继续最近任务" })).toBeInTheDocument();
    expect(screen.getAllByText("Resume me").length).toBeGreaterThan(0);
    expect(screen.getByText("Stale task")).toBeInTheDocument();
  });
});
