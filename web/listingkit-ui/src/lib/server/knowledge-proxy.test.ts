import {afterEach,expect,it,vi} from "vitest";
import {proxyKnowledge} from "./knowledge-proxy";
import {WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
afterEach(()=>{vi.unstubAllGlobals();vi.unstubAllEnvs();});
const headers={"X-Expected-User-ID":"actor","X-Expected-Organization-ID":"org",cookie:WORKBENCH_COOKIE_NAME+"=org",Origin:"http://localhost:3000"};
const id="4841d296-ef14-4c16-8d25-a7667e534feb";
function configure(){vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://127.0.0.1:9000/api/v1");vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost:3000");}
it("rejects stale contexts, unknown paths, duplicate query, and cross-site writes before dispatch",async()=>{
 configure();const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 for(const [path,options,status] of [
 ["knowledge-bases?page=1&page=2",{},400],
 ["knowledge-bases?organizationId=other",{},400],
 ["knowledge-bases",{headers:{...headers,cookie:WORKBENCH_COOKIE_NAME+"=other"}},409],
 ["knowledge-bases",{headers:{...headers,cookie:headers.cookie+"; "+headers.cookie}},409],
 ["knowledge-bases/"+id+"/restore",{method:"POST",headers:{...headers}},400],
 ["knowledge-bases",{method:"POST",headers:{...headers,Origin:"http://foreign.test"}},403],
 ] as const){const request=new Request("http://localhost:3000/api/workbench/"+path,{headers,...options});expect((await proxyKnowledge(request,"server-token","actor")).status).toBe(status);}
 expect(fetch).not.toHaveBeenCalled();
});
it("rejects ambiguous JSON and validates CAS before a business mutation",async()=>{
 configure();const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
 for(const [body,cas] of [[`{"name":"a","name":"b"}`,'"1"'],[`{"name":"a","organizationId":"other"}`,'"1"'],[`{"name":"a"}`,"1"],[`{"name":"a"}`,'"01"']]){
 const request=new Request("http://localhost:3000/api/workbench/knowledge-bases/"+id,{method:"PUT",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id,"If-Match":cas},body});
 expect((await proxyKnowledge(request,"server-token","actor")).status).toBe(400);}
 expect(fetch).not.toHaveBeenCalled();
});
it("uses the server credential and fresh selected organization, strips unapproved response data",async()=>{
 configure();const fetch=vi.fn().mockResolvedValue(Response.json({items:[],pagination:{page:1,pageSize:20,total:0}}));vi.stubGlobal("fetch",fetch);
 const request=new Request("http://localhost:3000/api/workbench/knowledge-bases",{headers:{...headers,Authorization:"Bearer browser-token","X-Requested-Organization-ID":"other"}});
 const response=await proxyKnowledge(request,"server-token","actor");
 expect(response.status).toBe(200);expect(response.headers.get("Cache-Control")).toBe("private, no-store");
 const forwarded=new Headers(fetch.mock.calls[0][1].headers);expect(forwarded.get("Authorization")).toBe("Bearer server-token");expect(forwarded.get("X-Requested-Organization-ID")).toBe("org");expect(forwarded.has("cookie")).toBe(false);
 fetch.mockResolvedValueOnce(Response.json({items:[],pagination:{page:1,pageSize:20,total:0},objectKey:"private"}));
 expect((await proxyKnowledge(new Request(request.url,{headers}),"server-token","actor")).status).toBe(502);
});
it("preserves the idempotency command on an ambiguous upstream mutation",async()=>{
 configure();const fetch=vi.fn().mockRejectedValue(new Error("transport failed"));vi.stubGlobal("fetch",fetch);
 const response=await proxyKnowledge(new Request("http://localhost:3000/api/workbench/knowledge-bases",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id},body:JSON.stringify({name:"Brand"})}),"server-token","actor");
 expect((await response.json()).code).toBe("OUTCOME_UNKNOWN");expect(new Headers(fetch.mock.calls[0][1].headers).get("Idempotency-Key")).toBe(id);
});
it("treats post-admission database 503 as unknown so retry retains the original operation",async()=>{
 configure();const fetch=vi.fn().mockResolvedValue(Response.json({code:"KNOWLEDGE_UNAVAILABLE",message:"知识库请求未完成",requestId:"",fieldErrors:[]},{status:503}));vi.stubGlobal("fetch",fetch);
 const request=new Request("http://localhost:3000/api/workbench/knowledge-bases",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":id},body:JSON.stringify({name:"Brand"})});
 const response=await proxyKnowledge(request,"server-token","actor");
 expect((await response.json()).code).toBe("OUTCOME_UNKNOWN");
});
