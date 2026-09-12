import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium } from "@playwright/test";
import { privateDirectory, processIdentity, sameProcess } from "../../../scripts/issue357/io.mjs";
import { finishOwnedBrowserAndProxy } from "./browser-capture-cleanup.mjs";

// Native policy gate, before Auth.js/Go/PG. All accepting targets are newly
// owned loopback sentinels; the proxy never resolves or forwards externally.
const dir = process.argv[2] ?? await mkdtemp(join(tmpdir(), "issue399-native-network-"));
await privateDirectory(dir);
const sourceHead = process.argv[3] ?? null;
if (sourceHead) assert.match(sourceHead, /^[a-f0-9]{40}$/);
const servers = [], counts = { allowed: 0, other: 0, ipv6: 0 }, proxyHits = new Map(), cases = [];
let browser, browserRecord, failure, receipt, currentCase = "setup", currentObservation;
console.log("Owned network evidence=" + join(dir, "network-probe.json"));

async function listener(name, host) {
  const server = createServer((request, response) => {
    counts[name]++;
    request.resume();
    response.writeHead(200, { "Content-Type": "text/html" }).end('<!doctype html><link rel="icon" href="data:,"><h1>owned fixture</h1>');
  });
  server.on("connection", socket => socket.on("error", () => socket.destroy()));
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(0, host, resolve); });
  servers.push(server);
  return server.address().port;
}
try {
  const allowedPort = await listener("allowed", "127.0.0.1");
  const otherPort = await listener("other", "127.0.0.1");
  const ipv6Port = await listener("ipv6", "::1");
  const origin = "http://127.0.0.1:" + allowedPort;
  const proxy = createServer((request, response) => {
    proxyHits.set(request.url, (proxyHits.get(request.url) ?? 0) + 1);
    request.resume();
    response.writeHead(403, { Connection: "close", "Content-Type": "text/plain" }).end("Task proxy denied");
  });
  proxy.on("connect", (request, socket) => {
    socket.on("error", () => socket.destroy());
    const key = "CONNECT " + request.url;
    proxyHits.set(key, (proxyHits.get(key) ?? 0) + 1);
    socket.end("HTTP/1.1 403 Forbidden\r\nConnection: close\r\nContent-Length: 0\r\n\r\n");
  });
  await new Promise((resolve, reject) => { proxy.once("error", reject); proxy.listen(0, "127.0.0.1", resolve); });
  servers.push(proxy);
  browser = await chromium.launch({
    executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe", headless: true,
    proxy: { server: "http://127.0.0.1:" + proxy.address().port, bypass: "<-loopback>," + origin },
    args: ["--disable-background-networking", "--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE 127.0.0.1, EXCLUDE localhost"],
  });
  const cdp = await browser.newBrowserCDPSession();
  const processes = await cdp.send("SystemInfo.getProcessInfo");
  const own = processes.processInfo.find(process => process.type === "browser");
  assert(own && Number.isSafeInteger(own.id));
  browserRecord = await processIdentity(own.id);
  await writeFile(join(dir, "network-probe-process.json"), JSON.stringify(browserRecord));
  const context = await browser.newContext();
  currentCase = "exact-allowed-origin";
  const allowed = await context.newPage();
  const good = await allowed.goto(origin + "/allowed", { timeout: 10_000 });
  assert.equal(good.status(), 200);
  assert.equal(await allowed.locator("h1").innerText(), "owned fixture");
  await allowed.close();
  assert.equal(counts.allowed, 1);
  assert.equal(proxyHits.get(origin + "/allowed") ?? 0, 0);
  const negatives = [
    ["other-127-port", "http://127.0.0.1:" + otherPort + "/denied"],
    ["localhost-other-port", "http://localhost:" + otherPort + "/denied"],
    ["localhost-allowed-port", "http://localhost:" + allowedPort + "/denied"],
    ["ipv6-loopback", "http://[::1]:" + ipv6Port + "/denied"],
    ["https-same-allowed-port", "https://127.0.0.1:" + allowedPort + "/denied"],
    ["external-http", "http://detail.1688.com/offer/0.html"],
    ["external-https", "https://detail.1688.com/offer/0.html"],
  ];
  for (const [name, url] of negatives) {
    currentCase = name;
    const parsed = new URL(url);
    const label = parsed.protocol === "https:" ? "CONNECT " + parsed.hostname + ":" + (parsed.port || "443") : url;
    const before = proxyHits.get(label) ?? 0;
    const page = await context.newPage();
    let status, code;
    try { status = (await page.goto(url, { timeout: 10_000 }))?.status(); }
    catch (error) {
      code = error.message.match(/net::ERR_[A-Z_]+/)?.[0];
      currentObservation = { code: code ?? "OTHER", proxyDenials: (proxyHits.get(label) ?? 0) - before, targetCounts: { ...counts } };
      assert(["net::ERR_TUNNEL_CONNECTION_FAILED", "net::ERR_HTTP_RESPONSE_CODE_FAILURE"].includes(code), "UNEXPECTED_DENIAL_CLASS");
    } finally { await page.close(); }
    assert(status === 403 || code, "MISSING_DENIAL");
    const delta = (proxyHits.get(label) ?? 0) - before;
    assert(delta > 0, "BROWSER_BYPASSED_DENY_PROXY");
    assert.equal(counts.other, 0);
    assert.equal(counts.ipv6, 0);
    assert.equal(counts.allowed, 1);
    cases.push({ name, status: status ?? null, code: code ?? null, proxyDenials: delta,
      targetReceivedRequests: name.startsWith("external-") ? null : 0,
      targetEvidence: name.startsWith("external-") ? "proxy rejected; no external forwarding capability; remote logs not accessed" : "owned sentinel counters unchanged",
    });
  }
  receipt = { passed: true, sourceHead, browserVersion: browser.version(), exactAllowedOrigin: origin,
    allowedTargetRequests: counts.allowed, otherTargetRequests: counts.other, ipv6TargetRequests: counts.ipv6, cases };
} catch (error) {
  failure = { error };
  receipt = { passed: false, sourceHead, currentCase, kind: error.name, signature: ["UNEXPECTED_DENIAL_CLASS", "MISSING_DENIAL", "BROWSER_BYPASSED_DENY_PROXY"].find(value => error.message.includes(value)) ?? "ASSERTION_OR_RUNTIME", currentObservation, completedCases: cases };
} finally {
  await finishOwnedBrowserAndProxy({
    failure,
    closeBrowser: async () => { await browser?.close(); },
    verifyBrowser: async () => { if (browserRecord) assert.equal(await sameProcess(browserRecord), false); },
    closeProxy: async () => {
      const errors = [];
      for (const server of servers.reverse()) {
        try { server.closeAllConnections(); await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve())); }
        catch (error) { errors.push(error); }
      }
      if (errors.length) throw new AggregateError(errors, "OWNED_NETWORK_CLOSE_FAILED");
    },
    record: async cleanup => { await writeFile(join(dir, "network-probe.json"), JSON.stringify({ ...receipt, cleanup }, null, 2)); },
  });
}
console.log("PASS native exact-origin network policy; seven negative targets denied; owned resources closed");
