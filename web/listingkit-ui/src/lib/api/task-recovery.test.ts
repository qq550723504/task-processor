import { afterEach, describe, expect, it, vi } from "vitest";

import { bulkRecoverTasks, recoverTaskNow } from "@/lib/api/task-recovery";

describe("task recovery api", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts to recover a blocked task immediately", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          task: {
            task_id: "task-1",
            status: "pending",
          },
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await recoverTaskNow("task-1");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/tasks/task-1/recover");
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });

  it("posts a bulk recovery request with query/body fields", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ recovered_count: 4 }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await bulkRecoverTasks({
      due_before: "2026-06-06T04:00:00Z",
      recover_at: "2026-06-06T04:05:00Z",
      limit: 7,
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toContain(
      "/tasks/recovery/recover?due_before=2026-06-06T04%3A00%3A00Z&limit=7",
    );
    const request = fetchMock.mock.calls[0]?.[1];
    expect(request?.method).toBe("POST");
    expect(JSON.parse(String(request?.body))).toEqual({
      recover_at: "2026-06-06T04:05:00Z",
    });
  });
});
