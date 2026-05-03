import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TaskSourceSummary } from "@/components/listingkit/tasks/task-source-summary";

describe("TaskSourceSummary", () => {
  it("renders a product URL source summary", () => {
    render(
      <TaskSourceSummary
        draft={{
          text: "",
          imageUrls: "",
          productUrl: "https://detail.1688.com/offer/123456789.html",
          platforms: ["shein"],
        }}
      />,
    );

    expect(screen.getByText("任务来源")).toBeInTheDocument();
    expect(screen.getByText("来自商品链接")).toBeInTheDocument();
    expect(
      screen.getByText(
        "这个任务从商品页链接开始创建，适合已经有明确原始商品来源的场景。",
      ),
    ).toBeInTheDocument();
  });

  it("renders an image URL source summary", () => {
    render(
      <TaskSourceSummary
        draft={{
          text: "",
          imageUrls: "https://example.com/1.jpg\nhttps://example.com/2.jpg",
          productUrl: "",
          platforms: ["shein"],
        }}
      />,
    );

    expect(screen.getByText("来自图片素材")).toBeInTheDocument();
    expect(
      screen.getByText("这个任务提交了 2 张图片，系统会根据图片内容继续生成和审核。"),
    ).toBeInTheDocument();
  });
});

