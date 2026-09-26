import { open } from "node:fs/promises";

// Acceptance-only observer; never sends, retries or changes a request.
// This is a projection of the existing BFF/owner contract, not a diagnostic API.
const ownerErrors = new Map([
  ["referral_invalid", 400], ["referral_authentication_required", 401],
  ["referral_missing", 404], ["referral_conflict", 409],
  ["referral_expired", 410], ["referral_verification_pending", 409],
  ["referral_outcome_unknown", 503], ["referral_capacity_exceeded", 429],
  ["referral_unavailable", 503],
]);
const bffErrors = new Set([
  "AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED", "PERMISSION_DENIED",
  "INVALID_REQUEST", "DEADLINE_EXCEEDED", "DEPENDENCY_UNAVAILABLE",
  "REFERRALS_NOT_CONFIGURED", "INVALID_UPSTREAM_RESPONSE",
]);
const opaque = (value, max) => typeof value === "string" &&
  value.length <= max && /^[A-Za-z0-9_-]+$/.test(value) ? value : null;

async function bounded(operation) {
  let timer;
  try {
    return await Promise.race([
      operation(),
      new Promise((_, reject) => { timer = setTimeout(() => reject(new Error("unobserved")), 2000); }),
    ]);
  } finally { clearTimeout(timer); }
}

export async function captureReferralCompletion(page, { origin, file }) {
  const base = new URL(origin);
  if (!["localhost", "127.0.0.1", "[::1]"].includes(base.hostname) ||
      !["http:", "https:"].includes(base.protocol) || origin !== base.origin) {
    throw new Error("evidence_requires_explicit_loopback_origin");
  }
  let handle;
  try { handle = await open(file, "wx", 0o600); }
  catch { throw new Error("evidence_open_failed"); }
  const attempts = new Map();
  let sequence = 0, failed = false, queue = Promise.resolve(), closing;
  const enqueue = operation => { queue = queue.then(operation).catch(() => { failed = true; }); };
  const persist = async row => {
    await handle.writeFile(`${JSON.stringify(row)}\n`);
    await handle.sync();
  };
  const onRequest = request => {
    const url = new URL(request.url());
    if (request.method() !== "POST" || url.origin !== origin ||
        url.pathname !== "/api/account/referrals/complete" || url.search || url.hash || request.url().endsWith("?")) return;
    const row = {
      attempt: ++sequence, observedAt: new Date().toISOString(), stage: "REQUEST",
      expectedSubject: null, expectedSubjectSource: "NOT_OBSERVED",
      authenticatedSubject: null, subjectSource: "NOT_OBSERVED",
      httpStatus: null, httpSource: "NOT_OBSERVED", errorCode: null, errorSource: "NOT_OBSERVED",
      intentId: null, intentSource: "NOT_OBSERVED",
      verifiedReadback: null, verifiedSource: "NOT_OBSERVED",
      receiptPresent: null, receiptSource: "NOT_OBSERVED", outcome: "UNKNOWN",
    };
    attempts.set(request, row);
    enqueue(async () => {
      // Read only this public assertion. Never enumerate headers or read bodies,
      // cookies, storage, Auth.js session payloads, or verification URLs.
      try {
        row.expectedSubject = opaque(await bounded(() => request.headerValue("x-expected-user-id")), 128);
        if (row.expectedSubject) row.expectedSubjectSource = "REQUEST_EXPECTED_USER_ID";
      }
      catch { /* The request still gets a durable UNKNOWN record. */ }
      await persist(row);
    });
  };
  const onResponse = response => {
    const row = attempts.get(response.request());
    if (!row) return;
    attempts.delete(response.request());
    const observedAt = new Date().toISOString();
    enqueue(async () => {
      row.observedAt = observedAt;
      row.stage = "RESPONSE";
      row.httpStatus = response.status();
      row.httpSource = "BROWSER_RESPONSE";
      try {
        const bytes = await bounded(() => response.body());
        if (bytes.length > 16 * 1024) throw new Error("unobserved");
        const body = JSON.parse(bytes.toString("utf8"));
        if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("unobserved");
        const ownerResponse = ownerErrors.get(body.code) === row.httpStatus;
        row.errorCode = ownerErrors.has(body.code) || bffErrors.has(body.code) ? body.code : null;
        if (row.errorCode) row.errorSource = "BFF_RESPONSE_CODE";
        const receipt = row.httpStatus === 200 && body.status === "complete" &&
          opaque(body.intentID, 200) && Object.keys(body).length === 3 &&
          typeof body.boundAt === "string" && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(body.boundAt) &&
          Number.isFinite(Date.parse(body.boundAt));
        // These replies are only forwarded after the BFF's Auth.js assertion
        // check. This is contract evidence, never a raw session observation.
        if ((ownerResponse || receipt) && row.expectedSubject) {
          row.authenticatedSubject = row.expectedSubject;
          row.subjectSource = "BFF_VALIDATED_EXPECTATION";
          if (receipt) {
            row.intentId = body.intentID;
            row.intentSource = "BFF_COMPLETION_RESPONSE";
            row.receiptPresent = true;
            row.receiptSource = "OWNER_COMPLETION_RECEIPT";
            row.outcome = "COMPLETED";
          } else if (body.code === "referral_verification_pending") {
            row.verifiedReadback = false;
            row.verifiedSource = "OWNER_PENDING_CONTRACT";
            row.outcome = "PENDING";
          }
        }
      } catch { row.errorCode = "RESPONSE_UNOBSERVED"; row.errorSource = "CAPTURE_DIAGNOSTIC"; }
      await persist(row);
    });
  };
  const onFailure = request => {
    const row = attempts.get(request);
    if (!row) return;
    attempts.delete(request);
    const observedAt = new Date().toISOString();
    enqueue(async () => {
      row.observedAt = observedAt;
      row.stage = "REQUEST_FAILED";
      row.errorCode = "TRANSPORT_UNOBSERVED";
      row.errorSource = "CAPTURE_DIAGNOSTIC";
      await persist(row);
    });
  };
  page.on("request", onRequest);
  page.on("response", onResponse);
  page.on("requestfailed", onFailure);
  return () => {
    if (closing) return closing;
    page.off("request", onRequest);
    page.off("response", onResponse);
    page.off("requestfailed", onFailure);
    closing = (async () => {
      try { await queue; } finally { await handle.close(); }
      if (failed) throw new Error("evidence_write_failed");
      return { attemptCount: sequence, status: sequence > 0 ? "OBSERVED" : "NOT_RUN" };
    })();
    return closing;
  };
}
