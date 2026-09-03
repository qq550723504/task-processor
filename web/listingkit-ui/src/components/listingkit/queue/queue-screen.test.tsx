import { render } from "@testing-library/react";

import { QueueScreen } from "@/components/listingkit/queue/queue-screen";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  useGenerationQueue: vi.fn(),
  useListingKitTaskResult: vi.fn(),
  useDispatchNavigation: vi.fn(() => ({ mutate: vi.fn() })),
  useExecuteAction: vi.fn(() => ({ mutate: vi.fn() })),
  useBulkRecoverTasks: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
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

vi.mock("@/lib/query/use-task-recovery", () => ({
  useBulkRecoverTasks: () => mocks.useBulkRecoverTasks(),
}));

describe("QueueScreen", () => {
  beforeEach(() => {
    mocks.replace.mockReset();
    mocks.push.mockReset();
    mocks.useGenerationQueue.mockReset();
    mocks.useListingKitTaskResult.mockReset();
  });

  it("redirects the retired queue entrypoint to the workspace", () => {
    render(<QueueScreen taskId="task-queue-1" />);

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-queue-1/workspace",
    );
  });
});
