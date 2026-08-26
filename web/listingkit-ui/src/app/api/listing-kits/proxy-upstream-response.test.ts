import { describe, expect, it, vi } from "vitest";

import { buildListingKitProxyResponse } from "@/app/api/listing-kits/proxy-upstream-response";
import * as requestLog from "@/lib/server/request-log";

describe("buildListingKitProxyResponse", () => {
  it("preserves 304 responses without reading a body", async () => {
    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/tasks/demo",
      requestId: "req-1",
      routePath: ["tasks", "demo"],
      upstream: new Response(null, {
        status: 304,
        headers: { etag: "delta-1" },
      }),
    });

    expect(response.status).toBe(304);
    expect(response.headers.get("ETag")).toBe("delta-1");
    expect(await response.text()).toBe("");
  });

  it("proxies text responses with upstream status and content type", async () => {
    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/tasks/demo",
      requestId: "req-1",
      routePath: ["tasks", "demo"],
      upstream: new Response(JSON.stringify({ ok: true }), {
        status: 202,
        headers: { "content-type": "application/json" },
      }),
    });

    expect(response.status).toBe(202);
    expect(response.headers.get("Content-Type")).toBe("application/json");
    await expect(response.json()).resolves.toEqual({ ok: true });
  });

  it("streams only image-agent SSE responses without buffering the body", async () => {
    const upstream = new Response("id: 1\ndata: {}\n\n", {
      status: 200,
      headers: { "content-type": "text/event-stream; charset=utf-8" },
    });
    const textSpy = vi
      .spyOn(upstream, "text")
      .mockRejectedValue(new Error("SSE must not be buffered"));

    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/image-agent/runs/run-1/events",
      requestId: "req-sse",
      routePath: ["image-agent", "runs", "run-1", "events"],
      upstream,
    });

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toContain("text/event-stream");
    expect(textSpy).not.toHaveBeenCalled();
    await expect(response.text()).resolves.toContain("id: 1");
  });

  it("proxies binary uploaded files as array buffers", async () => {
    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/uploads/files/demo.png",
      requestId: "req-1",
      routePath: ["uploads", "files", "demo.png"],
      upstream: new Response(new Uint8Array([1, 2, 3]), {
        status: 200,
        headers: { "content-type": "application/octet-stream" },
      }),
    });

    expect(response.status).toBe(200);
    expect(Array.from(new Uint8Array(await response.arrayBuffer()))).toEqual([
      1, 2, 3,
    ]);
  });

  it("returns a 502 JSON response when upstream body reading fails", async () => {
    const upstream = new Response("unavailable", { status: 200 });
    vi.spyOn(upstream, "text").mockRejectedValueOnce(new Error("stream failed"));

    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/tasks/demo",
      requestId: "req-1",
      routePath: ["tasks", "demo"],
      upstream,
    });

    expect(response.status).toBe(502);
    await expect(response.json()).resolves.toMatchObject({
      error: "listingkit_upstream_body_unavailable",
      message: "stream failed",
    });
  });

  it("logs an upstream error body preview for non-ok text responses", async () => {
    const warnSpy = vi.spyOn(requestLog, "logRequestWarn");

    const response = await buildListingKitProxyResponse({
      durationMs: 12,
      method: "GET",
      path: "/studio/batches",
      requestId: "req-500",
      routePath: ["studio", "batches"],
      upstream: new Response(
        JSON.stringify({ error: "boom", message: "database row scan failed" }),
        {
          status: 500,
          headers: { "content-type": "application/json" },
        },
      ),
    });

    expect(response.status).toBe(500);
    expect(warnSpy).toHaveBeenCalledWith(
      "listingkit proxy response",
      expect.objectContaining({
        requestId: "req-500",
        status: 500,
        upstreamBodyPreview: expect.stringContaining("database row scan failed"),
      }),
    );
  });
});
