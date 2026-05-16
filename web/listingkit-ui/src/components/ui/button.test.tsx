import { render, screen } from "@testing-library/react";

import { Button } from "@/components/ui/button";

describe("Button", () => {
  it("supports shadcn variant and size props", () => {
    render(
      <Button size="sm" variant="secondary">
        Refresh
      </Button>,
    );

    const button = screen.getByRole("button", { name: "Refresh" });
    expect(button).toHaveClass("h-8");
    expect(button).toHaveClass("bg-secondary");
  });

  it("renders as a child element when asChild is set", () => {
    render(
      <Button asChild variant="link">
        <a href="/listing-kits/prompts">Prompts</a>
      </Button>,
    );

    expect(screen.getByRole("link", { name: "Prompts" })).toHaveAttribute(
      "href",
      "/listing-kits/prompts",
    );
  });
});
