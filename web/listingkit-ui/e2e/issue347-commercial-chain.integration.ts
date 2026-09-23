import {createServer,type Server} from "node:http";
import type {AddressInfo} from "node:net";
import {afterAll,beforeAll,expect,it,vi} from "vitest";
import {NextRequest} from "next/server";
import {getCommercialOverview} from "@/lib/api/commercial";
import {getCommercialOrderSummary,getCommercialOrders,getCommercialWallet,getCommercialWalletEntries} from "@/lib/api/commercial-billing";

// Explicitly collected by the PostgreSQL harness config, not Playwright.
// Only external/session identity retrieval is substituted. The route export,
// BFF transport, typed client, Go auth/owner and PostgreSQL execute for real.
vi.mock("@/auth",()=>({serverAuth:(handler:(r:NextRequest)=>Promise<Response>)=>handler}));
vi.mock("@/lib/server/zitadel-server-token",()=>({readZitadelServerAccessToken:()=>"fixture-token"}));
vi.mock("@/lib/server/zitadel-auth",()=>({readZitadelIdentityFromSession:()=>({userId:"fixture-user"})}));
import * as route from "@/app/api/workbench/commercial/overview/route";
import * as walletRoute from "@/app/api/workbench/commercial/wallet/route";
import * as walletEntriesRoute from "@/app/api/workbench/commercial/wallet/entries/route";
import * as ordersRoute from "@/app/api/workbench/commercial/orders/route";
import * as orderSummaryRoute from "@/app/api/workbench/commercial/orders/summary/route";

const upstream=process.env.COMMERCIAL_INTEGRATION_ORIGIN;
if(!upstream || new URL(upstream).hostname!=="127.0.0.1") throw new Error("Start this suite through TestCommercialHTTPPostgresBFFClientZeroWrites; a private loopback fixture is required");
const nativeFetch=globalThis.fetch;
let server:Server,bffOrigin:string,selected="org-B";
const bffRoutes=new Map<string,(request:NextRequest)=>Promise<Response>>([
  ["/api/workbench/commercial/overview",request=>route.GET(request)],
  ["/api/workbench/commercial/wallet",request=>walletRoute.GET(request)],
  ["/api/workbench/commercial/wallet/entries",request=>walletEntriesRoute.GET(request)],
  ["/api/workbench/commercial/orders",request=>ordersRoute.GET(request)],
  ["/api/workbench/commercial/orders/summary",request=>orderSummaryRoute.GET(request)],
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

it("executes actual HTTP client → Next BFF → Go verified scope → subscription owner → isolated PostgreSQL",async()=>{
  const first=await getCommercialOverview("org-B");
  expect(first.organization_id).toBe("org-B");expect(first.subscription?.plan_code).toBe("paid_pilot");
  expect(first.plans.map(p=>p.code)).toEqual(["base_payg","paid_pilot"]);expect(first.plans.every(p=>p.price===null&&p.currency===null)).toBe(true);
  expect(first.usage[0]).toMatchObject({state:"known",committed:"1",reserved:"0",period_key:new Date().toISOString().slice(0,7)});
  expect(first.usage[1]).toMatchObject({state:"unknown",committed:null,reserved:null,unit:"operation"});
  expect(first.usage[4]).toMatchObject({state:"known",committed:"9007199254740993",reserved:"-1",unit:"byte",period_key:"__current__",window_start:null});
  expect(first.resource_balance).toEqual({state:"unsupported",value:null});expect(first.cash_balance).toEqual({state:"unsupported",value:null});
  const imageLimit=first.entitlements.find(e=>e.module_code==="listingkit")?.limits.find(l=>l.metric==="product_image_jobs_succeeded");
  expect(imageLimit).toMatchObject({source_key:"product_image_jobs_succeeded",kind:"unlimited",raw_value:"0",value:null,unit:"operation"});
  selected="org-C";const second=await getCommercialOverview("org-C");expect(second.usage[0].committed).toBe("2");
  await expect(getCommercialOverview("org-B")).rejects.toMatchObject({status:409,code:"ORGANIZATION_CONTEXT_CHANGED"});
  selected="org-custom";const custom=await getCommercialOverview(selected);expect(custom.subscription?.plan_code).toBe("定制 plan");expect(custom.plans.map(p=>p.code)).toEqual(["base_payg"]);
  selected="org-empty";const empty=await getCommercialOverview(selected);expect(empty.subscription).toBeNull();expect(empty.entitlements).toEqual([]);expect(empty.usage.every(u=>u.state==="unknown"&&u.committed===null)).toBe(true);
  for(const [org,status] of [["org-expired","expired"],["org-disabled","disabled"],["org-future","not_started"]]) {selected=org;expect((await getCommercialOverview(org)).subscription?.effective_status).toBe(status);}
  selected="org-viewer";await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"PERMISSION_DENIED"});
  selected="not-granted";await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"ORGANIZATION_ACCESS_DENIED"});
  selected="org-B";await getCommercialOverview(selected); // populate the real grant cache
  await mode(1);await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:403,code:"ORGANIZATION_ACCESS_REVOKED"});
  await mode(2);await expect(getCommercialOverview(selected)).rejects.toMatchObject({status:503,code:"DEPENDENCY_UNAVAILABLE"});
  await mode(0);expect((await getCommercialOverview(selected)).usage[0].committed).toBe("1");
  await mode(3);const controller=new AbortController();const pending=getCommercialOverview(selected,controller.signal);setTimeout(()=>controller.abort(),25);await expect(pending).rejects.toMatchObject({status:504,code:"DEADLINE_EXCEEDED"});await mode(0);
  const noScope=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview`);expect(noScope.status).toBe(409);
  const method=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview`,{method:"POST"});expect(method.status).toBe(405);
  const apiMethod=await nativeFetch(`${upstream}/api/v1/workbench/commercial/overview`,{method:"POST"});expect(apiMethod.status).toBe(404);
  const badQuery=await nativeFetch(`${bffOrigin}/api/workbench/commercial/overview?tenant_id=victim`,{headers:{cookie:"shuomi_effective_organization=org-B","X-Expected-Organization-ID":"org-B"}});expect(badQuery.status).toBe(400);
  console.info("CHAIN PASS: actual HTTP BFF/client; home A/effective B; current-window ledger; signed exact BIGINT; no subscription; expired/disabled/future; role denial; live revoke/cache drift/outage; scope switching; cancellation; unsupported cash/resources");
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
