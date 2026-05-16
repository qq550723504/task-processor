import { render, screen } from "@testing-library/react";

import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

describe("Card", () => {
  it("composes header, content, and footer sections", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>订阅</CardTitle>
          <CardDescription>模块开通能力</CardDescription>
        </CardHeader>
        <CardContent>套餐内容</CardContent>
        <CardFooter>保存</CardFooter>
      </Card>,
    );

    expect(screen.getByText("订阅")).toHaveClass("font-semibold");
    expect(screen.getByText("模块开通能力")).toHaveClass("text-muted-foreground");
    expect(screen.getByText("套餐内容")).toHaveClass("pt-0");
    expect(screen.getByText("保存")).toHaveClass("flex");
  });
});
