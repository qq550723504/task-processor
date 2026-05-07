import { render, screen } from "@testing-library/react";

import { SheinSubmissionTimeline } from "@/components/listingkit/shein/shein-submission-timeline";

describe("SheinSubmissionTimeline", () => {
  it("shows only customer-facing submission events in the main timeline", () => {
    render(
      <SheinSubmissionTimeline
        events={[
          {
            id: "event-1",
            action: "image_upload",
            status: "success",
            started_at: "2026-04-27T10:00:00Z",
          },
          {
            id: "event-2",
            action: "save_draft",
            status: "success",
            started_at: "2026-04-27T10:01:00Z",
            response: { message: "draft saved" },
          },
          {
            id: "event-3",
            action: "publish",
            status: "failed",
            started_at: "2026-04-27T10:02:00Z",
            error_message: "raw publish error",
            validation_notes: ["方形图必须有一个"],
          },
          {
            id: "event-4",
            action: "precheck",
            status: "success",
            started_at: "2026-04-27T10:03:00Z",
          },
        ]}
      />,
    );

    expect(screen.getByText("提交记录")).toBeInTheDocument();
    expect(screen.getByText("上传图片")).toBeInTheDocument();
    expect(screen.getByText("保存草稿")).toBeInTheDocument();
    expect(screen.getByText("正式发布")).toBeInTheDocument();
    expect(screen.getByText("高级日志（1）")).toBeInTheDocument();
    expect(screen.getByText("查看失败详情")).toBeInTheDocument();
    expect(screen.getByText("查看 SHEIN 校验提示")).toBeInTheDocument();
  });

  it("promotes submit phase events with request and remote details", () => {
    render(
      <SheinSubmissionTimeline
        events={[
          {
            id: "phase-1",
            action: "submit_phase",
            phase: "confirm_remote",
            status: "confirmed",
            request_id: "submit-123",
            remote_record_id: "record-9",
            detail: "SHEIN remote record confirmed",
            started_at: "2026-04-27T10:00:00Z",
          },
          {
            id: "event-2",
            action: "precheck",
            status: "success",
            started_at: "2026-04-27T10:01:00Z",
          },
        ]}
      />,
    );

    expect(screen.getByText("确认远端记录")).toBeInTheDocument();
    expect(screen.getByText("Request submit-123")).toBeInTheDocument();
    expect(screen.getByText("Record record-9")).toBeInTheDocument();
    expect(screen.getByText("SHEIN remote record confirmed")).toBeInTheDocument();
    expect(screen.getByText("高级日志（1）")).toBeInTheDocument();
  });

  it("renders nothing without events", () => {
    const { container } = render(<SheinSubmissionTimeline events={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
