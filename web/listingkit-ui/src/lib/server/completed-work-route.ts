import { NextRequest } from "next/server";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { parseSheinRecordList } from "@/lib/api/shein-records";
import { projectCompletedWork } from "@/lib/api/completed-work";
import { handleSheinRecordsGET } from "./shein-records-route";
import { workbenchProtocolError } from "./workbench-proxy";

const MAX_BYTES = 128 * 1024;
const invalid = () => workbenchProtocolError(400, "INVALID_REQUEST", "Only local preparation source and collection pagination are supported");

export async function projectCompletedWorkResponse(response: Response, signal: AbortSignal): Promise<Response> {
  // Preserve error status, request ID and revocation Set-Cookie exactly.
  if (response.status !== 200) return response;
  try {
    signal.throwIfAborted();
    const source = parseSheinRecordList(await readBoundedStrictJSON(response, MAX_BYTES, signal));
    if (!source) throw new Error("Invalid collection");
    const body = JSON.stringify(projectCompletedWork(source));
    if (new TextEncoder().encode(body).length > MAX_BYTES) throw new Error("Projection too large");
    signal.throwIfAborted();
    return new Response(body, { status: 200, headers: response.headers });
  } catch {
    return signal.aborted
      ? workbenchProtocolError(504, "DEADLINE_EXCEEDED", "Completed work request ended before completion")
      : workbenchProtocolError(502, "INVALID_UPSTREAM_RESPONSE", "Completed work source response is invalid");
  }
}

export async function completedWorkGET(request: NextRequest): Promise<Response> {
  if (request.signal.aborted) return workbenchProtocolError(504, "DEADLINE_EXCEEDED", "Completed work request ended before completion");
  const incoming = new URL(request.url);
  const raw = incoming.search.slice(1);
  if (incoming.pathname !== "/api/workbench/completed-work" || request.method !== "GET" || request.body !== null ||
      (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding") ||
      new TextEncoder().encode(raw).length > 1024 || raw.includes(";")) {
    void request.body?.cancel().catch(() => undefined); return invalid();
  }
  try { decodeURIComponent(raw); } catch { return invalid(); }
  if (incoming.searchParams.getAll("source").length !== 1 || incoming.searchParams.get("source") !== "listing-local-preparation") return invalid();
  // Keep raw pagination bytes and all unknown/duplicate parameters for the one
  // collection validator. Re-encoding here could silently repair invalid input.
  const query = raw.split("&").filter((part) => decodeURIComponent(part.split("=", 1)[0].replace(/\+/g, " ")) !== "source").join("&");
  incoming.pathname = "/api/listing/shein-records";
  incoming.search = query;
  const scoped = new NextRequest(incoming, { headers: request.headers, signal: request.signal });
  return handleSheinRecordsGET(scoped, projectCompletedWorkResponse);
}
