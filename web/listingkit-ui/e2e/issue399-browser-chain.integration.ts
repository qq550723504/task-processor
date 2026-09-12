import { readFile, writeFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { chromium } from "@playwright/test";
import { expect, it, vi } from "vitest";
import { capture1688, verifyBrowserCapture, readBrowserCapture, readBrowserCaptureByKey } from "@/lib/api/browser-capture";
import { acquire1688 } from "@/lib/api/product-acquisition";
import { browserCaptureFixture } from "@/lib/contracts/browser-capture.fixture";
type Manifest = { origin: string; goOrigin: string; controlKey: string; evidencePath: string; sourceHead: string; sessions: Record<string,{cookie:string;subject:string}> };
const path=process.env.BROWSER_CAPTURE_FIXTURE_MANIFEST;
if (!path) throw new Error("BROWSER_CAPTURE_FIXTURE_MANIFEST is required");
const manifest=JSON.parse(await readFile(path,"utf8")) as Manifest;
const nativeFetch=globalThis.fetch;
const mark=(stage:string)=>writeFile(manifest.evidencePath.replace("evidence.json","progress.json"),JSON.stringify({stage}));
let actor="operator",organization="B";
vi.stubGlobal("fetch",async(input:RequestInfo|URL,init:RequestInit={})=>{
  if(typeof input!=="string"||!input.startsWith("/"))return nativeFetch(input,init);
  const headers=new Headers(init.headers);
  headers.set("Cookie",`${manifest.sessions[actor]!.cookie}; shuomi_effective_organization=${organization}`);
  headers.set("Origin",manifest.origin);headers.set("Sec-Fetch-Site","same-origin");
  headers.set("Authorization","Bearer browser-forged-must-be-ignored");headers.set("X-Requested-Organization-ID","browser-forged-must-be-ignored");
  return nativeFetch(new URL(input,manifest.origin),{...init,headers});
});
async function control(action:string) {
  const response=await nativeFetch(`${manifest.goOrigin}/__issue399/${action}`,{method:action==="observe"?"GET":"POST",headers:{"X-Fixture-Control":manifest.controlKey}});
  expect(response.ok).toBe(true);return action==="observe"?response.json():undefined;
}
it("actual client -> Next/Auth.js -> BFF -> mounted Browser -> runtime PostgreSQL",async()=>{
  await mark("session");
  const body=JSON.stringify(browserCaptureFixture());const key=randomUUID();const scope={userId:"operator",organizationId:"B"};const intent={...scope,key,body};
  const session=await nativeFetch(`${manifest.origin}/api/auth/session`,{headers:{Cookie:manifest.sessions.operator!.cookie}});
  expect(session.ok).toBe(true);const publicSession=await session.json();expect(publicSession.identity.userId).toBe("operator");expect(publicSession.accessToken).toBeUndefined();
  await mark("capture");const first=await capture1688(intent);expect(first).toMatchObject({outcome:"published",catalogVersion:"1",productKey:"crawler:1688:981645030344",replayed:false});expect(first.operationId).not.toBe(key);expect(first.missingFacts.length).toBeGreaterThan(0);
  expect((await control("observe")).publicFetches).toBe(0);
  await mark("public-and-replay");const publicResult=await acquire1688({...scope,key:randomUUID(),source:"981645030344"});expect(publicResult.catalogVersion).toBe("2");
  for(const result of [await capture1688(intent),await verifyBrowserCapture(intent),await readBrowserCaptureByKey(scope,key),await readBrowserCapture(scope,first.operationId)])expect(result).toMatchObject({outcome:"published",replayed:true,catalogVersion:"1",publicationId:first.publicationId});
  await expect(capture1688({...intent,body:body.replace("12.34000001","12.34000002")})).rejects.toMatchObject({code:"IDEMPOTENCY_CONFLICT"});
  await expect(acquire1688({...scope,key,source:"981645030344"})).rejects.toMatchObject({code:"IDEMPOTENCY_CONFLICT"});
  await mark("scope-and-revocation");organization="A";await expect(readBrowserCaptureByKey({...scope,organizationId:"A"},key)).rejects.toMatchObject({code:"ACQUISITION_NOT_FOUND"});
  organization="B";actor="operator2";await expect(readBrowserCaptureByKey({...scope,userId:"operator2"},key)).rejects.toMatchObject({code:"ACQUISITION_NOT_FOUND"});
  await expect(capture1688(intent)).rejects.toMatchObject({code:"IDENTITY_CONTEXT_CHANGED"});
  actor="viewer";await expect(capture1688({...intent,userId:"viewer",key:randomUUID()})).rejects.toMatchObject({status:403});actor="operator";
  await control("revoke");await expect(readBrowserCaptureByKey(scope,key)).rejects.toMatchObject({status:403});await expect(capture1688({...intent,key:randomUUID()})).rejects.toMatchObject({status:403});await control("restore");
  await mark("lost-response-and-read-only");const lostKey=randomUUID();await control("drop-next");await expect(capture1688({...intent,key:lostKey})).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
  const before=await control("observe");expect(before.counts.product_snapshot_versions).toBe(3);
  await control("restart");const recovered=await readBrowserCaptureByKey(scope,lostKey);expect(recovered).toMatchObject({outcome:"published",catalogVersion:"3",replayed:true});
  await control("read-only");
  expect(await readBrowserCaptureByKey(scope,lostKey)).toEqual(recovered);expect(await verifyBrowserCapture({...intent,key:lostKey})).toEqual(recovered);expect((await readBrowserCapture(scope,first.operationId)).catalogVersion).toBe("1");
  const after=await control("observe");expect(after).toEqual(before);expect(after.cachedGrants).toBe(0);expect(after.publicFetches).toBe(1);
  await expect(readBrowserCaptureByKey(scope,randomUUID())).rejects.toMatchObject({code:"ACQUISITION_NOT_FOUND"});expect((await control("observe")).capturePosts).toBe(before.capturePosts);
  await mark("browser-recovery");await browserRecovery(lostKey);
  await writeFile(manifest.evidencePath,JSON.stringify({passed:true,sourceHead:manifest.sourceHead,actual:["Browser TypeScript client","Next/Auth.js encrypted fixture session","Workbench BFF","mounted current-application Browser route middleware","Browser service/staging/SRC-1/Catalog","task-owned PostgreSQL runtime role","Chromium receiver reload and key-free login new tab"],controlled:["external token verifier","live grant provider","Auth.js session issuance/replacement","public 1688 HTML fixture"],notRun:["official ZITADEL login completion","real1688","production cutover","actual-main combination"],snapshotCount:3,readOnlyRecoveryUnchanged:true},null,2));
});

async function browserRecovery(key:string) {
  const browser=await chromium.launch({headless:true});
  const context=await browser.newContext({viewport:{width:1280,height:900}});
  async function select(name:string,org:string) {
    await context.clearCookies();
    const cookie=manifest.sessions[name]!.cookie;
    await context.addCookies([{name:"authjs.session-token",value:cookie.slice(cookie.indexOf("=")+1),url:manifest.origin},{name:"shuomi_effective_organization",value:org,url:manifest.origin}]);
  }
  try {
    await select("operator","B");const page=await context.newPage();
    const url=`${manifest.origin}/capture/1688#operationKey=${key}`;
    const before=await control("observe");
    await page.goto(url);await page.getByRole("button",{name:"Check original operation"}).click();
    await page.getByText("Published version 3",{exact:true}).waitFor();
    await context.clearCookies();await page.reload();
    const link=page.getByRole("link",{name:"在新标签页重新登录"});await link.waitFor();
    expect(await link.getAttribute("href")).toBe("/login?returnTo=%2Fworkbench");
    expect(await link.getAttribute("rel")).toBe("noopener noreferrer");
    const requests:Array<{path:string;search:string;referrer:string|undefined}>=[];
    context.on("request",request=>{const requestURL=new URL(request.url());if(requestURL.origin===manifest.origin&&(requestURL.pathname==="/login"||requestURL.pathname==="/api/zitadel-auth/login"))requests.push({path:requestURL.pathname,search:requestURL.search,referrer:request.headers().referer});});
    const opened=context.waitForEvent("page");await link.click();const popup=await opened;
    await popup.waitForLoadState("domcontentloaded").catch(()=>undefined);
    expect(page.url()).toBe(url);expect(requests.some(r=>r.path==="/login")).toBe(true);
    for(const request of requests){expect(request.search).not.toContain(key);expect(request.referrer??"").not.toContain(key);if(request.path==="/login")expect(request.referrer??"").toBe("");}
    await popup.close();
    // The provider boundary is deliberately controlled: replace only this
    // isolated browser's fixture session, not any real user's IAM/session.
    await select("operator2","B");await page.getByRole("button",{name:"登录完成后刷新此页核实"}).click();
    await page.getByRole("button",{name:"Check original operation"}).click();await page.getByText(/No operation is visible for this key/).waitFor();expect(page.url()).toBe(url);
    await select("operator","A");await page.reload();await page.getByRole("button",{name:"Check original operation"}).click();await page.getByText(/No operation is visible for this key/).waitFor();expect(page.url()).toBe(url);
    await select("operator","B");await page.setViewportSize({width:390,height:844});await page.reload();await page.getByRole("button",{name:"Check original operation"}).click();await page.getByText("Published version 3",{exact:true}).waitFor();
    expect(await page.locator("body").evaluate(element=>element.scrollWidth<=window.innerWidth)).toBe(true);
    await page.screenshot({path:manifest.evidencePath.replace("evidence.json","receiver-mobile.png"),fullPage:true});
    const after=await control("observe");expect(after.capturePosts).toBe(before.capturePosts);expect(after.counts).toEqual(before.counts);expect(after.stagingDigest).toBe(before.stagingDigest);
  } finally {await context.close();await browser.close();}
}
