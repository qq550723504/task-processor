import { afterEach, describe, expect, it, vi } from "vitest";

import { retryChildTask } from "@/lib/api/child-task-retry";

describe("retryChildTask", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts the child task kind to the retry endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ task_id: "task-1" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await retryChildTask("task-1", { kind: "sds_design_sync" });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/tasks/task-1/child-tasks/retry");
    const request = fetchMock.mock.calls[0]?.[1];
    expect(request?.method).toBe("POST");
    expect(JSON.parse(String(request?.body))).toEqual({
      kind: "sds_design_sync",
    });
  });
});
