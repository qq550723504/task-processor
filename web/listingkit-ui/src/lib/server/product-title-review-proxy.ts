import { PRODUCT_REVIEW_RESPONSE_BYTES, parseProductTitleProposal, parseProductTitleProposalList, parseProductTitleReviewFailure, productTitleApplySchema, productTitleCursorSchema, productTitleDecisionSchema, productTitleIDSchema, productTitleIdempotencyKeySchema } from "@/lib/api/product-title-review";
import { readProductReviewJSON } from "@/lib/api/product-title-review-json";
import { configuredProductReviewOrigin, hasTrustedReviewWriteOrigin, reviewSelectedOrganization } from "./product-title-review-request";
import { productReviewFailure, productReviewJSON, type ProductReviewDispatch } from "./product-title-review-deadline";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const base = "/api/product/text-proposals";
class InputTooLarge extends Error {}
function reject(request: Request, status = 400, code = "INVALID_REQUEST") {
  void request.body?.cancel().catch(() => undefined);
  return productReviewFailure(status, code);
}
async function writeBody(request: Request, apply: boolean): Promise<string | null> {
  let size = 0;
  const stream = request.body?.pipeThrough(new TransformStream<Uint8Array, Uint8Array>({ transform(chunk, controller) {
    size += chunk.byteLength; if (size > 32768) throw new InputTooLarge(); controller.enqueue(chunk);
  } }));
  if (!stream) return null;
  const payload = await readProductReviewJSON(new Response(stream, { headers: { "Content-Type": "application/json" } }), 32768, request.signal);
  if (apply) {
    const result = productTitleApplySchema.safeParse(payload);
    return result.success ? `{"expected_revision":${result.data.expected_revision}}` : null;
  }
  const result = productTitleDecisionSchema.safeParse(payload);
  if (!result.success) return null;
  const d = result.data;
  return `{"action":${JSON.stringify(d.action)},"expected_revision":${d.expected_revision}${d.action === "edit" ? `,"title":${JSON.stringify(d.title)}` : ""}}`;
}

/** Only called by the authenticated route with the shared total-deadline signal. */
export async function proxyProductTitleReview(request: Request, accessToken: string, state: ProductReviewDispatch): Promise<Response> {
  const write = request.method === "POST";
  if (request.method !== "GET" && !write) return reject(request, 405);
  if (!accessToken) return reject(request, 401, "AUTHENTICATION_REQUIRED");
  const org = reviewSelectedOrganization(request);
  if (!org || org !== request.headers.get("X-Expected-Organization-ID")) return reject(request, 409, "ORGANIZATION_CONTEXT_CHANGED");
  const incoming = new URL(request.url);
  const collection = incoming.pathname === base;
  const match = incoming.pathname.match(/^\/api\/product\/text-proposals\/([^/]+)(?:\/(decisions|apply))?$/);
  const id = match?.[1]; const action = match?.[2];
  if (!collection && (!match || !productTitleIDSchema.safeParse(id).success)) return reject(request);
  if (write ? collection || !action : !!action) return reject(request, 405);
  if (!collection && incoming.search) return reject(request);
  if (!write && (request.body !== null || request.headers.has("transfer-encoding") || (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))) return reject(request);
  const query = new URLSearchParams();
  if (collection) {
    const raw = incoming.search.slice(1);
    if (new TextEncoder().encode(raw).length > 1024 || raw.includes(";")) return reject(request);
    try { decodeURIComponent(raw); } catch { return reject(request); }
    for (const key of incoming.searchParams.keys()) if (!["view", "limit", "cursor"].includes(key) || incoming.searchParams.getAll(key).length !== 1) return reject(request);
    const limit = incoming.searchParams.get("limit") ?? "20";
    const cursor = incoming.searchParams.get("cursor");
    if (incoming.searchParams.get("view") !== "actionable" || !/^[1-9][0-9]{0,2}$/.test(limit) || Number(limit) > 100 || (cursor !== null && !productTitleCursorSchema.safeParse(cursor).success)) return reject(request);
    query.set("view", "actionable"); query.set("limit", limit); if (cursor !== null) query.set("cursor", cursor);
  }
  const origin = configuredProductReviewOrigin();
  if (!origin) return reject(request, 503, "DEPENDENCY_UNAVAILABLE");
  const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${accessToken}`, "X-Requested-Organization-ID": org });
  let body: string | undefined;
  if (write) {
    if (!hasTrustedReviewWriteOrigin(request)) return reject(request, 403, "PERMISSION_DENIED");
    if (!/^application\/json(?:;\s*charset=utf-8)?$/i.test(request.headers.get("content-type") ?? "")) return reject(request);
    const key = request.headers.get("idempotency-key");
    // Headers combines duplicates with comma; the browser contract excludes comma
    // so two keys cannot be mistaken for one operation identity.
    if (!productTitleIdempotencyKeySchema.safeParse(key).success || key!.includes(",")) return reject(request);
    try { const encoded = await writeBody(request, action === "apply"); if (encoded === null) return reject(request); body = encoded; }
    catch (e) {
      if (request.signal.aborted) return reject(request, 504, "DEADLINE_EXCEEDED");
      return reject(request, e instanceof InputTooLarge ? 413 : 400);
    }
    headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", key!);
  }
  const url = new URL(incoming.pathname, origin); url.search = query.toString();
  const invalidResponse = () => write ? productReviewFailure(502, "RESULT_UNVERIFIED", "unknown") : productReviewFailure(502, "INVALID_UPSTREAM_RESPONSE");
  try {
    request.signal.throwIfAborted(); state.forwarded = true;
    const upstream = await fetch(url, { method: request.method, headers, ...(body !== undefined ? { body } : {}), cache: "no-store", redirect: "manual", signal: request.signal });
    const payload = await readProductReviewJSON(upstream, PRODUCT_REVIEW_RESPONSE_BYTES, request.signal);
    if (upstream.status === 200) {
      const result = collection ? parseProductTitleProposalList(payload) : parseProductTitleProposal(payload);
      if (!result || (!collection && (!("proposal_id" in result) || result.proposal_id !== id))) return invalidResponse();
      return productReviewJSON(result);
    }
    const failure = parseProductTitleReviewFailure(payload, upstream.status);
    if (!failure || "outcome" in failure) return invalidResponse();
    if (write && upstream.status >= 500) return productReviewFailure(upstream.status === 504 ? 504 : 502, "RESULT_UNVERIFIED", "unknown");
    const response = productReviewJSON(failure, upstream.status);
    if ("code" in failure && ["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(failure.code)) response.cookies.set(WORKBENCH_COOKIE_NAME, "", { httpOnly: true, sameSite: "lax", path: "/", secure: process.env.NODE_ENV !== "development", maxAge: 0 });
    return response;
  } catch {
    if (write && state.forwarded) return productReviewFailure(request.signal.aborted ? 504 : 502, "RESULT_UNVERIFIED", "unknown");
    return productReviewFailure(request.signal.aborted ? 504 : 502, request.signal.aborted ? "DEADLINE_EXCEEDED" : "DEPENDENCY_UNAVAILABLE");
  }
}
