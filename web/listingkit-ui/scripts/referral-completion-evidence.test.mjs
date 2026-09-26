import { test, onTestFinished } from "vitest";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { captureReferralCompletion } from "./referral-completion-evidence.mjs";

const subject = "01a00000-0000-7000-8000-000000000001";
async function setup() {
  const dir = await mkdtemp(join(tmpdir(), "referral-evidence-"));
  onTestFinished(() => rm(dir, { recursive: true, force: true }));
  const file = join(dir, "completion.jsonl");
  const page = new EventEmitter();
  const close = await captureReferralCompletion(page, { origin: "https://localhost:19444", file });
  return { page, close, file, rows: async () => (await readFile(file, "utf8")).trim().split("\n").filter(Boolean).map(JSON.parse) };
}
function request(url = "https://localhost:19444/api/account/referrals/complete", expected = subject) {
  return { url: () => url, method: () => "POST", headerValue: async name => {
    assert.equal(name, "x-expected-user-id"); return expected;
  } };
}
function response(req, status, payload) {
  return { request: () => req, status: () => status, body: async () => Buffer.from(JSON.stringify(payload)) };
}

test("retains one attempt before a response and records pending with contract provenance", async t => {
  const f = await setup(t), req = request();
  f.page.emit("request", req);
  f.page.emit("response", response(req, 409, { code: "referral_verification_pending", message: "private-sentinel" }));
  assert.deepEqual(await f.close(), { attemptCount: 1, status: "OBSERVED" });
  const rows = await f.rows();
  assert.equal(rows.length, 2);
  assert.equal(rows[0].httpStatus, null);
  assert.equal(rows[0].outcome, "UNKNOWN");
  assert.equal(rows[1].attempt, rows[0].attempt);
  assert.equal(rows[1].httpStatus, 409);
  assert.equal(rows[1].httpSource, "BROWSER_RESPONSE");
  assert.equal(rows[1].errorCode, "referral_verification_pending");
  assert.equal(rows[1].errorSource, "BFF_RESPONSE_CODE");
  assert.equal(rows[1].expectedSubject, subject);
  assert.equal(rows[1].expectedSubjectSource, "REQUEST_EXPECTED_USER_ID");
  assert.equal(rows[1].authenticatedSubject, subject);
  assert.equal(rows[1].subjectSource, "BFF_VALIDATED_EXPECTATION");
  assert.equal(rows[1].verifiedReadback, false);
  assert.equal(rows[1].verifiedSource, "OWNER_PENDING_CONTRACT");
  assert.equal(rows[1].receiptPresent, null);
  assert.equal(rows[1].intentId, null);
  assert.equal(rows[1].outcome, "PENDING");
  assert.ok(!(await readFile(f.file, "utf8")).includes("private-sentinel"));
});

test("successful receipt does not claim a fresh provider verification on replay", async t => {
  const f = await setup(t), req = request();
  f.page.emit("request", req);
  f.page.emit("response", response(req, 200, { status: "complete", intentID: "opaque-intent_1", boundAt: "2026-09-25T00:00:00Z" }));
  await f.close();
  const row = (await f.rows()).at(-1);
  assert.equal(row.receiptPresent, true);
  assert.equal(row.receiptSource, "OWNER_COMPLETION_RECEIPT");
  assert.equal(row.intentId, "opaque-intent_1");
  assert.equal(row.intentSource, "BFF_COMPLETION_RESPONSE");
  assert.equal(row.verifiedReadback, null);
  assert.equal(row.outcome, "COMPLETED");
});

