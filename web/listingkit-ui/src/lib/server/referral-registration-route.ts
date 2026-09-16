import { BlockList, isIP } from "node:net";
import { execFile as execFileCallback } from "node:child_process";
import { lstat, readFile } from "node:fs/promises";
import path from "node:path";
import { promisify } from "node:util";

import { parseTree, type Node as JSONNode, type ParseError } from "jsonc-parser";
import { NextResponse, type NextRequest } from "next/server";
import { z } from "zod";

import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";

const publicInput = z.object({
  code: boundedString(1, 200),
  email: boundedString(3, 254).email(),
  givenName: boundedString(1, 120),
  familyName: boundedString(1, 120),
}).strict();
const resumeInput = z.object({
  intentID: z.string().min(1).max(200).regex(/^[A-Za-z0-9._:-]+$/),
  resumeSecret: z.string().min(43).max(256).regex(/^[A-Za-z0-9_-]+$/),
}).strict();
const utc = z.string().regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/).refine((value) => Number.isFinite(Date.parse(value)));
const admission = z.object({
  intentID: z.string().min(1).max(200).regex(/^[A-Za-z0-9._:-]+$/),
  resumeSecret: z.string().min(43).max(256).regex(/^[A-Za-z0-9_-]+$/),
  createExpiresAt: utc,
  completionExpiresAt: utc,
}).strict().superRefine((value, context) => {
  if (Date.parse(value.completionExpiresAt) <= Date.parse(value.createExpiresAt)) context.addIssue({ code: "custom", message: "Invalid expiry order" });
});
const resumed = z.object({ status: z.literal("created") }).strict();
const safeErrors = new Set([
  "referral_invalid", "referral_authentication_required", "referral_missing", "referral_conflict",
  "referral_expired", "referral_verification_pending", "referral_outcome_unknown",
  "referral_capacity_exceeded", "referral_unavailable",
]);
const responseHeaders = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
const execFile = promisify(execFileCallback);
const loopbackAddresses = new BlockList();
loopbackAddresses.addSubnet("127.0.0.0", 8, "ipv4");
loopbackAddresses.addAddress("::1", "ipv6");
loopbackAddresses.addSubnet("::ffff:127.0.0.0", 104, "ipv6");

export async function handleReferralRegistrationPOST(request: NextRequest, kind: "start" | "resume") {
  if (!hasTrustedSameOriginWrite(request)) return failure(403, "PERMISSION_DENIED");
  const url = new URL(request.url);
  const expectedPath = kind === "start" ? "/api/referral-registration" : "/api/referral-registration/resume";
  if (url.pathname !== expectedPath || url.search || request.url.endsWith("?")) return failure(400, "INVALID_REQUEST");
  if (["authorization", "x-listingkit-client-ip", "x-referral-service-credential", "x-referral-client-ip", "x-requested-organization-id", "x-expected-user-id"].some((name) => request.headers.has(name))) {
    void request.body?.cancel().catch(() => undefined);
    return failure(400, "INVALID_REQUEST");
  }
  // The UI Service accepts ingress only from Traefik (see the paired
  // NetworkPolicy). Traefik removes client-supplied forwarding headers and
  // writes X-Real-IP from its peer connection; browser-specific headers are
  // never an authority for the Go command's rate-limit identity.
  const clientIP = singleHeader(request.headers, "x-real-ip");
  if (!clientIP || isIP(clientIP) === 0 || clientIP.includes("%") || isLoopback(clientIP)) return failure(403, "TRUSTED_PROXY_REQUIRED");
  if (!/^application\/json(?:\s*;|$)/i.test(request.headers.get("content-type") ?? "")) return failure(400, "INVALID_REQUEST");
  const key = kind === "start" ? singleHeader(request.headers, "idempotency-key") : null;
  if (kind === "start" && (!key || !/^[A-Za-z0-9_-]{43,128}$/.test(key))) return failure(400, "INVALID_REQUEST");
  if (kind === "resume" && request.headers.has("idempotency-key")) return failure(400, "INVALID_REQUEST");

  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15_000);
  try {
    const raw = await readRequestBody(request, controller.signal);
    if (raw instanceof Response) return raw;
    const parsed = strictParse(raw);
    const input = (kind === "start" ? publicInput : resumeInput).safeParse(parsed);
    if (!input.success) return failure(400, "INVALID_REQUEST");
    const credential = await readServiceCredential();
    const origin = serviceOrigin();
    if (!credential || !origin) return failure(503, "REFERRALS_NOT_CONFIGURED");
    controller.signal.throwIfAborted();
    const headers = new Headers({
      Accept: "application/json",
      "Content-Type": "application/json",
      "X-Referral-Service-Credential": credential,
      "X-Referral-Client-IP": clientIP,
    });
    if (key) headers.set("Idempotency-Key", key);
    const upstream = await fetch(`${origin}/api/v1/referral-registration/${kind === "start" ? "intents" : "resume"}`, {
      method: "POST",
      headers,
      body: JSON.stringify(input.data),
      cache: "no-store",
      redirect: "manual",
      signal: controller.signal,
    });
    const payload = await readBoundedStrictJSON(upstream, 16 * 1024, controller.signal);
    controller.signal.throwIfAborted();
    if (!upstream.ok) return upstreamFailure(upstream.status, payload);
    const output = (kind === "start" ? admission : resumed).safeParse(payload);
    return output.success ? json(output.data, 200) : failure(502, "INVALID_UPSTREAM_RESPONSE");
  } catch {
    return controller.signal.aborted ? failure(504, "DEADLINE_EXCEEDED") : failure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}

