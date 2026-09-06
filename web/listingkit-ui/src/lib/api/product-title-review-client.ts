import { PRODUCT_REVIEW_RESPONSE_BYTES, parseProductTitleProposal, parseProductTitleProposalList, parseProductTitleReviewFailure, productTitleApplySchema, productTitleCursorSchema, productTitleDecisionSchema, productTitleIDSchema, productTitleIdempotencyKeySchema, productTitleOrganizationSchema, type ProductTitleApplyInput, type ProductTitleDecisionInput, type ProductTitleProposal, type ProductTitleProposalList, type ProductTitleReviewFailure } from "./product-title-review";
import { readProductReviewJSON } from "./product-title-review-json";

type Scope = { organizationId: string; signal?: AbortSignal };
type Detail = Scope & { proposalId: string };
type Write<T> = Detail & { idempotencyKey: string; input: T };
type Outcome = "not_sent" | "rejected" | "unknown";
export class ProductTitleReviewError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly payload: ProductTitleReviewFailure, public readonly outcome: Outcome) {
    super(outcome === "unknown" ? "操作结果待核实，请显式核实本次操作" : `Product title review request failed (${code})`);
    this.name = "ProductTitleReviewError";
  }
}
const error = (status: number, code: string, outcome: "not_sent" | "unknown") => new ProductTitleReviewError(status, code, { code, message: "Product title review request could not complete", requestId: "", fieldErrors: [], outcome }, outcome);
const invalid = () => error(400, "INVALID_REQUEST", "not_sent");
const base = "/api/product/text-proposals";
function validateScope(input: Scope) { if (!productTitleOrganizationSchema.safeParse(input.organizationId).success) throw invalid(); }
function validateDetail(input: Detail) { validateScope(input); if (!productTitleIDSchema.safeParse(input.proposalId).success) throw invalid(); }

async function request<T>(path: string, input: Scope, parse: (value: unknown) => T | null, write?: { key: string; body: string }): Promise<T> {
  if (input.signal?.aborted) throw error(504, "DEADLINE_EXCEEDED", "not_sent");
  let dispatched = false;
  try {
    dispatched = true;
    const response = await fetch(path, {
      method: write ? "POST" : "GET", credentials: "same-origin", cache: "no-store", redirect: "error", signal: input.signal,
      headers: { Accept: "application/json", "X-Expected-Organization-ID": input.organizationId,
        ...(write ? { "Content-Type": "application/json", "Idempotency-Key": write.key } : {}) },
      ...(write ? { body: write.body } : {}),
    });
    const payload = await readProductReviewJSON(response, PRODUCT_REVIEW_RESPONSE_BYTES, input.signal);
    if (response.status === 200) {
      const parsed = parse(payload); if (parsed) return parsed;
    } else {
      const failure = parseProductTitleReviewFailure(payload, response.status);
      if (failure) {
        const outcome = "outcome" in failure ? failure.outcome : write && response.status >= 500 ? "unknown" : "rejected";
        if (outcome === "unknown") throw error(502, "RESULT_UNVERIFIED", outcome);
        throw new ProductTitleReviewError(response.status, "error" in failure ? failure.error : failure.code, failure, outcome);
      }
    }
    throw error(502, write ? "RESULT_UNVERIFIED" : "INVALID_UPSTREAM_RESPONSE", write ? "unknown" : "not_sent");
  } catch (e) {
    if (e instanceof ProductTitleReviewError) throw e;
    throw error(input.signal?.aborted ? 504 : 502, write && dispatched ? "RESULT_UNVERIFIED" : "DEPENDENCY_UNAVAILABLE", write && dispatched ? "unknown" : "not_sent");
  }
}
const detailParser = (id: string) => (value: unknown) => { const p = parseProductTitleProposal(value); return p?.proposal_id === id ? p : null; };
export async function fetchProductTitleProposals(input: Scope & { limit?: number; cursor?: string }): Promise<ProductTitleProposalList> {
  validateScope(input); const limit = input.limit ?? 20;
  if (!Number.isInteger(limit) || limit < 1 || limit > 100 || (input.cursor !== undefined && !productTitleCursorSchema.safeParse(input.cursor).success)) throw invalid();
  const query = new URLSearchParams({ view: "actionable", limit: String(limit) });
  if (input.cursor !== undefined) query.set("cursor", input.cursor);
  return request(`${base}?${query}`, input, parseProductTitleProposalList);
}
export async function fetchProductTitleProposal(input: Detail): Promise<ProductTitleProposal> {
  validateDetail(input); return request(`${base}/${input.proposalId}`, input, detailParser(input.proposalId));
}
export async function decideProductTitleProposal(input: Write<ProductTitleDecisionInput>): Promise<ProductTitleProposal> {
  validateDetail(input);
  const parsed = productTitleDecisionSchema.safeParse(input.input);
  if (!parsed.success || !productTitleIdempotencyKeySchema.safeParse(input.idempotencyKey).success) throw invalid();
  return request(`${base}/${input.proposalId}/decisions`, input, detailParser(input.proposalId), { key: input.idempotencyKey, body: JSON.stringify(parsed.data) });
}
export async function applyProductTitleProposal(input: Write<ProductTitleApplyInput>): Promise<ProductTitleProposal> {
  validateDetail(input);
  const parsed = productTitleApplySchema.safeParse(input.input);
  if (!parsed.success || !productTitleIdempotencyKeySchema.safeParse(input.idempotencyKey).success) throw invalid();
  return request(`${base}/${input.proposalId}/apply`, input, detailParser(input.proposalId), { key: input.idempotencyKey, body: JSON.stringify(parsed.data) });
}
