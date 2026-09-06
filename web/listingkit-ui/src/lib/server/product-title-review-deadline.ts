import { NextResponse } from "next/server";
import { newRequestLogId } from "./request-log";

export type ProductReviewDispatch = { forwarded: boolean };
export function productReviewJSON(payload: unknown, status = 200): NextResponse {
  return NextResponse.json(payload, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
}
export function productReviewFailure(status: number, code: string, outcome: "not_sent" | "unknown" = "not_sent"): NextResponse {
  return productReviewJSON({ code, message: outcome === "unknown" ? "操作结果待核实，请显式核实本次操作" : "Product title review request could not complete", requestId: newRequestLogId(), fieldErrors: [], outcome }, status);
}

/** One budget, including Auth.js, request body, Go HTTP and response parsing. */
export async function withProductReviewDeadline(request: Request, execute: (signal: AbortSignal, state: ProductReviewDispatch) => Promise<Response>): Promise<Response> {
  const state: ProductReviewDispatch = { forwarded: false };
  const controller = new AbortController();
  const failure = (deadline: boolean) => request.method === "POST" && state.forwarded
    ? productReviewFailure(deadline ? 504 : 502, "RESULT_UNVERIFIED", "unknown")
    : productReviewFailure(deadline ? 504 : 503, deadline ? "DEADLINE_EXCEEDED" : "DEPENDENCY_UNAVAILABLE");
  if (request.signal.aborted) return failure(true);
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15000);
  let onEnd!: () => void;
  const ended = new Promise<Response>((resolve) => {
    onEnd = () => resolve(failure(true));
    controller.signal.addEventListener("abort", onEnd, { once: true });
  });
  try {
    return await Promise.race([execute(controller.signal, state), ended]);
  } catch { return failure(controller.signal.aborted); }
  finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", onEnd);
  }
}
