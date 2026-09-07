import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyProductTitleReview } from "./product-title-review-proxy";
import { productReviewFailure, withProductReviewDeadline } from "./product-title-review-deadline";

export function handleProductTitleReview(request: NextRequest): Promise<Response> {
  return withProductReviewDeadline(request, async (signal, state) => {
    // Next can proxy the incoming Request; cloning its native private slots is
    // invalid on current Node. Preserve the actual request fields explicitly.
    const scoped = new NextRequest(request.url, { method: request.method, headers: request.headers, body: request.body, signal });
    const authenticated = serverAuth(async (sessionRequest) => {
      signal.throwIfAborted();
      return proxyProductTitleReview(scoped, readZitadelServerAccessToken(sessionRequest.auth), state);
    });
    const result = await authenticated(scoped, { params: Promise.resolve({}) });
    return result ?? productReviewFailure(503, "DEPENDENCY_UNAVAILABLE");
  });
}
export const rejectProductTitleReviewMethod = () => productReviewFailure(405, "INVALID_REQUEST");
