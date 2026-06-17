import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { QueueTable } from "@/components/listingkit/queue/queue-table";

describe("QueueTable", () => {
  it("renders scene preset summary when queue item includes it", () => {
    const onAction = vi.fn();

    render(
      <QueueTable
        items={[
          {
            platform: "amazon",
            slot: "main",
            state: "ready",
            render_preview_available: true,
            quality_grade_label: "Exact asset",
            review_status: "pending",
            retry_hint: "review ready",
            generation_task: "task-1",
            scene_preset: {
              scene_category: "shoes",
              defaults_source: "platform_category",
              scene_style: "studio",
            },
          },
        ]}
        onAction={onAction}
      />,
    );

    expect(screen.getByText("Shoes")).toBeInTheDocument();
    expect(
      screen.getByText("Platform + category default · Studio"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /review/i }));
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it("surfaces action semantics ownership and failure review fields", () => {
    render(
      <QueueTable
        items={[
          {
            platform: "shein",
            slot: "gallery",
            state: "failed",
            render_preview_available: false,
            retryable: false,
            quality_grade: "missing",
            execution_quality: "failed",
            retry_hint: "no_retry",
            generation_task: "gen-task-9",
          },
        ]}
        onAction={vi.fn()}
      />,
    );

    expect(screen.getByText("工程介入候选")).toBeInTheDocument();
    expect(screen.getAllByText("Inspect").length).toBeGreaterThanOrEqual(2);
    expect(
      screen.getByText(/复盘字段：Platform=shein · Slot=gallery · Generation task=gen-task-9/),
    ).toBeInTheDocument();
  });
});
