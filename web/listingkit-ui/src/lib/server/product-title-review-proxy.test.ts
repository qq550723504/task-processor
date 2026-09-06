// @vitest-environment node
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyProductTitleReview } from "./product-title-review-proxy";
import { titleProposalFixture } from "@/test/product-title-review-fixture";
import { applyProductTitleProposal, decideProductTitleProposal } from "@/lib/api/product-title-review-client";

const id = titleProposalFixture().proposal_id;
const base = "/api/product/text-proposals";
const list = { schema_version: 1, coverage: "product-title-proposals-only", items: [], next_cursor: null };
function request(path = `${base}/${id}/decisions`, body = '{"action":"accept","expected_revision":"9007199254740993"}', headers: HeadersInit = {}, method = "POST") {
  return new Request(`https://console.test${path}`, { method, ...(method === "POST" ? { body } : {}), headers: {
    Cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", Origin: "https://console.test", "Sec-Fetch-Site": "same-origin",
    "Content-Type": "application/json", "Idempotency-Key": "explicit-key", ...headers,
  } });
}
beforeEach(() => { vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", "http://127.0.0.1:9876"); vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://console.test"); });
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); });
const call = (r: Request, token = "server-token") => proxyProductTitleReview(r, token, { forwarded: false });

it.each(["decisions", "apply"])("client retains actual %s BFF capacity failure as not_sent", async (action) => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  const response = await call(request(`${base}/${id}/${action}`, " ".repeat(32769)));
  expect(response.status).toBe(413); expect(fetcher).not.toHaveBeenCalled();
  fetcher.mockResolvedValueOnce(response);
  const scope = { organizationId: "B", proposalId: id, idempotencyKey: "intent" };
  const pending = action === "apply" ? applyProductTitleProposal({ ...scope, input: { expected_revision: "2" } }) : decideProductTitleProposal({ ...scope, input: { action: "accept", expected_revision: "1" } });
  await expect(pending).rejects.toMatchObject({ status: 413, outcome: "not_sent" });
  expect(fetcher).toHaveBeenCalledOnce();
});

