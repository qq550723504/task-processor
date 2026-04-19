import { render, screen } from "@testing-library/react";

import { ResolvedActionCard } from "@/components/listingkit/resolved-action-card";

describe("ResolvedActionCard", () => {
  it("renders the primary summary and CTA", () => {
    render(
      <ResolvedActionCard
        summary={{
          title: "Review detail previews",
          summary: "3 preview-ready detail sections need review.",
          cta_kind: "review",
          action_key: "review_detail_previews",
        }}
        onSelect={() => undefined}
      />,
    );

    expect(screen.getByText("Review detail previews")).toBeInTheDocument();
    expect(
      screen.getByText("3 preview-ready detail sections need review."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review" })).toBeInTheDocument();
  });
});
