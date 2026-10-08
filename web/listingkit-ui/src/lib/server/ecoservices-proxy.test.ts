import {afterEach,expect,it,vi} from "vitest";
import {proxyEcoservices} from "./ecoservices-proxy";
import {WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
import {ecoMerchantSchema,ecoCheckoutSchema} from "@/lib/api/ecoservices";
import {ecoservicesEndpoint} from "./ecoservices-proxy";
afterEach(()=>{vi.unstubAllGlobals();vi.unstubAllEnvs()});
const id="4841d296-ef14-4c16-8d25-a7667e534feb";
it("keeps actual fee refresh platform scoped and rejects submitted amounts or provider addresses",()=>{
 const url=new URL("http://localhost:3000/api/admin/ecoservices/requests/"+id+"/fees");
 const route=ecoservicesEndpoint(url,"POST");expect(route?.admin).toBe(true);expect(route?.input?.safeParse({date:"2026-10-08"}).success).toBe(true);
 for(const value of [{date:"2026-10-08",amountMinor:"1"},{date:"2026-10-08",downloadUrl:"https://foreign.example"},{date:"2026-10-08",merchantId:"other"}])expect(route?.input?.safeParse(value).success).toBe(false);
 expect(ecoservicesEndpoint(new URL(url.href.replace("/api/admin/","/api/")),"POST")).toBeNull();expect(ecoservicesEndpoint(new URL(url.href.replace("/fees","/financial")),"POST")).toBeNull();
});
it("bounds original merchant resume and never exposes arbitrary channel fields or links",()=>{
 const path="http://localhost:3000/api/ecoservices/applications/"+id+"/merchant/resume";
 expect(ecoservicesEndpoint(new URL(path),"POST")).toMatchObject({cas:true,key:true,admin:false});expect(ecoservicesEndpoint(new URL(path+"?merchantId=other"),"POST")).toBeNull();expect(ecoservicesEndpoint(new URL(path.replace("/api/ecoservices/","/api/admin/ecoservices/")),"POST")).toBeNull();
 const view={revisionVersion:"1",canCorrect:false,verificationPending:false,id,applicationId:id,state:"NEED_SIGN",signState:"UNSIGNED",signUrl:"https://pay.weixin.qq.com/public/apply4ec_sign/s?applymentId=1&sign=original",legalValidationUrl:"",reason:"",updatedAt:"2026-10-08T00:00:00Z"};expect(ecoMerchantSchema.safeParse(view).success).toBe(true);for(const extra of [{merchantId:"arbitrary"},{sealedDetails:"private"},{signUrl:"https://attacker.example/sign"},{signUrl:"https://pay.weixin.qq.com.attacker.example/public/sign"}])expect(ecoMerchantSchema.safeParse({...view,...extra}).success).toBe(false);
});
const headers={"X-Expected-User-ID":"actor","X-Expected-Organization-ID":"org",cookie:WORKBENCH_COOKIE_NAME+"=org",Origin:"http://localhost:3000"};
function configure(){vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://127.0.0.1:9000/api/v1");vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost:3000")}
it("delivers the original supported Native up QR through the BFF and browser contract",async()=>{
 configure();const codeUrl="weixin://wxpay/bizpayurl/up?pr=original";
 const fetch=vi.fn().mockResolvedValue(Response.json({orderId:id,codeUrl}));vi.stubGlobal("fetch",fetch);
 const request=new Request("http://localhost:3000/api/ecoservices/orders/"+id+"/checkout",{method:"POST",headers:{...headers,"Content-Type":"application/json","If-Match":'"7"'},body:"{}"});
 const response=await proxyEcoservices(request,"server-token","actor");expect(response.status).toBe(200);
 expect(ecoCheckoutSchema.parse(await response.json())).toEqual({orderId:id,codeUrl});expect(fetch).toHaveBeenCalledTimes(1);
});
it("forwards exact application correction versions and rejects malformed versions",async()=>{
 configure();const application={id,companyName:"corrected",registrationNumber:"registration",categories:["COMPANY_REGISTRATION"],regions:["China"],fileIds:[id],state:"SUBMITTED",version:"3",agreementVersion:"current",agreementAccepted:false,onboardingState:"NOT_STARTED",reviewReason:"",updatedAt:"2026-10-08T00:00:00Z"};const fetch=vi.fn().mockImplementation(()=>Promise.resolve(Response.json({application})));vi.stubGlobal("fetch",fetch);const body=JSON.stringify({companyName:"corrected",registrationNumber:"registration",categories:["COMPANY_REGISTRATION"],regions:["China"],fileIds:[id]});
 const make=(cas:string)=>new Request("http://localhost:3000/api/ecoservices/applications",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id,"If-Match":cas},body});
 expect((await proxyEcoservices(make('"2"'),"server-token","actor")).status).toBe(200);expect(new Headers(fetch.mock.calls[0][1].headers).get("If-Match")).toBe('"2"');
 for(const cas of ['"02"','"0"','2','"9223372036854775808"'])expect((await proxyEcoservices(make(cas),"server-token","actor")).status).toBe(400);expect(fetch).toHaveBeenCalledTimes(1);
});
it("refuses stale scopes, arbitrary provider paths and cross-site mutations before dispatch",async()=>{
 configure();const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 for(const [path,options,status]of [["catalog?organizationId=other",{},400],["requests?page=1&page=2",{},400],["requests",{headers:{...headers,cookie:WORKBENCH_COOKIE_NAME+"=other"}},409],["provider/refund",{method:"POST"},400],["requests/"+id+"/accept",{method:"POST",headers:{...headers,Origin:"http://foreign.test"}},403]] as const){expect((await proxyEcoservices(new Request("http://localhost:3000/api/ecoservices/"+path,{headers,...options}),"server-token","actor")).status).toBe(status)}
 expect(fetch).not.toHaveBeenCalled();
});
it("rejects ambiguous exact-version consent and overposted ownership",async()=>{
 configure();const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 for(const body of [`{"deliveryVersion":"3","deliveryVersion":"4"}`,`{"deliveryVersion":"3","organizationId":"other"}`,`{"deliveryVersion":"03"}`,`{"deliveryVersion":3}`]){
 const request=new Request("http://localhost:3000/api/ecoservices/requests/"+id+"/accept",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id,"If-Match":"\"7\""},body});expect((await proxyEcoservices(request,"server-token","actor")).status).toBe(400)}expect(fetch).not.toHaveBeenCalled();
});
it("forwards only the server token and selected organization and rejects private upstream fields",async()=>{
 configure();const fetch=vi.fn().mockResolvedValue(Response.json({listings:[],total:"0"}));vi.stubGlobal("fetch",fetch);
 const request=new Request("http://localhost:3000/api/ecoservices/catalog",{headers:{...headers,Authorization:"Bearer browser-token","X-Requested-Organization-ID":"other"}});
 expect((await proxyEcoservices(request,"server-token","actor")).status).toBe(200);const sent=new Headers(fetch.mock.calls[0][1].headers);expect(sent.get("Authorization")).toBe("Bearer server-token");expect(sent.get("X-Requested-Organization-ID")).toBe("org");expect(sent.has("cookie")).toBe(false);
 fetch.mockResolvedValueOnce(Response.json({listings:[],total:"0",privateKey:"private"}));expect((await proxyEcoservices(new Request(request.url,{headers}),"server-token","actor")).status).toBe(502);
});
it("forwards the same-org managed qualification boolean without disclosing join-only application data",async()=>{
 configure();const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 const request=()=>new Request("http://localhost:3000/api/ecoservices/provider/listings?page=1&pageSize=20",{headers});
 for(const providerQualified of [true,false]){fetch.mockResolvedValueOnce(Response.json({providerQualified,listings:[],total:"0"}));const result=await proxyEcoservices(request(),"server-token","actor");expect(result.status).toBe(200);expect(await result.json()).toEqual({providerQualified,listings:[],total:"0"})}
 fetch.mockResolvedValueOnce(Response.json({providerQualified:"true",listings:[],total:"0"}));expect((await proxyEcoservices(request(),"server-token","actor")).status).toBe(502);
 fetch.mockResolvedValueOnce(Response.json({providerQualified:true,listings:[],total:"0",merchantId:"private-sub"}));expect((await proxyEcoservices(request(),"server-token","actor")).status).toBe(502);
 expect(new Headers(fetch.mock.calls[0][1].headers).get("X-Requested-Organization-ID")).toBe("org");
});
it("keeps global platform scope neutral and preserves unknown mutation identity",async()=>{
 configure();const fetch=vi.fn().mockResolvedValue(Response.json({applications:[],total:"0"}));vi.stubGlobal("fetch",fetch);
 expect((await proxyEcoservices(new Request("http://localhost:3000/api/admin/ecoservices/applications",{headers:{"X-Expected-User-ID":"actor"}}),"server-token","actor")).status).toBe(200);
 expect(new Headers(fetch.mock.calls[0][1].headers).has("X-Requested-Organization-ID")).toBe(false);
 fetch.mockRejectedValueOnce(new Error("lost acknowledgement"));const response=await proxyEcoservices(new Request("http://localhost:3000/api/ecoservices/requests/"+id+"/accept",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id,"If-Match":"\"7\""},body:`{"deliveryVersion":"3"}`}),"server-token","actor");expect((await response.json()).code).toBe("OUTCOME_UNKNOWN");expect(new Headers(fetch.mock.calls[1][1].headers).get("Idempotency-Key")).toBe(id);
});
