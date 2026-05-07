import { afterEach, describe, expect, it, vi } from "vitest";

import { submitTask } from "@/lib/api/submit";

describe("submitTask", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("adds an idempotency key when the caller does not provide one", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ task_id: "task-1" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("crypto", { randomUUID: () => "submit-uuid-1" });

    await submitTask("task-1", { platform: "shein", action: "publish" });

    const request = fetchMock.mock.calls[0]?.[1];
    const body = JSON.parse(String(request?.body));
    expect(body).toMatchObject({
      platform: "shein",
      action: "publish",
      idempotency_key: "submit-uuid-1",
    });
  });
});
