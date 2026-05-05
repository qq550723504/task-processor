import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi } from "vitest";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";

describe("PlatformCardRail", () => {
  it("renders platform action and recovery summaries", () => {
    const onSelect = vi.fn();

    render(
      <PlatformCardRail
        cards={[
          {
            platform: "shein",
            status: "review_ready",
            summary: "2 previewable slots are ready.",
            resolved_action_summary: {
              title: "Review detail previews",
              cta_kind: "review",
            },
            recovery_summary: {
              title: "Use fallback review",
              severity: "medium",
              urgency: "now",
              recommended_descriptors: [
                {
                  platform: "shein",
                  recovery_hint: "review_fallback",
                  recovery_severity: "medium",
                  recovery_urgency: "now",
                  recovery_cta_kind: "review",
                },
              ],
            },
          },
        ]}
        onSelect={onSelect}
      />,
    );

    expect(screen.getAllByText("Review detail previews")).toHaveLength(2);
    expect(screen.getByText("使用兜底结果继续检查")).toBeInTheDocument();
    expect(screen.getByText("中优先级 / 立即处理")).toBeInTheDocument();
  });

  it("exposes separate review and recovery actions", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onSelectRecovery = vi.fn();

    render(
      <PlatformCardRail
        cards={[
          {
            platform: "shein",
            status: "review_ready",
            resolved_action_summary: {
              title: "Review detail previews",
              cta_kind: "review",
            },
            recovery_summary: {
              title: "Use fallback review",
              severity: "medium",
              urgency: "now",
              primary_descriptor: {
                platform: "shein",
                recovery_hint: "review_fallback",
              },
              recommended_descriptors: [
                {
                  platform: "shein",
                  recovery_hint: "review_fallback",
                  recovery_severity: "medium",
                  recovery_urgency: "now",
                  recovery_cta_kind: "review",
                },
              ],
            },
          },
        ]}
        onSelect={onSelect}
        onSelectRecovery={onSelectRecovery}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Review detail previews" }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelectRecovery).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "检查恢复项" }));
    expect(onSelectRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        recovery_hint: "review_fallback",
      }),
      expect.objectContaining({
        platform: "shein",
      }),
    );
  });

  it("uses a platform-matching recovery descriptor when the summary is shared", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onSelectRecovery = vi.fn();

    render(
      <PlatformCardRail
        cards={[
          {
            platform: "temu",
            status: "retry_needed",
            recovery_summary: {
              title: "Two recovery paths need attention",
              severity: "medium",
              urgency: "now",
              primary_descriptor: {
                platform: "shein",
                recovery_hint: "review_fallback",
              },
              recommended_descriptors: [
                {
                  platform: "shein",
                  recovery_hint: "review_fallback",
                  recovery_severity: "medium",
                  recovery_urgency: "now",
                  recovery_cta_kind: "review",
                },
                {
                  platform: "temu",
                  recovery_hint: "retry_dispatch",
                  recovery_severity: "high",
                  recovery_urgency: "now",
                  recovery_cta_kind: "retry",
                },
              ],
            },
          },
        ]}
        onSelect={onSelect}
        onSelectRecovery={onSelectRecovery}
      />,
    );

    expect(screen.getByText("重新生成当前内容")).toBeInTheDocument();
    expect(screen.getByText("高优先级 / 立即处理")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "立即重试" }));

    expect(onSelectRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: "temu",
        recovery_hint: "retry_dispatch",
      }),
      expect.objectContaining({
        platform: "temu",
      }),
    );
  });
});
