import { afterEach, describe, expect, it, vi } from "vitest";
import { capture1688, readBrowserCaptureByKey, verifyBrowserCapture } from "./browser-capture";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
const key = "22222222-2222-4222-8222-22222222222b";
const op = "11111111-1111-4111-8111-11111111111a";
const intent = { userId: "actor", organizationId: "org-B", key, body: JSON.stringify(browserCaptureFixture()) };
const receipt = { schemaVersion: 1, operationId: op, outcome: "published", replayed: true, productKey: "crawler:1688:981645030344", publicationId: `source-run:acquisition:${op}`, catalogVersion: "1", warnings: [], missingFacts: [] };
afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });
describe("Browser transport", () => {
  it("sends the original body/key and assertions exactly once", async () => {
    const fetcher = vi.fn().mockResolvedValue(Response.json(receipt)); vi.stubGlobal("fetch", fetcher);
    expect(await capture1688(intent)).toEqual(receipt);
    const [url, options] = fetcher.mock.calls[0]!;
    expect(url).toBe("/api/workbench/sourcing/1688/browser-captures");
    expect(options.body).toBe(intent.body);
    const h = new Headers(options.headers);
    expect(h.get("Idempotency-Key")).toBe(key); expect(h.get("X-Expected-User-ID")).toBe("actor"); expect(h.get("X-Expected-Organization-ID")).toBe("org-B");
    expect(h.has("Authorization")).toBe(false); expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("recovers by original key without a body or POST", async () => {
    const fetcher = vi.fn().mockResolvedValue(Response.json(receipt)); vi.stubGlobal("fetch", fetcher);
    await readBrowserCaptureByKey(intent, key);
    const [url, options] = fetcher.mock.calls[0]!;
    expect(url).toContain(`/by-key/${key}`); expect(options.method).toBe("GET"); expect(options.body).toBeUndefined();
  });
  it("verify reuses frozen intent and never falls back to create", async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error("network")); vi.stubGlobal("fetch", fetcher);
    await expect(verifyBrowserCapture(intent)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" });
    expect(fetcher.mock.calls[0]![0]).toMatch(/\/verify$/); expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it.each(["network", "bad receipt", "foreign product", "oversize"])("keeps %s as unknown after dispatch without retries", async (mode) => {
    const fetcher = mode === "network" ? vi.fn().mockRejectedValue(new Error("network")) : vi.fn().mockResolvedValue(mode === "bad receipt" ? Response.json({ ok: true }) : mode === "oversize" ? new Response("x".repeat(2 * 1024 * 1024 + 1)) : Response.json({ ...receipt, productKey: "crawler:1688:42" }));
    vi.stubGlobal("fetch", fetcher);
    await expect(capture1688(intent)).rejects.toMatchObject({ code: "OUTCOME_UNKNOWN" }); expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("pre-cancel does not dispatch", async () => {
    const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher); const controller = new AbortController(); controller.abort();
    await expect(capture1688(intent, controller.signal)).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" }); expect(fetcher).not.toHaveBeenCalled();
  });
});
