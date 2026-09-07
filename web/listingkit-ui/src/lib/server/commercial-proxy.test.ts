import {afterEach,expect,it,vi} from "vitest";
import {proxyCommercialRead} from "./commercial-proxy";

afterEach(()=>{vi.unstubAllGlobals();vi.unstubAllEnvs();});
const request=(suffix="",org="org-B")=>new Request(`http://localhost/api/workbench/commercial/overview${suffix}`,{headers:{cookie:"shuomi_effective_organization=org-B","X-Expected-Organization-ID":org,"X-Tenant-ID":"victim","X-User-ID":"attacker"}});
it("rejects scope drift and queries before forwarding",async()=>{
  const fetchMock=vi.fn();vi.stubGlobal("fetch",fetchMock);vi.stubEnv("COMMERCIAL_API_ORIGIN","http://localhost:8888");
  expect((await proxyCommercialRead(request("","org-A"),"fixture-token")).status).toBe(409);
  expect((await proxyCommercialRead(request("?organization_id=victim"),"fixture-token")).status).toBe(400);
  expect(fetchMock).not.toHaveBeenCalled();
});
it("uses a fixed origin and safe headers, strips upstream failures and clears revoked scope",async()=>{
  const fetchMock=vi.fn().mockResolvedValue(Response.json({code:"ORGANIZATION_ACCESS_REVOKED",message:"private upstream payload",requestId:"fixture-request",fieldErrors:[]},{status:403}));
  vi.stubGlobal("fetch",fetchMock);vi.stubEnv("COMMERCIAL_API_ORIGIN","http://localhost:8888");
  const response=await proxyCommercialRead(request(),"fixture-token");
  expect(response.status).toBe(403);expect(await response.text()).not.toContain("private upstream");
  expect(response.headers.get("set-cookie")).toContain("Max-Age=0");
  expect(fetchMock.mock.calls[0][0].toString()).toBe("http://localhost:8888/api/v1/workbench/commercial/overview");
  const init=fetchMock.mock.calls[0][1];expect(init).toMatchObject({method:"GET",cache:"no-store",redirect:"manual"});
  expect(new Headers(init.headers).get("X-Tenant-ID")).toBeNull();
});
it("rejects unconfigured origin, redirect and invalid actual bytes",async()=>{
  vi.stubEnv("COMMERCIAL_API_ORIGIN","https://user:password@example.com/api");expect((await proxyCommercialRead(request(),"fixture-token")).status).toBe(503);
  vi.stubEnv("COMMERCIAL_API_ORIGIN","http://localhost:8888");vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(null,{status:302,headers:{location:"https://other.invalid"}})));
  expect((await proxyCommercialRead(request(),"fixture-token")).status).toBe(502);
});
