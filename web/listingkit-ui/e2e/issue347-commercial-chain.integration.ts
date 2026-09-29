import {createServer,type Server} from "node:http";
import type {AddressInfo} from "node:net";
import {afterAll,beforeAll,expect,it,vi} from "vitest";
import {NextRequest} from "next/server";
import {getCommercialOverview} from "@/lib/api/commercial";
import {getCommercialOrderSummary,getCommercialOrders,getCommercialWallet,getCommercialWalletEntries} from "@/lib/api/commercial-billing";

// Explicitly collected by the PostgreSQL harness config, not Playwright.
// Only external/session identity retrieval is substituted. The route export,
// BFF transport, typed client, Go auth/owner and PostgreSQL execute for real.
const fixtureSession=vi.hoisted(()=>({fixture:"issue347-commercial-chain",accessToken:"fixture-token",userId:"fixture-user"}));
vi.mock("@/auth",()=>({serverAuth:(handler:(request:NextRequest & {auth?:unknown},context?:unknown)=>Promise<Response|void>)=>async(request:NextRequest,context?:unknown)=>{Object.defineProperty(request,"auth",{value:fixtureSession,configurable:true});return handler(request as NextRequest & {auth?:unknown},context);}}));
vi.mock("@/lib/server/zitadel-server-token",()=>({readZitadelServerAccessToken:(session:unknown)=>{if(session!==fixtureSession)throw new Error("commercial BFF route did not receive the authenticated fixture session");return "fixture-token";}}));
vi.mock("@/lib/server/zitadel-auth",()=>({readZitadelIdentityFromSession:(session:unknown)=>{if(session!==fixtureSession)throw new Error("commercial BFF route did not pass the authenticated fixture session to identity lookup");return {userId:"fixture-user"};}}));
import * as route from "@/app/api/workbench/commercial/overview/route";
import * as walletRoute from "@/app/api/workbench/commercial/wallet/route";
import * as walletEntriesRoute from "@/app/api/workbench/commercial/wallet/entries/route";
import * as ordersRoute from "@/app/api/workbench/commercial/orders/route";
import * as orderSummaryRoute from "@/app/api/workbench/commercial/orders/summary/route";

