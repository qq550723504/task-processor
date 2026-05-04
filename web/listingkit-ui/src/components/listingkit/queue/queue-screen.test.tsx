import { render, screen } from "@testing-library/react";

import { QueueScreen } from "@/components/listingkit/queue/queue-screen";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  useGenerationQueue: vi.fn(),
  useListingKitTaskResult: vi.fn(),
  useDispatchNavigation: vi.fn(() => ({ mutate: vi.fn() })),
  useExecuteAction: vi.fn(() => ({ mutate: vi.fn() })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    replace: mocks.replace,
    push: mocks.push,
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

vi.mock("@/lib/query/use-queue", () => ({
  useGenerationQueue: (...args: unknown[]) => mocks.useGenerationQueue(...args),
}));

vi.mock("@/lib/query/use-task-result", () => ({
  useListingKitTaskResult: (...args: unknown[]) => mocks.useListingKitTaskResult(...args),
}));

vi.mock("@/lib/query/use-dispatch", () => ({
  useDispatchNavigation: () => mocks.useDispatchNavigation(),
}));

vi.mock("@/lib/query/use-action", () => ({
  useExecuteAction: () => mocks.useExecuteAction(),
}));

describe("QueueScreen", () => {
  beforeEach(() => {
    mocks.replace.mockReset();
    mocks.push.mockReset();
    mocks.useGenerationQueue.mockReset();
    mocks.useListingKitTaskResult.mockReset();
  });

  it("shows a page-level recovery state when the queue request fails", () => {
    mocks.useGenerationQueue.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    });
    mocks.useListingKitTaskResult.mockReturnValue({
      data: { status: "processing" },
      isError: false,
    });

    render(<QueueScreen taskId="task-queue-1" />);

    expect(screen.getByText("队列暂时无法加载")).toBeInTheDocument();
    expect(
      screen.getByText("当前无法读取生成队列或任务状态。你可以返回工作台继续查看，或回到任务列表稍后重试。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "返回任务列表" })).toHaveAttribute(
      "href",
      "/listing-kits",
    );
    expect(screen.getByRole("link", { name: "打开工作台" })).toHaveAttribute(
      "href",
      "/listing-kits/task-queue-1/workspace?platform=shein&section_key=general_review",
    );
  });

  it("shows a waiting message when queue data is not ready yet", () => {
    mocks.useGenerationQueue.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    });
    mocks.useListingKitTaskResult.mockReturnValue({
      data: { status: "processing" },
      isError: false,
    });

    render(<QueueScreen taskId="task-queue-2" />);

    expect(screen.getByText("队列数据暂未准备完成")).toBeInTheDocument();
    expect(
      screen.getByText("当前任务还没有返回完整的生成队列。你可以先打开工作台查看处理进度，或稍后回到这里继续。"),
    ).toBeInTheDocument();
  });
});
