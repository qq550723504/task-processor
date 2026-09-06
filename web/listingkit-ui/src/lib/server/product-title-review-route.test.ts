// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, expect, it, vi } from "vitest";
import { titleProposalFixture } from "@/test/product-title-review-fixture";
const auth = vi.hoisted(() => ({ token: "server-token", wait: () => Promise.resolve() }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (r: NextRequest & { auth: unknown }) => Promise<Response>) => async (r: NextRequest) => { await auth.wait(); return handler(Object.assign(r, { auth: { token: auth.token } })); } }));
vi.mock("@/lib/server/zitadel-server-token", () => ({ readZitadelServerAccessToken: (v: { token: string }) => v.token }));
import { GET as listGET, POST as listPOST } from "@/app/api/product/text-proposals/route";
import { GET as detailGET } from "@/app/api/product/text-proposals/[proposal_id]/route";
import { POST as decisionPOST } from "@/app/api/product/text-proposals/[proposal_id]/decisions/route";
import { POST as applyPOST } from "@/app/api/product/text-proposals/[proposal_id]/apply/route";

const base = "/api/product/text-proposals"; const id = titleProposalFixture().proposal_id;
function req(path: string, body?: string) { return new NextRequest(`https://console.test${path}`, { method: body ? "POST" : "GET", body, headers: { Cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", Origin: "https://console.test", "Content-Type": "application/json", "Idempotency-Key": "intent" } }); }
afterEach(() => { auth.token = "server-token"; auth.wait = () => Promise.resolve(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
it("actual four exported routes call the authenticated narrow proxy, with no create", async () => {
  vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", "http://127.0.0.1:9999"); vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://console.test");
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ schema_version: 1, coverage: "product-title-proposals-only", items: [], next_cursor: null }));
  for (let i = 0; i < 3; i++) fetcher.mockResolvedValueOnce(Response.json(titleProposalFixture()));
  vi.stubGlobal("fetch", fetcher);
  expect((await listGET(req(`${base}?view=actionable`))).status).toBe(200);
  expect((await detailGET(req(`${base}/${id}`))).status).toBe(200);
  expect((await decisionPOST(req(`${base}/${id}/decisions`, '{"action":"accept","expected_revision":"1"}'))).status).toBe(200);
  expect((await applyPOST(req(`${base}/${id}/apply`, '{"expected_revision":"2"}'))).status).toBe(200);
  expect((await listPOST()).status).toBe(405); expect(fetcher).toHaveBeenCalledTimes(4);
});
it("late authentication cannot forward after the route already returned not_sent", async () => {
  vi.useFakeTimers(); const fetcher = vi.fn(); vi.stubGlobal("fetch", fetcher); let finish!: () => void;
  auth.wait = () => new Promise<void>((resolve) => { finish = resolve; });
  const pending = decisionPOST(req(`${base}/${id}/decisions`, '{"action":"accept","expected_revision":"1"}'));
  await vi.advanceTimersByTimeAsync(15000);
  expect(await (await pending).json()).toMatchObject({ outcome: "not_sent" });
  finish(); await vi.advanceTimersByTimeAsync(1); expect(fetcher).not.toHaveBeenCalled();
});
it("handles a framework-wrapped Request without using native private slots on its proxy", async () => {
  vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", "http://127.0.0.1:9999");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ schema_version: 1, coverage: "product-title-proposals-only", items: [], next_cursor: null })));
  const wrapped = new Proxy(req(`${base}?view=actionable`), { get(target, key) { const value = Reflect.get(target, key, target); return typeof value === "function" ? value.bind(target) : value; } });
  expect((await listGET(wrapped)).status).toBe(200);
});
