import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { dirname, join, resolve } from "node:path";
import type { AddressInfo } from "node:net";
import { chromium, type BrowserContext, type Page } from "@playwright/test";
import { it } from "vitest";
import { browserCaptureSchema, BROWSER_CAPTURE_BASE } from "@/lib/contracts/browser-capture";
import { acquisitionResultSchema } from "@/lib/contracts/product-acquisition";

const nativeFetch = globalThis.fetch;
const pluginHead = "7d689dc19461b2b6b60ffc972020e6c06283fca0";
const sourceURL = "https://detail.1688.com/offer/981645030344.html";
const missedInterceptionProbe = "https://detail.1688.com/offer/0.html?issue399-denied-probe";
type Manifest = {
  origin: string; goOrigin: string; controlKey: string; sourceHead: string; evidencePath: string;
  pluginRoot: string; pluginHead: string; pluginBuild: Record<string, string>; profilePath: string;
  sessions: Record<string, { cookie: string; subject: string }>;
};
type Observation = { capturePosts: number; stagingDigest: string };
const pause = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
async function waitFor<T>(read: () => Promise<T | undefined | false>, label: string): Promise<T> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const value = await read();
    if (value) return value;
    await pause(100);
  }
  throw new Error(label);
}

// Registered after the original actual-chain test, never before its read-only proof.
export function registerFrozenExtensionCombination() {
  it("combines frozen extension DOM/handoff with the actual receiver, BFF and persistence", async () => {
    const manifestPath = process.env.BROWSER_CAPTURE_FIXTURE_MANIFEST;
    assert(manifestPath);
    const m = JSON.parse(await readFile(manifestPath, "utf8")) as Manifest;
    const dir = dirname(manifestPath);
    assert.equal(m.pluginHead, pluginHead);
    assert.equal(new URL(m.origin).hostname, "127.0.0.1");
    assert.equal(resolve(m.profilePath), join(resolve(dir), "extension-profile"));
    const previous = JSON.parse(await readFile(m.evidencePath, "utf8"));
    assert.equal(previous.passed, true);
    assert.equal(previous.sourceHead, m.sourceHead);
    await writeFile(m.evidencePath, JSON.stringify({ ...previous, passed: false, extensionCombination: { passed: false, pluginHead } }));
    const stage = async (name: string) => writeFile(join(dir, "progress.json"), JSON.stringify({ stage: "extension-" + name }));
    const control = async (action: string) => {
      const response = await nativeFetch(m.goOrigin + "/__issue399/" + action, { method: "POST", headers: { "X-Fixture-Control": m.controlKey }, signal: AbortSignal.timeout(5000) });
      assert.equal(response.ok, true, "FIXTURE_CONTROL_FAILED");
      await response.arrayBuffer();
    };
    const observe = async (): Promise<Observation> => {
      const response = await nativeFetch(m.goOrigin + "/__issue399/observe", { headers: { "X-Fixture-Control": m.controlKey }, signal: AbortSignal.timeout(5000) });
      assert.equal(response.ok, true);
      return response.json();
    };
    await stage("offline-fixture");
    const html = await readFile(join(m.pluginRoot, "tests", "fixtures", "product.html"), "utf8");
    const dist = join(m.pluginRoot, "dist-fixture");
    for (const [file, hash] of Object.entries(m.pluginBuild)) {
      assert.equal(createHash("sha256").update(await readFile(join(dist, file))).digest("hex"), hash);
    }
    await mkdir(m.profilePath);
    const allowed = new URL(m.origin);
    const missedLoopbackProbe = m.goOrigin + "/__issue399/observe";
    assert.notEqual(new URL(missedLoopbackProbe).origin, allowed.origin);
    const ownedRuntimeURL = new URL("../../../scripts/issue357/io.mjs", import.meta.url).href;
    const ownedRuntime = await import(/* @vite-ignore */ ownedRuntimeURL);
    const cleanupURL = new URL("../scripts/browser-capture-cleanup.mjs", import.meta.url).href;
    const { finishOwnedBrowserAndProxy } = await import(/* @vite-ignore */ cleanupURL);
    const network = { allowedLoopbackRequests: 0, deniedHTTP: 0, deniedCONNECT: 0, deniedLoopbackProbe: 0, deniedSourceConnect: 0, interceptedFixture: 0, abortedOther: 0, proxyConnectionErrors: 0 };
    const browserDiagnostics: Record<string, string | number>[] = [];
    let consoleErrorCount = 0;
    const diagnostic = (value: Record<string, string | number>) => { if (browserDiagnostics.length < 100) browserDiagnostics.push(value); };
    const category = (message: string) => message.includes("Content Security Policy") ? "CSP"
      : /Cross-Origin|CORS/.test(message) ? "CORS" : /local network|private network/i.test(message) ? "LOCAL_NETWORK"
        : message.includes("Loading chunk") ? "CHUNK_LOAD" : /WebSocket/i.test(message) ? "WEBSOCKET"
          : message.includes("HMR") ? "HMR" : /hydrat/i.test(message) ? "HYDRATION" : /React/.test(message) ? "REACT_INIT" : "OTHER";
    // This proxy never resolves or connects to an external host, even if page
    // interception is missed or a worker bypasses Playwright's route handler.
    const proxy = createServer((request, response) => {
      network.deniedHTTP++;
      if (request.url === missedLoopbackProbe) network.deniedLoopbackProbe++;
      request.resume();
      response.writeHead(403, { Connection: "close", "Content-Type": "text/plain" }).end("Task proxy denied");
    });
    proxy.on("connect", (request, socket) => {
      socket.on("error", () => { network.proxyConnectionErrors++; socket.destroy(); });
      network.deniedCONNECT++;
      if (request.url === "detail.1688.com:443") network.deniedSourceConnect++;
      socket.end("HTTP/1.1 403 Forbidden\r\nConnection: close\r\nContent-Length: 0\r\n\r\n");
    });
    proxy.on("upgrade", (_request, socket) => socket.destroy());
    await new Promise<void>((resolve, reject) => { proxy.once("error", reject); proxy.listen(0, "127.0.0.1", resolve); });
    const proxyPort = (proxy.address() as AddressInfo).port;
    let context: BrowserContext | undefined;
    let released = false;
    let verified = false;
    let browserRecord: { pid: number } | undefined;
    let failure: { error: unknown } | undefined;
    try {
      await stage("owned-chromium");
      context = await chromium.launchPersistentContext(m.profilePath, {
        executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
        headless: true,
        ignoreDefaultArgs: ["--disable-extensions"],
        viewport: { width: 1280, height: 900 },
        proxy: { server: "http://127.0.0.1:" + proxyPort, bypass: "<-loopback>," + allowed.origin },
        args: ["--no-first-run", "--no-default-browser-check", "--disable-background-networking",
          "--enable-unsafe-extension-debugging", "--remote-debugging-port=0",
          "--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1, EXCLUDE localhost"],
      });
      const browser = context.browser();
      assert(browser);
      context.on("page", page => {
        page.on("websocket", socket => {
          const target = new URL(socket.url());
          diagnostic({ event: "websocket", target: target.protocol === "ws:" && target.hostname === allowed.hostname && target.port === allowed.port && target.pathname === "/_next/webpack-hmr" ? "OWNED_NEXT_HMR" : "OTHER" });
        });
        page.on("pageerror", error => diagnostic({ event: "pageerror", category: category(error.message), kind: ["Error", "TypeError", "ReferenceError", "SyntaxError", "ChunkLoadError"].includes(error.name) ? error.name : "OTHER" }));
        page.on("console", message => {
          if (message.type() === "error" && ++consoleErrorCount <= 10) diagnostic({ event: "consoleerror", category: category(message.text()), code: message.text().match(/net::ERR_[A-Z_]+/)?.[0] ?? "NONE" });
        });
      });
      const requestLabel = (url: string) => {
        const parsed = new URL(url);
        if (parsed.origin !== allowed.origin) return "not-app";
        if (parsed.pathname.startsWith("/_next/static/")) return parsed.pathname.split("/").at(-1) ?? "static";
        if (parsed.pathname === "/capture/1688") return "receiver-document";
        if (parsed.pathname === "/api/auth/session") return "auth-session";
        if (parsed.pathname === "/api/workbench/context") return "workbench-context";
        return "other-app";
      };
      context.on("request", request => diagnostic({ event: "request", asset: requestLabel(request.url()), type: request.resourceType() }));
      context.on("response", response => diagnostic({ event: "response", asset: requestLabel(response.url()), status: response.status() }));
      context.on("requestfailed", request => diagnostic({ event: "requestfailed", asset: requestLabel(request.url()), code: request.failure()?.errorText.match(/net::ERR_[A-Z_]+/)?.[0] ?? "OTHER" }));
      const browserCDP = await browser.newBrowserCDPSession();
      const processes = await browserCDP.send("SystemInfo.getProcessInfo");
      const ownProcess = processes.processInfo.find(process => process.type === "browser");
      assert(ownProcess && Number.isSafeInteger(ownProcess.id));
      browserRecord = await ownedRuntime.processIdentity(ownProcess.id);
      await writeFile(join(dir, "extension-process.json"), JSON.stringify(browserRecord));
      await context.route("**/*", async route => {
        const url = route.request().url();
        if (url === sourceURL) { network.interceptedFixture++; await route.fulfill({ status: 200, contentType: "text/html; charset=utf-8", body: html }); return; }
        if (url === missedInterceptionProbe || url === missedLoopbackProbe) { await route.continue(); return; }
        if (new URL(url).origin === allowed.origin) { network.allowedLoopbackRequests++; await route.continue(); return; }
        network.abortedOther++;
        await route.abort("blockedbyclient");
      });
      const source = context.pages()[0] ?? await context.newPage();
      await stage("fail-closed-probe");
      // Chrome's asynchronous error-page commit can interrupt a subsequent
      // navigation on the same tab. Keep the denied probe off the capture tab.
      const probe = await context.newPage();
      try {
        await assert.rejects(probe.goto(missedInterceptionProbe, { timeout: 10_000 }));
        assert(network.deniedSourceConnect > 0, "EXTERNAL_SOURCE_NOT_DENIED_BY_PROXY");
      }
      finally { await probe.close(); }
      const loopbackProbe = await context.newPage();
      try {
        const response = await loopbackProbe.goto(missedLoopbackProbe, { timeout: 10_000 });
        assert.equal(response?.status(), 403);
        assert.equal(network.deniedLoopbackProbe, 1, "UNALLOWED_LOOPBACK_PORT_BYPASSED_PROXY");
      } finally { await loopbackProbe.close(); }
      await source.goto(sourceURL);
      assert.equal(network.interceptedFixture, 1);
      const title = await source.locator("h2").innerText();
      const cookie = m.sessions.operator.cookie;
      const separator = cookie.indexOf("=");
      assert(separator > 0);
      // Controlled fixture issuance only, never collect a user's browser session.
      await context.addCookies([
        { name: cookie.slice(0, separator), value: cookie.slice(separator + 1), url: m.origin, httpOnly: true, sameSite: "Lax" },
        { name: "shuomi_effective_organization", value: "B", url: m.origin, sameSite: "Lax" },
      ]);
      await control("restore");
      await control("read-write");
      const before = await observe();
      await stage("real-extension-action");
      await browserCDP.send("Extensions.loadUnpacked" as never, { path: dist } as never);
      const worker = context.serviceWorkers()[0] ?? await context.waitForEvent("serviceworker", { timeout: 15_000 });
      const extensionId = new URL(worker.url()).hostname;
      assert.match(extensionId, /^[a-p]{32}$/);
      const permissions = await worker.evaluate(() => {
        const api = globalThis as unknown as { chrome: { runtime: { getManifest(): { permissions: string[]; host_permissions?: string[] } } } };
        return api.chrome.runtime.getManifest();
      });
      assert.deepEqual(permissions.permissions.slice().sort(), ["activeTab", "scripting"]);
      assert.equal(permissions.host_permissions, undefined);
      const targets = await browserCDP.send("Target.getTargets", { filter: [{ type: "tab", exclude: false }] });
      const tabs = targets.targetInfos.filter(target => target.type === "tab");
      assert.equal(tabs.length, 1, "OWNED_SOURCE_TAB_NOT_UNIQUE");
      await browserCDP.send("Extensions.triggerAction" as never, { id: extensionId, targetId: tabs[0].targetId } as never);
      const popupTarget = await waitFor(async () => {
        const info = await browserCDP.send("Target.getTargets");
        return info.targetInfos.find(target => target.url === "chrome-extension://" + extensionId + "/popup.html");
      }, "OWNED_POPUP_NOT_FOUND");
      const activePort = (await readFile(join(m.profilePath, "DevToolsActivePort"), "utf8")).trim().split(/\r?\n/);
      assert.match(activePort[0], /^\d+$/);
      assert.match(activePort[1], /^\/devtools\/browser\/[a-z0-9-]+$/i);
      assert(Number(activePort[0]) > 0 && Number(activePort[0]) <= 65535);
      const CDP = createRequire(join(m.pluginRoot, "package.json"))("chrome-remote-interface");
      const popup = await CDP({ target: "ws://127.0.0.1:" + activePort[0] + "/devtools/page/" + popupTarget.targetId, local: true });
      let receiver: Page;
      try {
        const evaluate = async (expression: string): Promise<unknown> => {
          const result = await popup.Runtime.evaluate({ expression, userGesture: true, returnByValue: true, awaitPromise: true });
          assert.equal(result.exceptionDetails, undefined, "OWNED_POPUP_EVALUATION_FAILED");
          return result.result.value;
        };
        await waitFor(async () => await evaluate("!!document.querySelector('#capture') && !document.querySelector('#capture').disabled"), "POPUP_CAPTURE_NOT_READY");
        await evaluate("document.querySelector('#capture').click()");
        await waitFor(async () => await evaluate("!!document.querySelector('#handoff') && !document.querySelector('#handoff').disabled"), "DOM_CAPTURE_NOT_READY");
        const opened = context.waitForEvent("page", { timeout: 15_000 });
        await evaluate("document.querySelector('#handoff').click()");
        receiver = await opened;
      } finally { await popup.close().catch(() => undefined); }
      await stage("receiver-before-consent");
      await receiver.waitForURL(url => url.pathname === "/capture/1688");
      const beforeFocus = await receiver.evaluate(() => ({ visibility: document.visibilityState, focused: document.hasFocus(), online: navigator.onLine }));
      await receiver.bringToFront();
      const afterFocus = await receiver.evaluate(() => ({ visibility: document.visibilityState, focused: document.hasFocus(), online: navigator.onLine }));
      await writeFile(join(dir, "receiver-focus.json"), JSON.stringify({ beforeFocus, afterFocus }));
      const initialURL = new URL(receiver.url());
      assert.equal(initialURL.origin, m.origin);
      const fragment = new URLSearchParams(initialURL.hash.slice(1));
      assert.equal(fragment.get("extensionId"), extensionId);
      assert.equal([...fragment.keys()].length, 3);
      const key = fragment.get("idempotencyKey");
      assert(key && /^[0-9a-f-]{36}$/.test(key));
      await receiver.getByRole("heading", { name: title, exact: true }).waitFor();
      assert.equal((await observe()).capturePosts, before.capturePosts, "HANDOFF_POSTED_WITHOUT_CONFIRMATION");
      let submittedBody: string | undefined;
      let receiverPosts = 0;
      receiver.on("request", request => {
        if (new URL(request.url()).pathname === BROWSER_CAPTURE_BASE && request.method() === "POST") {
          receiverPosts++;
          submittedBody = request.postData() ?? undefined;
        }
      });
      await control("drop-next");
      await receiver.getByRole("button", { name: "Confirm and submit" }).click();
      await receiver.getByRole("status").filter({ hasText: "Outcome is unknown." }).waitFor({ timeout: 35_000 });
      assert.equal(new URL(receiver.url()).hash, "#operationKey=" + key);
      assert.equal(receiverPosts, 1);
      assert(submittedBody);
      const payload = browserCaptureSchema.parse(JSON.parse(submittedBody));
      assert.equal(payload.evidence.title, title);
      assert.equal(payload.evidence.sourceURL, sourceURL);
      assert.equal(payload.evidence.variants[0].sourceID, "99999999999999999999");
      assert.equal(payload.evidence.variants[0].price?.amount, "1.23000001");
      for (const marker of ["PASSWORD_CANARY", "AUTH_CANARY", "TOKEN_CANARY", "SESSION_CANARY"]) {
        assert.equal(submittedBody.includes(marker), false, "NON_PRODUCT_DOM_DATA_CAPTURED");
      }
      await stage("committed-response-loss-recovery");
      await receiver.getByRole("button", { name: "Check original operation" }).click();
      await receiver.getByRole("status").filter({ hasText: "Published version 4" }).waitFor();
      const committed = await observe();
      assert.equal(committed.capturePosts, before.capturePosts + 1);
      const requests: string[] = [];
      receiver.on("request", request => {
        const path = new URL(request.url()).pathname;
        if (request.method() === "GET" && path.startsWith(BROWSER_CAPTURE_BASE + "/")) requests.push(path);
      });
      await control("read-only");
      await control("restart");
      await receiver.getByRole("button", { name: "Check original operation" }).click();
      await receiver.getByRole("status").filter({ hasText: "Published version 4" }).waitFor();
      assert(requests.some(path => !path.includes("/by-key/")), "VERIFIED_RECEIPT_ID_NOT_USED");
      const headers = { Cookie: cookie + "; shuomi_effective_organization=B", Origin: m.origin,
        "X-Expected-User-ID": "operator", "X-Expected-Organization-ID": "B", "Idempotency-Key": key, "Content-Type": "application/json" };
      const verifiedResponse = await nativeFetch(m.origin + BROWSER_CAPTURE_BASE + "/verify", { method: "POST", headers, body: submittedBody, signal: AbortSignal.timeout(25_000) });
      assert.equal(verifiedResponse.status, 200);
      const receipt = acquisitionResultSchema.parse(await verifiedResponse.json());
      assert.equal(receipt.outcome, "published");
      assert.equal(receipt.catalogVersion, "4");
      await receiver.reload();
      await receiver.getByRole("button", { name: "Check original operation" }).click();
      await receiver.getByRole("status").filter({ hasText: "Published version 4" }).waitFor();
      assert.equal(receiverPosts, 1, "RELOAD_REPOSTED_CAPTURE");
      const final = await observe();
      assert.equal(final.capturePosts, committed.capturePosts);
      assert.equal(final.stagingDigest, committed.stagingDigest, "READ_ONLY_RECOVERY_MUTATED_OPERATION");
      assert(network.allowedLoopbackRequests > 0);
      await receiver.screenshot({ path: join(dir, "extension-receiver.png"), fullPage: true });
      verified = true;
      await stage("verified");
    } catch (error) {
      failure = { error };
      const receiver = context?.pages().find(page => { try { return new URL(page.url()).pathname === "/capture/1688"; } catch { return false; } });
      if (receiver) {
        try { await receiver.screenshot({ path: join(dir, "extension-failure.png"), fullPage: true, timeout: 5000 }); }
        catch { /* The primary failure remains authoritative if diagnostic rendering fails. */ }
        try {
          const diagnostic = await receiver.evaluate(async () => {
            const runtime = (globalThis as unknown as { chrome?: { runtime?: { lastError?: unknown; sendMessage(id: string, value: unknown, callback: (response: unknown) => void): void } } }).chrome?.runtime;
            const body = document.body.innerText;
            const state = {
              runtimeAvailable: Boolean(runtime), receiverMounted: body.includes("Confirm a browser capture"), readyState: document.readyState, scriptCount: document.scripts.length,
              visibility: document.visibilityState, focused: document.hasFocus(), online: navigator.onLine,
              handoffExpired: body.includes("Browser handoff expired"), handoffInvalid: body.includes("This handoff is invalid"),
              reading: body.includes("Reading the browser handoff"), contextUnavailable: body.includes("Context changed or access is unavailable"),
              confirmation: body.includes("Confirm and submit"),
            };
            if (!runtime) return { state, response: undefined };
            const params = new URLSearchParams(location.hash.slice(1));
            const response = await new Promise<unknown>(resolve => {
              const timeout = setTimeout(() => resolve(undefined), 1000);
              runtime.sendMessage(params.get("extensionId") ?? "", { version: 1, type: "capture.read", handoffId: params.get("handoffId"), idempotencyKey: params.get("idempotencyKey") }, response => {
                clearTimeout(timeout); resolve(runtime.lastError ? undefined : response);
              });
            });
            return { state, response };
          });
          const envelope = diagnostic.response && typeof diagnostic.response === "object" ? diagnostic.response as Record<string, unknown> : undefined;
          const parsed = browserCaptureSchema.safeParse(envelope?.payload);
          await writeFile(join(dir, "extension-failure-state.json"), JSON.stringify({
            ...diagnostic.state, network, browserDiagnostics, consoleErrorCount, envelopeKeys: envelope ? Object.keys(envelope).sort() : [],
            wirePayloadValid: parsed.success,
            issues: parsed.success ? [] : parsed.error.issues.map(issue => ({ path: issue.path.join("."), code: issue.code })),
          }));
        } catch { /* Optional diagnostics never replace the actual test failure. */ }
      }
    } finally {
      await finishOwnedBrowserAndProxy({
        failure,
        closeBrowser: async () => { await context?.close(); },
        verifyBrowser: async () => { if (browserRecord) assert.equal(await ownedRuntime.sameProcess(browserRecord), false, "OWNED_BROWSER_NOT_CLOSED"); },
        closeProxy: async () => {
          proxy.closeAllConnections();
          await new Promise<void>((resolve, reject) => proxy.close(error => error ? reject(error) : resolve()));
        },
        record: async (result: Record<string, boolean | number>) => {
          await writeFile(join(dir, "extension-cleanup.json"), JSON.stringify({ ...result, pluginHead, sourceHead: m.sourceHead }));
        },
      });
      released = true;
    }
    assert(verified && released);
    await writeFile(m.evidencePath, JSON.stringify({ ...previous, passed: true, extensionCombination: {
      passed: true, pluginHead, sourceHead: m.sourceHead, pluginBuild: m.pluginBuild,
      actual: ["frozen extension action/DOM extraction", "bound external handoff", "receiver explicit confirmation POST",
        "actual Next/Auth.js/BFF/Go/task-PG publication", "committed response loss", "same-key/by-ID/verify read-only recovery after restart",
        "reload without capture POST", "safe decimal/id lexemes", "sensitive DOM canaries excluded"],
      controlled: ["local intercepted product HTML, not real 1688", "new owned Chrome profile", "isolated Auth.js issuance",
        "deny-all proxy, exact Next-origin native bypass and external DNS backstop", "other loopback port negative proof"],
      network, browserAndProxyReleased: released,
      notRun: ["real 1688", "real OIDC authentication", "production deployment", "shared data", "merged #412 audit composition"],
    } }, null, 2));
  });
}
