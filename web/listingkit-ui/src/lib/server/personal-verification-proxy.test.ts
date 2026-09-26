import { afterEach,expect,it,vi } from "vitest";
import { proxyPersonalVerification } from "./personal-verification-proxy";
afterEach(()=>{vi.unstubAllGlobals();vi.unstubAllEnvs();});
const input={name:"测试姓名",idNumber:"110101199001010010",metaInfo:"{}",consent:true,idempotencyKey:"35e86bfb-9c42-4c85-9647-b5a1702c57e9"};
function request(headers:Record<string,string>={},body:unknown=input){return new Request("http://localhost/api/account/personal-verification/applications",{method:"POST",headers:{"Content-Type":"application/json",Origin:"http://localhost","X-Expected-User-ID":"user",...headers},body:JSON.stringify(body)});}
it("rejects wrong identity, cross-origin and client phone without dispatch",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://owner/api/v1");const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 for(const req of [request({"X-Expected-User-ID":"other"}),request({Origin:"http://other"}),request({}, {...input,phone:"13800000001"})])expect((await proxyPersonalVerification(req,"token","user")).status).toBeGreaterThanOrEqual(400);
 expect(fetch).not.toHaveBeenCalled();
});
it("preserves lifetime exhaustion without requiring organization cookies",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://owner/api/v1");const fetch=vi.fn().mockResolvedValue(Response.json({code:"VERIFICATION_TOTAL_LIMIT"},{status:429}));vi.stubGlobal("fetch",fetch);
 const r=await proxyPersonalVerification(request(),"token","user");expect(r.status).toBe(429);expect(await r.json()).toMatchObject({code:"VERIFICATION_TOTAL_LIMIT"});expect(fetch).toHaveBeenCalledTimes(1);expect(fetch.mock.calls[0][0]).toBe("http://owner/api/v1/account/verification/applications");expect(fetch.mock.calls[0][1].headers).not.toHaveProperty("X-Requested-Organization-ID");
});
it("does not retry a lost initialization response",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://owner/api/v1");const fetch=vi.fn().mockRejectedValue(new Error("lost"));vi.stubGlobal("fetch",fetch);const r=await proxyPersonalVerification(request(),"token","user");expect(await r.json()).toMatchObject({code:"RESULT_UNVERIFIED",outcome:"unknown"});expect(fetch).toHaveBeenCalledTimes(1);
});
