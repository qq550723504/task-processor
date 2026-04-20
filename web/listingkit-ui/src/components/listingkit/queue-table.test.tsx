import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { QueueTable } from "@/components/listingkit/queue-table";

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
});
