import { afterEach, describe, expect, it, vi } from "vitest";

import { POST } from "./route";

describe("demo request webhook route", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("forwards a valid request to the configured webhook", async () => {
    vi.stubEnv("LISTINGKIT_DEMO_WEBHOOK_URL", "https://webhook.example.test/send");
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ errcode: 0 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(new Request("http://localhost/api/demo-requests", {
      method: "POST",
      body: JSON.stringify({
        name: "李明",
        company: "示例店铺",
        contact: "hello@example.com",
        platforms: ["SHEIN", "TEMU"],
        message: "希望了解多平台资料同步。",
      }),
    }));

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: "POST" });
  });

  it("rejects incomplete requests before calling the webhook", async () => {
    vi.stubEnv("LISTINGKIT_DEMO_WEBHOOK_URL", "https://webhook.example.test/send");
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(new Request("http://localhost/api/demo-requests", {
      method: "POST",
      body: JSON.stringify({ name: "李明", company: "示例店铺", contact: "" }),
    }));

    expect(response.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