function boundedString(min: number, max: number) {
  return z.string().refine((value) => value === value.trim() && !/[\u0000-\u001f\u007f]/u.test(value) && Buffer.byteLength(value, "utf8") >= min && Buffer.byteLength(value, "utf8") <= max);
}

function singleHeader(headers: Headers, name: string) {
  const value = headers.get(name)?.trim() ?? "";
  return !value || value.includes(",") || /[\r\n]/.test(value) ? null : value;
}

function isLoopback(address: string) {
  return loopbackAddresses.check(address, isIP(address) === 6 ? "ipv6" : "ipv4");
}

async function readRequestBody(request: Request, signal: AbortSignal): Promise<string | Response> {
  const reader = request.body?.getReader();
  if (!reader) return failure(400, "INVALID_REQUEST");
  const cancel = () => { void reader.cancel().catch(() => undefined); };
  signal.addEventListener("abort", cancel, { once: true });
  try {
    const chunks: Uint8Array[] = [];
    let bytes = 0;
    while (true) {
      signal.throwIfAborted();
      const { done, value } = await reader.read();
      if (done) break;
      bytes += value.byteLength;
      if (bytes > 4096) {
        cancel();
        return failure(413, "INPUT_TOO_LARGE");
      }
      chunks.push(value);
    }
    const joined = new Uint8Array(bytes);
    let offset = 0;
    for (const chunk of chunks) { joined.set(chunk, offset); offset += chunk.byteLength; }
    return new TextDecoder("utf-8", { fatal: true }).decode(joined);
  } finally {
    signal.removeEventListener("abort", cancel);
    reader.releaseLock();
  }
}

function strictParse(raw: string): unknown {
  const errors: ParseError[] = [];
  const root = parseTree(raw, errors, { disallowComments: true, allowTrailingComma: false });
  if (!root || errors.length || root.type !== "object") return undefined;
  const pending: JSONNode[] = [root];
  while (pending.length) {
    const node = pending.pop()!;
    if (node.type === "object") {
      const keys = new Set<string>();
      for (const property of node.children ?? []) {
        const key = property.children?.[0]?.value as string;
        if (keys.has(key)) return undefined;
        keys.add(key);
      }
    }
    pending.push(...(node.children ?? []));
  }
  try { return JSON.parse(raw); } catch { return undefined; }
}

function serviceOrigin() {
  try {
    const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim();
    if (!raw) return null;
    const url = new URL(raw);
    const host = url.hostname.replace(/^\[|\]$/g, "");
    if (!["127.0.0.1", "::1", "localhost"].includes(host) || !["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash || !["/api/v1", "/api/v1/"].includes(url.pathname)) return null;
    return url.origin;
  } catch { return null; }
}

async function readServiceCredential() {
  try {
    const configured = process.env.LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE?.trim();
    if (!configured || !path.isAbsolute(configured)) return null;
    const stat = await lstat(configured);
    if (!stat.isFile() || stat.isSymbolicLink() || (process.platform !== "win32" && (stat.mode & 0o077) !== 0)) return null;
    if (process.platform === "win32" && !(await hasPrivateWindowsACL(configured))) return null;
    const value = (await readFile(configured, { encoding: "utf8" })).trim();
    return /^[a-fA-F0-9]{64}$/.test(value) ? value : null;
  } catch { return null; }
}

export async function isReferralRegistrationAvailable() {
  return Boolean(hasConfiguredPublicOrigin() && serviceOrigin() && await readServiceCredential());
}

function hasConfiguredPublicOrigin() {
  const configured =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim();
  if (!configured) return false;
  try {
    const origin = new URL(configured).origin;
    return hasTrustedSameOriginWrite(new Request(`${origin}/`, {
      method: "POST",
      headers: { Origin: origin, "Sec-Fetch-Site": "same-origin" },
    }));
  } catch {
    return false;
  }
}

async function hasPrivateWindowsACL(file: string) {
  const environment: NodeJS.ProcessEnv = { ...process.env, LISTINGKIT_REFERRAL_CREDENTIAL_PATH: file };
  for (const key of Object.keys(environment)) if (key.toLowerCase() === "psmodulepath") delete environment[key];
  try {
    await execFile("powershell.exe", [
      "-NoProfile", "-NonInteractive", "-Command",
      "$ErrorActionPreference='Stop';$p=$env:LISTINGKIT_REFERRAL_CREDENTIAL_PATH;$allowed=@([System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544');$acl=[System.IO.File]::GetAccessControl($p);foreach($ace in $acl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])){if($ace.AccessControlType -eq 'Allow' -and $allowed -notcontains $ace.IdentityReference.Value){exit 1}};exit 0",
    ], { env: environment, timeout: 3000, windowsHide: true });
    return true;
  } catch { return false; }
}

function upstreamFailure(status: number, payload: unknown) {
  const code = payload && typeof payload === "object" && !Array.isArray(payload) && typeof (payload as { error?: unknown }).error === "string" ? (payload as { error: string }).error : "";
  return safeErrors.has(code) ? failure(status, code) : failure(502, "INVALID_UPSTREAM_RESPONSE");
}

export function referralMethodNotAllowed() { return failure(405, "INVALID_REQUEST"); }
export function referralFailure(status: number, code: string) { return failure(status, code); }
export function referralJSON(value: unknown, status = 200) { return json(value, status); }

function failure(status: number, code: string) {
  return json({ code, message: "Referral request could not be completed", requestId: "", fieldErrors: [] }, status);
}
function json(value: unknown, status: number) { return NextResponse.json(value, { status, headers: responseHeaders }); }
