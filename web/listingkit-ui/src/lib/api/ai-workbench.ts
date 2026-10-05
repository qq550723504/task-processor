import { aiResponseSchemas, parseAIWorkbenchResponse, type AIWorkbenchRoute } from "@/lib/contracts/ai-workbench";
import { readBoundedStrictJSON } from "./strict-json-response";

export class AIWorkbenchError extends Error {
  constructor(public code: string) { super(code); }
}
export type AIScope = { userId: string; organizationId: string };

type AIRequest = {
  route: AIWorkbenchRoute; method: "GET" | "POST" | "PATCH"; path: string; scope: AIScope;
  signal?: AbortSignal; body?: object; key?: string; revision?: number;
};

export async function requestAIWorkbench<R extends AIWorkbenchRoute>(input: AIRequest & { route: R }): Promise<ReturnType<(typeof aiResponseSchemas)[R]["parse"]>> {
  const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": input.scope.userId,
    "X-Expected-Organization-ID": input.scope.organizationId });
  if (input.body) headers.set("Content-Type", "application/json");
  if (input.key) headers.set("Idempotency-Key", input.key);
  if (input.revision) headers.set("If-Match", String(input.revision));
  let response: Response;
  try {
    response = await fetch(`/api/workbench/${input.path}`, { method: input.method, headers,
      body: input.body ? JSON.stringify(input.body) : undefined, credentials: "same-origin", redirect: "error", cache: "no-store", signal: input.signal });
  } catch {
    throw new AIWorkbenchError(input.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN");
  }
  let payload: unknown;
  try { payload = await readBoundedStrictJSON(response, 256 * 1024, input.signal); }
  catch { throw new AIWorkbenchError(input.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN"); }
  if (!response.ok) {
    const code = payload && typeof payload === "object" && "code" in payload && typeof payload.code === "string" ? payload.code : "DEPENDENCY_UNAVAILABLE";
    throw new AIWorkbenchError(code);
  }
  const checked = parseAIWorkbenchResponse(input.route, response.status, payload);
  if (!checked) throw new AIWorkbenchError(input.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN");
  return checked as ReturnType<(typeof aiResponseSchemas)[R]["parse"]>;
}
