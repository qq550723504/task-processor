import { render, screen } from "@testing-library/react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

describe("Alert", () => {
  it("renders accessible status content with title and description", () => {
    render(
      <Alert>
        <AlertTitle>保存失败</AlertTitle>
        <AlertDescription>请稍后重试。</AlertDescription>
      </Alert>,
    );

    expect(screen.getByRole("status")).toHaveTextContent("保存失败");
    expect(screen.getByRole("status")).toHaveTextContent("请稍后重试。");
  });

  it("supports destructive alerts", () => {
    render(
      <Alert variant="destructive">
        <AlertTitle>无法保存</AlertTitle>
      </Alert>,
    );

    expect(screen.getByRole("status")).toHaveClass("border-destructive/50");
  });

  it("supports success alerts", () => {
    render(
      <Alert variant="success">
        <AlertTitle>保存成功</AlertTitle>
      </Alert>,
    );

    expect(screen.getByRole("status")).toHaveClass("border-success/50");
  });
});
