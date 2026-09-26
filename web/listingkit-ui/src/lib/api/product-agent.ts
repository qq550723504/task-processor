import { agentResultSchema, agentReviewLinkSchema } from "../contracts/product-agent";
import { isAcquisitionUUID } from "../contracts/product-acquisition";
import type { AcquisitionContext } from "./product-acquisition";
import { readBoundedStrictJSON } from "./strict-json-response";
export class ProductAgentError extends Error {
    constructor(public code: string) { super(code); }
}
export async function requestProductAgent(action: "start" | "read" | "resume" | "review", operationId: string, key: string, scope: AcquisitionContext, signal: AbortSignal, revision?: string, feedback?: string) {
    if (!isAcquisitionUUID(operationId) || !isAcquisitionUUID(key))
        throw new ProductAgentError("INVALID_AGENT_REQUEST");
    const base = `/api/workbench/sourcing/1688/acquisitions/${operationId}/product-agent/runs`;
    const path = action === "start" ? base : `${base}/${key}${action === "read" ? "" : `/${action}`}`;
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal.addEventListener("abort", abort, { once: true });
    if (signal.aborted)
        abort();
    const timeout = setTimeout(abort, 130000);
    try {
        const headers = new Headers({ "X-Expected-User-ID": scope.userId, "X-Expected-Organization-ID": scope.organizationId, Accept: "application/json" });
        if (action !== "read") {
            headers.set("Content-Type", "application/json");
            if (action === "start")
                headers.set("Idempotency-Key", key);
        }
        const response = await fetch(path, { method: action === "read" ? "GET" : "POST", headers, body: action === "read" ? undefined : JSON.stringify(action === "resume" ? { revision, feedback } : {}), credentials: "same-origin", redirect: "error", cache: "no-store", signal: controller.signal });
        const payload = await readBoundedStrictJSON(response, 128 * 1024, controller.signal);
        if (!response.ok) {
            const code = payload && typeof payload === "object" && "code" in payload && typeof payload.code === "string" ? payload.code : "OUTCOME_UNKNOWN";
            throw new ProductAgentError(code);
        }
        const result = (action === "review" ? agentReviewLinkSchema : agentResultSchema).safeParse(payload);
        if (response.status !== 200 || !result.success || result.data.operationId !== operationId || result.data.requestKey !== key)
            throw new ProductAgentError("OUTCOME_UNKNOWN");
        return result.data;
    }
    catch (error) {
        if (error instanceof ProductAgentError)
            throw error;
        throw new ProductAgentError(action === "read" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN");
    }
    finally {
        clearTimeout(timeout);
        signal.removeEventListener("abort", abort);
    }
}
