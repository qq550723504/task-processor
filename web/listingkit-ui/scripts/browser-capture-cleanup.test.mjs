import assert from "node:assert/strict";
import { test } from "node:test";
import { finishOwnedBrowserAndProxy } from "./browser-capture-cleanup.mjs";

function fixture() {
  const calls = [];
  let result;
  return {
    calls, result: () => result,
    options: {
      closeBrowser: async () => { calls.push("browser"); },
      verifyBrowser: async () => { calls.push("verify"); },
      closeProxy: async () => { calls.push("proxy"); },
      record: async value => { result = value; },
    },
  };
}
test("success records all owned cleanup steps", async () => {
  const f = fixture();
  await finishOwnedBrowserAndProxy(f.options);
  assert.deepEqual(f.calls, ["browser", "verify", "proxy"]);
  assert.deepEqual(f.result(), { browserClosed: true, browserVerified: true, proxyClosed: true, primaryFailed: false, failureCount: 0 });
});
test("browser close failure still executes verification and proxy close", async () => {
  const f = fixture(), failure = new Error("synthetic browser close failure");
  f.options.closeBrowser = async () => { f.calls.push("browser"); throw failure; };
  await assert.rejects(finishOwnedBrowserAndProxy(f.options), error => error === failure);
  assert.deepEqual(f.calls, ["browser", "verify", "proxy"]);
  assert.equal(f.result().browserClosed, false);
  assert.equal(f.result().proxyClosed, true);
});
test("browser identity assertion failure still executes proxy close", async () => {
  const f = fixture(), failure = new Error("synthetic identity assertion failure");
  f.options.verifyBrowser = async () => { f.calls.push("verify"); throw failure; };
  await assert.rejects(finishOwnedBrowserAndProxy(f.options), error => error === failure);
  assert.deepEqual(f.calls, ["browser", "verify", "proxy"]);
  assert.equal(f.result().browserVerified, false);
  assert.equal(f.result().proxyClosed, true);
});
test("primary and cleanup failures retain their order and cannot become PASS", async () => {
  const f = fixture(), primary = new Error("synthetic primary"), close = new Error("synthetic close"), proxy = new Error("synthetic proxy");
  f.options.closeBrowser = async () => { throw close; };
  f.options.closeProxy = async () => { throw proxy; };
  await assert.rejects(finishOwnedBrowserAndProxy({ ...f.options, failure: { error: primary } }), error => {
    assert(error instanceof AggregateError);
    assert.deepEqual(error.errors, [primary, close, proxy]);
    return true;
  });
  assert.equal(f.result().failureCount, 3);
});
test("receipt write failure cannot become PASS after successful resource cleanup", async () => {
  const f = fixture(), failure = new Error("synthetic receipt failure");
  f.options.record = async () => { throw failure; };
  await assert.rejects(finishOwnedBrowserAndProxy(f.options), error => error === failure);
  assert.deepEqual(f.calls, ["browser", "verify", "proxy"]);
});
