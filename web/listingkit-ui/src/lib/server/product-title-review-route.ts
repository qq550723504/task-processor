import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyProductTitleReview } from "./product-title-review-proxy";
import { productReviewFailure, withProductReviewDeadline } from "./product-title-review-deadline";

export function handleProductTitleReview(request: NextRequest): Promise<Response> {
  return withProductReviewDeadline(request, async (signal, state) => {
    const scoped = new NextRequest(request, { signal });
    const authenticated = serverAuth(async (sessionRequest) => {
      signal.throwIfAborted();
      return proxyProductTitleReview(scoped, readZitadelServerAccessToken(sessionRequest.auth), state);
    });
    const result = await authenticated(scoped, { params: Promise.resolve({}) });
    return result ?? productReviewFailure(503, "DEPENDENCY_UNAVAILABLE");
  });
}
export const rejectProductTitleReviewMethod = () => productReviewFailure(405, "INVALID_REQUEST");
