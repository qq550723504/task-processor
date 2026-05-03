import { render, screen } from "@testing-library/react";

import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";

describe("TaskProgressNotice", () => {
  it("renders nothing for failed tasks", () => {
    const { container } = render(<TaskProgressNotice task={{ status: "failed" }} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("renders processing guidance", () => {
    render(<TaskProgressNotice task={{ status: "processing" }} />);

    expect(screen.getByText("正在生成图片")).toBeInTheDocument();
    expect(
      screen.getByText(
        "系统正在补齐预览、审核和提交所需结果。你可以留在这里等待，也可以稍后回到任务列表继续。",
      ),
    ).toBeInTheDocument();
  });

  it("renders pending guidance", () => {
    render(<TaskProgressNotice task={{ status: "pending" }} />);

    expect(screen.getByText("正在等待开始")).toBeInTheDocument();
    expect(
      screen.getByText(
        "任务已经创建成功，系统正在排队准备处理。现在可以离开页面，稍后从任务列表继续。",
      ),
    ).toBeInTheDocument();
  });
});
