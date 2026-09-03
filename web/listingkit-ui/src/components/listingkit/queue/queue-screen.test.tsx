import { render } from "@testing-library/react";

import { QueueScreen } from "@/components/listingkit/queue/queue-screen";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    replace: mocks.replace,
    push: mocks.push,
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

describe("QueueScreen", () => {
  beforeEach(() => {
    mocks.replace.mockReset();
    mocks.push.mockReset();
  });

  it("redirects the retired queue entrypoint to the workspace", () => {
    render(<QueueScreen taskId="task-queue-1" />);

    expect(mocks.replace).toHaveBeenCalledWith(
      "/listing-kits/task-queue-1/workspace",
    );
  });
});
