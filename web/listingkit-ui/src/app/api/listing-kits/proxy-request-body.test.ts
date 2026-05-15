import { describe, expect, it, vi } from "vitest";

import { readProxyRequestBody } from "@/app/api/listing-kits/proxy-request-body";

describe("readProxyRequestBody", () => {
  it("returns undefined for empty request bodies", async () => {
    await expect(
      readProxyRequestBody(new Request("http://localhost", { method: "POST" }), 100),
    ).resolves.toBeUndefined();
  });

  it("returns an ArrayBuffer for non-empty request bodies", async () => {
    const body = await readProxyRequestBody(
      new Request("http://localhost", {
        method: "POST",
        body: "hello",
      }),
      100,
    );

    expect(new TextDecoder().decode(body)).toBe("hello");
  });

  it("rejects when the body read exceeds the timeout", async () => {
    vi.useFakeTimers();
    const request = {
      arrayBuffer: () => new Promise<ArrayBuffer>(() => undefined),
    } as Request;

    const promise = readProxyRequestBody(request, 100);
    const assertion = expect(promise).rejects.toThrow(
      "ListingKit proxy request body read timed out",
    );
    await vi.advanceTimersByTimeAsync(101);

    await assertion;
    vi.useRealTimers();
  });
});
