import { fireEvent, render, screen } from "@testing-library/react";
import { vi } from "vitest";

import { RecoverySummaryCard } from "@/components/listingkit/recovery-summary-card";

describe("RecoverySummaryCard", () => {
  it("renders readable urgency metadata", () => {
    render(
      <RecoverySummaryCard
        summary={{
          title: "Use fallback review",
          summary: "A fallback result is available and should be reviewed first.",
          severity: "medium",
          urgency: "now",
          cta_kind: "review",
        }}
      />,
    );

    expect(screen.getByText("Use fallback review")).toBeInTheDocument();
    expect(screen.getByText("Medium severity / act now")).toBeInTheDocument();
    expect(screen.getByText("Review fallback")).toBeInTheDocument();
  });

  it("triggers the primary recovery action", () => {
    const onSelect = vi.fn();

    render(
      <RecoverySummaryCard
        summary={{
          title: "Use fallback review",
          severity: "medium",
          urgency: "now",
          cta_kind: "review",
          primary_descriptor: {
            recovery_hint: "review_fallback",
            recovery_target: {
              dispatch_kind: "session",
            },
          },
        }}
        onSelect={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Review fallback" }));

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({
        recovery_hint: "review_fallback",
      }),
    );
  });
});
