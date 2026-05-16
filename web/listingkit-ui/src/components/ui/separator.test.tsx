import { render, screen } from "@testing-library/react";

import { Separator } from "@/components/ui/separator";

describe("Separator", () => {
  it("renders a horizontal separator by default", () => {
    render(<Separator data-testid="separator" />);

    const separator = screen.getByTestId("separator");
    expect(separator).toHaveAttribute("role", "separator");
    expect(separator).toHaveAttribute("aria-orientation", "horizontal");
    expect(separator).toHaveClass("h-px");
  });

  it("supports vertical orientation", () => {
    render(<Separator data-testid="separator" orientation="vertical" />);

    const separator = screen.getByTestId("separator");
    expect(separator).toHaveAttribute("aria-orientation", "vertical");
    expect(separator).toHaveClass("h-full");
    expect(separator).toHaveClass("w-px");
  });
});
