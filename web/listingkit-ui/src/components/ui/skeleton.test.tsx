import { render, screen } from "@testing-library/react";

import { Skeleton } from "@/components/ui/skeleton";

describe("Skeleton", () => {
  it("renders a loading placeholder with caller classes", () => {
    render(<Skeleton aria-label="loading card" className="h-12 rounded-lg" />);

    const skeleton = screen.getByLabelText("loading card");
    expect(skeleton).toHaveClass("animate-pulse");
    expect(skeleton).toHaveClass("h-12");
    expect(skeleton).toHaveClass("rounded-lg");
  });
});
