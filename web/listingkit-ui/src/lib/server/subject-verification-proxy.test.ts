import { afterEach, expect, it, vi } from "vitest";
import { proxyVerification } from "./subject-verification-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { parseVerification } from "../api/subject-verification";
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const input = {companyName:"企业",creditCode:"91310000MA00000001",consent:true,idempotencyKey:"35e86bfb-9c42-4c85-9647-b5a1702c57e9"};
function request(overrides: Record<string,string> = {}) {return new Request("http://localhost/api/account/verification/applications", {method:"POST",headers:{"Content-Type":"application/json",Origin:"http://localhost","X-Expected-User-ID":"user","X-Expected-Organization-ID":"org",cookie:`${WORKBENCH_COOKIE_NAME}=org`,...overrides},body:JSON.stringify(input)});}
it("rejects stale organization, session and cross-origin writes before dispatch",async()=>{
 const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 const cases: Record<string,string>[] = [{"X-Expected-Organization-ID":"other"},{"X-Expected-User-ID":"other"},{Origin:"https://other.test"}];
 for(const headers of cases) { expect((await proxyVerification(request(headers),"token","user")).status).toBeGreaterThanOrEqual(400); }
 expect(fetch).not.toHaveBeenCalled();
});
it("preserves uncertain mutation outcome without retrying",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://owner/api/v1");const fetch=vi.fn().mockRejectedValue(new TypeError("lost response"));vi.stubGlobal("fetch",fetch);
 const response=await proxyVerification(request(),"private-token","user");expect(await response.json()).toMatchObject({code:"RESULT_UNVERIFIED",outcome:"unknown"});expect(fetch).toHaveBeenCalledTimes(1);
 expect(fetch.mock.calls[0][0]).toBe("http://owner/api/v1/account/organization/verification/applications");
});
it("refuses wrong-subject or unsafe capability URLs",()=>{
 const state={state:"PENDING",userId:"user",organizationId:"org",canStart:false,maskedPhone:"138****0001",applicationId:"app",companyName:"企业",creditCode:"91310000MA00000001",isApplicant:true,verificationUrl:"https://qian.tencent.com/auth",expiresAt:"2026-10-26T00:00:00Z"};
 expect(parseVerification(state,"user","org").state).toBe("PENDING");
 expect(()=>parseVerification(state,"user","other")).toThrow();
 expect(()=>parseVerification({...state,verificationUrl:"https://qian.tencent.com.evil.test"},"user","org")).toThrow();
 expect(()=>parseVerification({...state,state:"VERIFIED"},"user","org")).toThrow();
});
