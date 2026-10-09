import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyAgentCustomization } from "./agent-customization-proxy";
const id = "b510c346-54e7-4cbd-91b2-05f7c0149b42";
const input = { recordId:id,expectedRevision:3 };
const delivery = { id, requestId: id, organizationId: "org-a", definition: "product.quality.check", version: "1.0.0", name: "商品资料质检", createdBy: "staff", createdAt: "2026-10-09T00:00:00Z" };
function request(path = "", body?: unknown) { return new Request("https://console.test/api/workbench/agent-customization/agents" + path, { method: body ? "POST" : "GET", headers: { "Content-Type": "application/json", Origin: "https://console.test", "X-Expected-User-ID": "user-a", "X-Expected-Organization-ID": "org-a", Cookie: "shuomi_effective_organization=org-a", "Idempotency-Key": id }, ...(body ? { body: JSON.stringify(body) } : {}) }); }
beforeEach(() => { vi.restoreAllMocks(); vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "https://service.test/api/v1"); vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://console.test"); });
afterEach(() => vi.unstubAllEnvs());
it("loads only the selected enterprise private deliveries", async () => {
  const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(Response.json({ items: [delivery], nextCursor: "" }));
  expect((await proxyAgentCustomization(request(), "token", "user-a")).status).toBe(200);
  expect(String(fetch.mock.calls[0][0])).toBe("https://service.test/api/v1/agent-customization/agents");
  expect(new Headers(fetch.mock.calls[0][1]?.headers).get("X-Requested-Organization-ID")).toBe("org-a");
  fetch.mockResolvedValue(Response.json({ items: [{ ...delivery, organizationId: "org-b" }], nextCursor: "" }));
  expect((await proxyAgentCustomization(request(), "token", "user-a")).status).toBe(502);
});
it("rejects injected authority and retains unknown execution identity", async () => {
  const fetch = vi.spyOn(globalThis, "fetch");
  for (const body of [{ ...input, organizationId: "org-b" }, { ...input, definition: "arbitrary" },{...input,issues:[]},{name:"盒子",material:"",dimensions:"",description:"",specifications:[]}]) expect((await proxyAgentCustomization(request(`/${id}/reports`, body), "token", "user-a")).status).toBe(400);
  expect(fetch).not.toHaveBeenCalled(); fetch.mockRejectedValue(new Error("response lost"));
  expect(await (await proxyAgentCustomization(request(`/${id}/reports`, input), "token", "user-a")).json()).toEqual({ code: "OUTCOME_UNKNOWN" });
  expect(new Headers(fetch.mock.calls[0][1]?.headers).get("Idempotency-Key")).toBe(id);
});
it("rejects another actor, wrong report identity and substituted draft references", async () => {
  const hash="a".repeat(64),at="2026-10-09T00:00:00Z";
  const draft={recordId:id,revision:3,sourceId:id,preparationId:id,storeId:id,platform:"shein",site:"shein-us",productKey:"own:fixture",productVersion:"1",title:"草稿",recordHash:hash,productHash:hash,rulesHash:hash,inventoryHash:hash,savedAt:at};
  const run={id,deliveryId:id,organizationId:"org-a",actorId:"user-a",key:id,definition:"product.quality.check",version:"2.0.0",draft:{...draft,issues:[],readyForUpload:true},createdAt:at};
  const summary={id,deliveryId:id,organizationId:"org-a",actorId:"user-a",version:"2.0.0",draft,title:"草稿",findingCount:0,createdAt:at};
  const fetch=vi.spyOn(globalThis,"fetch").mockResolvedValue(Response.json({items:[summary],nextCursor:""}));
  expect((await proxyAgentCustomization(request(`/${id}/reports`),"token","user-a")).status).toBe(200);
  fetch.mockResolvedValue(Response.json({items:[{...summary,actorId:"other-member"}],nextCursor:""}));
  expect((await proxyAgentCustomization(request(`/${id}/reports`),"token","user-a")).status).toBe(502);
  fetch.mockResolvedValue(Response.json(run));
  expect((await proxyAgentCustomization(request(`/${id}/reports/${id}`),"token","user-a")).status).toBe(200);
  fetch.mockResolvedValue(Response.json({...run,id:"11111111-1111-4111-8111-111111111111"}));
  expect((await proxyAgentCustomization(request(`/${id}/reports/${id}`),"token","user-a")).status).toBe(502);
  fetch.mockResolvedValue(Response.json({...run,draft:{...run.draft,revision:4}}));
  expect(await(await proxyAgentCustomization(request(`/${id}/reports`,input),"token","user-a")).json()).toEqual({code:"OUTCOME_UNKNOWN"});
});
