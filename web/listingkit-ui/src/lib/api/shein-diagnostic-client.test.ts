// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { fetchSheinDiagnostic, SheinDiagnosticError } from "./shein-diagnostic-client";

afterEach(() => vi.unstubAllGlobals());
const input = { recordId: "12345678-1234-4234-8234-123456789abc", action: "publish" as const, organizationId: "200" };
it("uses only same-origin GET and expected org assertion and preserves typed failure", async () => {
  const fetcher = vi.fn().mockResolvedValue(Response.json({ error: "stale_input" }, { status: 409 })); vi.stubGlobal("fetch", fetcher);
  const controller = new AbortController();
  await expect(fetchSheinDiagnostic({ ...input, signal: controller.signal })).rejects.toMatchObject({ status: 409, code: "stale_input", payload: { error: "stale_input" } });
  expect(fetcher.mock.calls[0]).toEqual([`/api/listing/shein-records/${input.recordId}/offline-diagnostic?action=publish`, { method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error", headers: { Accept: "application/json", "X-Expected-Organization-ID": "200" }, signal: controller.signal }]);
});
it("never calls fetch with invalid path/organization/digest", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  for (const patch of [{ recordId: "../secret" }, { organizationId: "200,100" }, { expectedDigest: "" }]) {
    await expect(fetchSheinDiagnostic({ ...input, ...patch })).rejects.toBeInstanceOf(SheinDiagnosticError);
  }
  expect(fetcher).not.toHaveBeenCalled();
});
it("keeps abort distinct from server diagnostics and sanitizes invalid responses", async () => {
  const abort = new DOMException("Aborted", "AbortError");
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abort));
  await expect(fetchSheinDiagnostic(input)).rejects.toBe(abort);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("private SQL", { status: 500 })));
  await expect(fetchSheinDiagnostic(input)).rejects.toMatchObject({ status: 502, code: "INVALID_UPSTREAM_RESPONSE" });
});