test("identity rejection, transport loss, malformed and unknown responses remain unclassified", async t => {
  for (const [status, payload] of [[409, { code: "IDENTITY_CONTEXT_CHANGED" }], [503, { code: "DEPENDENCY_UNAVAILABLE" }], [409, { code: "secret-error-sentinel" }], [200, { status: "complete", intentID: "private@example.test" }], [503, { code: "referral_verification_pending" }]]) {
    const f = await setup(t), req = request();
    f.page.emit("request", req); f.page.emit("response", response(req, status, payload));
    await f.close();
    const row = (await f.rows()).at(-1);
    assert.equal(row.authenticatedSubject, null);
    assert.equal(row.verifiedReadback, null);
    assert.equal(row.receiptPresent, null);
    assert.equal(row.outcome, "UNKNOWN");
    assert.ok(!(await readFile(f.file, "utf8")).includes("secret-error-sentinel"));
  }
  const f = await setup(t), req = request();
  f.page.emit("request", req); f.page.emit("requestfailed", req); await f.close();
  assert.equal((await f.rows()).at(-1).httpStatus, null);
  assert.equal((await f.rows()).at(-1).errorCode, "TRANSPORT_UNOBSERVED");
  assert.equal((await f.rows()).at(-1).errorSource, "CAPTURE_DIAGNOSTIC");
});

test("ignores other endpoints, origins, query strings and unattached historical responses", async t => {
  const f = await setup(t);
  for (const url of ["https://evil.test/api/account/referrals/complete", "https://localhost:19444/api/auth/session", "https://localhost:19444/api/account/referrals/complete?token=sentinel"]) {
    const req = request(url); f.page.emit("request", req); f.page.emit("response", response(req, 200, {}));
  }
  f.page.emit("response", response(request(), 200, {}));
  assert.deepEqual(await f.close(), { attemptCount: 0, status: "NOT_RUN" });
  assert.deepEqual(await f.rows(), []);
});

test("does not overwrite earlier evidence and reports open failures safely", async t => {
  const f = await setup(t); await f.close();
  await writeFile(f.file, "preserved");
  await assert.rejects(captureReferralCompletion(f.page, { origin: "https://localhost:19444", file: f.file }), /evidence_open_failed/);
  assert.equal(await readFile(f.file, "utf8"), "preserved");
});

test("preserves request identity and pairing when concurrent replies finish out of order", async t => {
  const f = await setup(t), first = request(), second = request(undefined, "01a00000-0000-7000-8000-000000000002");
  f.page.emit("request", first); f.page.emit("request", second);
  f.page.emit("response", response(second, 503, { code: "referral_outcome_unknown" }));
  f.page.emit("response", response(first, 409, { code: "referral_verification_pending" }));
  await f.close();
  const rows = await f.rows();
  assert.deepEqual(rows.map(row => row.attempt), [1, 2, 2, 1]);
  assert.equal(rows[2].authenticatedSubject, "01a00000-0000-7000-8000-000000000002");
  assert.equal(rows[2].outcome, "UNKNOWN");
  assert.equal(rows[3].authenticatedSubject, subject);
  assert.equal(f.page.listenerCount("response"), 0);
});

test("body read failure preserves HTTP status and no private body or header content", async t => {
  const f = await setup(t), req = request(undefined, "pii@example.test");
  f.page.emit("request", req);
  f.page.emit("response", { request: () => req, status: () => 503, body: async () => { throw new Error("private-token-sentinel"); } });
  await f.close();
  const row = (await f.rows()).at(-1);
  assert.equal(row.httpStatus, 503);
  assert.equal(row.expectedSubject, null);
  assert.equal(row.errorCode, "RESPONSE_UNOBSERVED");
  const raw = await readFile(f.file, "utf8");
  assert.ok(!raw.includes("pii@example.test") && !raw.includes("private-token-sentinel"));
});

test("a still-pending request survives close, and no inferred facts arise without a safe subject", async t => {
  const f = await setup(t), req = request();
  f.page.emit("request", req); await f.close();
  assert.equal((await f.rows()).length, 1);
  assert.equal((await f.rows())[0].outcome, "UNKNOWN");
  const absent = await setup(t), noSubject = request(undefined, null);
  absent.page.emit("request", noSubject);
  absent.page.emit("response", response(noSubject, 409, { code: "referral_verification_pending" }));
  await absent.close();
  assert.equal((await absent.rows()).at(-1).authenticatedSubject, null);
  assert.equal((await absent.rows()).at(-1).verifiedReadback, null);
});
