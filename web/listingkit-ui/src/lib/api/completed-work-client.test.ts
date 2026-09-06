// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { CompletedWorkError, fetchCompletedWork } from "./completed-work-client";
import { SheinRecordListError } from "./shein-records-client";
import { projectCompletedWork } from "./completed-work";
const page = projectCompletedWork({ items: [], next_cursor: null });
afterEach(() => vi.unstubAllGlobals());
it("uses the fixed source and existing error class", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(page)));
  expect(CompletedWorkError).toBe(SheinRecordListError);
  expect(await fetchCompletedWork({ organizationId: "200", cursor: "opaque" })).toEqual(page);
  expect(fetch).toHaveBeenCalledWith("/api/workbench/completed-work?source=listing-local-preparation&limit=20&cursor=opaque", expect.objectContaining({ cache: "no-store", redirect: "error", credentials: "same-origin", headers: { Accept: "application/json", "X-Expected-Organization-ID": "200" } }));
});
it.each([[403, "permission_denied"], [504, "deadline_exceeded"]])("preserves existing typed %s errors", async (status, error) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ error }, { status: Number(status) })));
  await expect(fetchCompletedWork({ organizationId: "200" })).rejects.toMatchObject({ status, code: error });
});
it("preserves protocol deadline and rejects forged coverage and duplicate fields", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "DEADLINE_EXCEEDED", message: "timeout", requestId: "id", fieldErrors: [] }, { status: 504 })));
  await expect(fetchCompletedWork({ organizationId: "200" })).rejects.toMatchObject({ status: 504, code: "DEADLINE_EXCEEDED" });
  for (const payload of ['{"items":[],"items":[],"next_cursor":null}', JSON.stringify({ ...page, coverage: "all-tasks" }), "x".repeat(128 * 1024 + 1)]) {
    vi.mocked(fetch).mockResolvedValue(new Response(payload, { headers: { "Content-Type": "application/json" } }));
    await expect(fetchCompletedWork({ organizationId: "200" })).rejects.toMatchObject({ status: 502, code: "INVALID_UPSTREAM_RESPONSE" });
  }
});
it("does not fetch invalid or cancelled input and preserves cancellation reasons", async () => {
  vi.stubGlobal("fetch", vi.fn());
  await expect(fetchCompletedWork({ organizationId: "200", limit: 0 })).rejects.toBeInstanceOf(CompletedWorkError);
  const controller = new AbortController(); const reason = new DOMException("cancelled", "AbortError"); controller.abort(reason);
  await expect(fetchCompletedWork({ organizationId: "200", signal: controller.signal })).rejects.toBe(reason);
  expect(fetch).not.toHaveBeenCalled();
});
it("cancels response reading without turning it into an empty page", async () => {
  const controller = new AbortController();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(new ReadableStream({ start() {} }), { headers: { "Content-Type": "application/json" } })));
  const pending = fetchCompletedWork({ organizationId: "200", signal: controller.signal });
  await Promise.resolve(); controller.abort();
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
});
