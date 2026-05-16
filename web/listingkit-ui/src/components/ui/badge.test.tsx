import { render, screen } from "@testing-library/react";

import { Badge } from "@/components/ui/badge";

describe("Badge", () => {
  it("renders success and warning variants", () => {
    render(
      <>
        <Badge variant="success">已通过</Badge>
        <Badge variant="warning">待处理</Badge>
      </>,
    );

    expect(screen.getByText("已通过")).toHaveClass("bg-success");
    expect(screen.getByText("待处理")).toHaveClass("bg-warning");
  });

  it("merges caller class names", () => {
    render(<Badge className="rounded-full">状态</Badge>);

    expect(screen.getByText("状态")).toHaveClass("rounded-full");
  });
});