const upstream=process.env.COMMERCIAL_INTEGRATION_ORIGIN;
if(!upstream || new URL(upstream).hostname!=="127.0.0.1") throw new Error("Start this suite through TestCommercialHTTPPostgresBFFClientZeroWrites; a private loopback fixture is required");
const nativeFetch=globalThis.fetch;
let server:Server,bffOrigin:string,selected="org-B";
const asResponse=async(value:Response|void|Promise<Response|void>):Promise<Response>=>await value??new Response(null,{status:401});
const bffRoutes=new Map<string,(request:NextRequest)=>Promise<Response>>([
  ["/api/workbench/commercial/overview",request=>asResponse(route.GET(request))],
  ["/api/workbench/commercial/wallet",request=>asResponse(walletRoute.GET(request,{params:Promise.resolve({})}))],
  ["/api/workbench/commercial/wallet/entries",request=>asResponse(walletEntriesRoute.GET(request,{params:Promise.resolve({})}))],
  ["/api/workbench/commercial/orders",request=>asResponse(ordersRoute.GET(request,{params:Promise.resolve({})}))],
  ["/api/workbench/commercial/orders/summary",request=>asResponse(orderSummaryRoute.GET(request,{params:Promise.resolve({})}))],
]);
beforeAll(async()=>{
  process.env.COMMERCIAL_API_ORIGIN=upstream;
  server=createServer(async(req,res)=>{
    const controller=new AbortController();res.on("close",()=>{if(!res.writableEnded)controller.abort();});
    try {
      const headers=new Headers();for(const [key,value] of Object.entries(req.headers)) if(value!==undefined)headers.set(key,Array.isArray(value)?value.join(","):value);
      const request=new NextRequest(new URL(req.url!,bffOrigin),{method:req.method,headers,signal:controller.signal});
      const handler=req.method==="GET"?bffRoutes.get(new URL(req.url!,bffOrigin).pathname):undefined;
      const response=handler?await handler(request):req.method==="POST"?route.POST():new Response(null,{status:404});
      res.writeHead(response.status,Object.fromEntries(response.headers));res.end(Buffer.from(await response.arrayBuffer()));
    } catch {res.statusCode=500;res.end();}
  });
  await new Promise<void>(resolve=>server.listen(0,"127.0.0.1",resolve));bffOrigin=`http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  vi.stubGlobal("fetch",(input:RequestInfo|URL,init?:RequestInit)=>{
    if(typeof input==="string" && input.startsWith("/")) {const headers=new Headers(init?.headers);headers.set("cookie",`shuomi_effective_organization=${selected}`);return nativeFetch(new URL(input,bffOrigin),{...init,headers});}
    return nativeFetch(input,init);
  });
});
afterAll(async()=>{vi.unstubAllGlobals();if(server)await new Promise<void>((resolve,reject)=>server.close(err=>err?reject(err):resolve()));});
const mode=async(value:number)=>{const response=await nativeFetch(`${upstream}/fixture/mode`,{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({mode:value})});expect(response.status).toBe(204);};

it("executes actual HTTP client → Next BFF → Go verified scope → native owners → isolated PostgreSQL",async()=>{
 const first=await getCommercialOverview("org-B");expect(first.organization_id).toBe("org-B");expect(first.base_plan).toMatchObject({code:"base_payg",subscription_required:false,store_period_days:30});expect(first.store_services).toEqual({state:"available",value:{records:0,active:0,expired:0,expiring_soon:0}});if(first.resources.state!=="available")throw new Error("native resource read unavailable");expect(first.resources.value.resources.find(r=>r.resource_type==="ai_point")).toMatchObject({state:"recorded",available:"9007199254740993"});
 selected="org-C";const second=await getCommercialOverview(selected);if(second.resources.state!=="available")throw new Error("native resource read unavailable");expect(second.resources.value.resources.find(r=>r.resource_type==="ai_point")).toMatchObject({state:"recorded",available:"2"});await expect(getCommercialOverview("org-B")).rejects.toMatchObject({status:409,code:"ORGANIZATION_CONTEXT_CHANGED"});
 selected="org-empty";const empty=await getCommercialOverview(selected);if(empty.resources.state!=="available")throw new Error("native resource read unavailable");expect(empty.resources.value.resources.every(r=>r.state==="not_recorded"&&r.available===null)).toBe(true);
  selected="org-viewer";await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"PERMISSION_DENIED"});
  selected="not-granted";await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"ORGANIZATION_ACCESS_DENIED"});
  selected="org-B";await getCommercialOverview(selected); // populate the real grant cache
  await mode(1);await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"ORGANIZATION_ACCESS_REVOKED"});
  await mode(2);await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:503,code:"DEPENDENCY_UNAVAILABLE"});
  await mode(0);expect((await getCommercialOverview(selected)).base_plan.code).toBe("base_payg");
  await mode(3);const controller=new AbortController();const pending=getCommercialOverview(selected,controller.signal);setTimeout(()=>controller.abort(),25);await expect(pending).rejects.toMatchObject({status:504,code:"DEADLINE_EXCEEDED"});await mode(0);
  const noScope=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview`);expect(noScope.status).toBe(409);
  const method=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview`,{method:"POST"});expect(method.status).toBe(405);
  const apiMethod=await nativeFetch(`${upstream}/api/v1/workbench/commercial/overview`,{method:"POST"});expect(apiMethod.status).toBe(404);
  const badQuery=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview?tenant_id=victim`,{headers:{cookie:"shuomi_effective_organization=org-B","X-Expected-Organization-ID":"org-B"}});expect(badQuery.status).toBe(400);
  console.info("CHAIN PASS: actual HTTP BFF/client; current native resource and Store owners; exact BIGINT; live grants; empty resource facts; cancellation; GET-only access");
});

it("reads wallet, entries, orders, and summary through their actual BFF routes and separate read-only owners",async()=>{
  const wallet=await getCommercialWallet("fixture-user","org-B");
  expect(wallet).toMatchObject({organization_id:"org-B",currency:"CNY",available_minor:"0",reserved_minor:"0",debt_minor:"0",lifetime_topup_minor:"0",lifetime_spend_minor:"0",version:"1"});
  const entries=await getCommercialWalletEntries("fixture-user","org-B");
  expect(entries).toMatchObject({organization_id:"org-B",items:[],next_cursor:""});
  const orders=await getCommercialOrders("fixture-user","org-B");
  expect(orders).toMatchObject({organization_id:"org-B",items:[],next_cursor:""});
  const summary=await getCommercialOrderSummary("fixture-user","org-B");
  expect(summary.organization_id).toBe("org-B");
  expect(summary.spend_minor).toBe("0");
  expect(Date.parse(summary.from)).toBeLessThan(Date.parse(summary.until));
  selected="org-C";
  await expect(getCommercialWallet("fixture-user","org-B")).rejects.toMatchObject({status:409,code:"ORGANIZATION_CONTEXT_CHANGED"});
  selected="org-B";
  console.info("BILLING READ PASS: actual wallet/entries/orders/summary BFF routes; isolated empty billing+money owners; no fabricated payment or accounting rows");
});