it("forwards exact numeric revision token and only trusted headers once", async () => {
  const fetcher = vi.fn().mockResolvedValue(Response.json(titleProposalFixture())); vi.stubGlobal("fetch", fetcher);
  expect((await call(request(undefined, undefined, { Authorization: "Bearer forged", "X-Requested-Organization-ID": "A", "X-User-Roles": "admin" }))).status).toBe(200);
  expect(fetcher).toHaveBeenCalledOnce();
  const [url, options] = fetcher.mock.calls[0];
  expect(url.toString()).toBe(`http://127.0.0.1:9876${base}/${id}/decisions`);
  expect(options.body).toBe('{"action":"accept","expected_revision":9007199254740993}');
  expect(Object.fromEntries(new Headers(options.headers))).toEqual({ accept: "application/json", authorization: "Bearer server-token", "x-requested-organization-id": "B", "content-type": "application/json", "idempotency-key": "explicit-key" });
  expect(options).toMatchObject({ cache: "no-store", redirect: "manual", method: "POST" });
});
it.each([
  '{"action":"accept","expected_revision":"1","expected_revision":"2"}',
  '{"action":"accept","expected_revision":1}',
  '{"action":"accept","expected_revision":"01"}',
  '{"action":"accept","expected_revision":"9223372036854775808"}',
  '{"action":"accept","expected_revision":"1","actor":"admin"}',
  '{"action":"edit","expected_revision":"1","title":"\\uD800"}',
  '{"action":"edit","expected_revision":"1","title":" title "}',
  '{"action":"edit","expected_revision":"1","title":"' + "中".repeat(1366) + '"}',
])("rejects invalid write body before forwarding: %s", async (body) => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await call(request(undefined, body))).status).toBe(400); expect(fetcher).not.toHaveBeenCalled();
});
it.each([
  [`${base}`, "POST"], [`${base}/${id}/generate`, "POST"], [`${base}/${id}/apply?extra=x`, "POST"],
  [`${base}/${id}/decisions`, "GET"], [`${base}/${id}?x=1`, "GET"],
  [`${base}?view=actionable&limit=101`, "GET"], [`${base}?view=actionable&view=actionable`, "GET"],
  [`${base}?view=actionable&limit=020`, "GET"], [`${base}?view=actionable&unknown=1`, "GET"], [`${base}`, "GET"],
  [`${base}?view=actionable&cursor=${"x".repeat(257)}`, "GET"],
])("rejects unapproved path/query/method %s %s", async (path, method) => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await call(request(path, undefined, {}, method))).status).toBeGreaterThanOrEqual(400); expect(fetcher).not.toHaveBeenCalled();
});
it("blocks identity/org/CSRF/content type/key before sending and bounds actual bytes", async () => {
  const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher);
  expect((await call(request(), "")).status).toBe(401);
  for (const headers of [
    { "X-Expected-Organization-ID": "A" }, { Origin: "https://evil.test", Host: "evil.test" },
    { Origin: "" }, { "Sec-Fetch-Site": "cross-site" }, { "Content-Type": "text/plain" }, { "Idempotency-Key": "" },
    { "Idempotency-Key": "first, second" },
  ] as HeadersInit[]) expect((await call(request(undefined, undefined, headers))).status).toBeGreaterThanOrEqual(400);
  expect((await call(request(undefined, " ".repeat(32769), { "Content-Length": "1" }))).status).toBe(413);
  expect(fetcher).not.toHaveBeenCalled();
});
it("serves collection, detail and standalone apply using the same contract", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json(list)).mockResolvedValueOnce(Response.json(titleProposalFixture())).mockResolvedValueOnce(Response.json(titleProposalFixture())); vi.stubGlobal("fetch", fetcher);
  expect(await (await call(request(`${base}?view=actionable`, undefined, {}, "GET"))).json()).toEqual(list);
  expect((await call(request(`${base}/${id}`, undefined, {}, "GET"))).status).toBe(200);
  expect((await call(request(`${base}/${id}/apply`, '{"expected_revision":"2"}'))).status).toBe(200);
  expect(fetcher).toHaveBeenCalledTimes(3); expect(fetcher.mock.calls[2][1].body).toBe('{"expected_revision":2}');
});
it.each(["throw", "redirect", "oversize", "invalid", "503", "504", "mismatched-id"])("marks dispatched %s as unverified, never repeats POST", async (mode) => {
  const fetcher = vi.fn().mockImplementation(async () => {
    if (mode === "throw") throw new Error("private SQL/token");
    if (mode === "redirect") return new Response(null, { status: 307, headers: { Location: "https://evil.test" } });
    if (mode === "oversize") return new Response(" ".repeat(131073), { headers: { "Content-Type": "application/json" } });
    if (mode === "503" || mode === "504") return Response.json({ error: mode === "503" ? "unavailable" : "deadline_exceeded" }, { status: Number(mode) });
    return Response.json(mode === "invalid" ? { token: "secret" } : { ...titleProposalFixture(), proposal_id: "12345678-1234-4234-8234-123456789abd" });
  }); vi.stubGlobal("fetch", fetcher);
  expect(await (await call(request())).json()).toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
  expect(fetcher).toHaveBeenCalledOnce();
});
it("preserves known rejection and safe auth request ID/cookie revocation", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ error: "operation_conflict" }, { status: 409 }))
    .mockResolvedValueOnce(Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "safe", requestId: "req-1", fieldErrors: [] }, { status: 403 })); vi.stubGlobal("fetch", fetcher);
  expect(await (await call(request())).json()).toEqual({ error: "operation_conflict" });
  const revoked = await call(request()); expect(await revoked.json()).toMatchObject({ code: "ORGANIZATION_ACCESS_REVOKED", requestId: "req-1" });
  expect(revoked.headers.get("set-cookie")).toContain("shuomi_effective_organization=;");
});
