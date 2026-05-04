import { render, screen } from "@testing-library/react";

import { ResolvedActionCard } from "@/components/listingkit/review/resolved-action-card";

describe("ResolvedActionCard", () => {
  it("renders localized summary and CTA", () => {
    render(
      <ResolvedActionCard
        summary={{
          title: "Review Previews",
          summary: "Review the current section and preview focus.",
          cta_kind: "review",
          action_key: "review_detail_previews",
        }}
        onSelect={() => undefined}
      />,
    );

    expect(screen.getByText("当前建议")).toBeInTheDocument();
    expect(screen.getByText("检查预览结果")).toBeInTheDocument();
    expect(
      screen.getByText("先检查当前分区和预览焦点，再继续处理。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "去检查" })).toBeInTheDocument();
  });
});
