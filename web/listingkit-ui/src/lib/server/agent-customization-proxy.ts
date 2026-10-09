import { z } from "zod";
import { customDetail, customId, customInput, customPage, customReceipt, customUpdate, customVersion } from "@/lib/api/agent-customization";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { hasEmptyBody } from "./members-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const safe = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
export const customizationFailure = (status: number, code: string) => Response.json({ code }, { status, headers: safe });
export function customizationEndpoint(url: URL, method: string) {
    const admin = url.pathname.startsWith("/api/workbench/admin/agent-customization/requests");
    const prefix = `/api/workbench/${admin ? "admin/" : ""}agent-customization/requests`;
    if (!url.pathname.startsWith(prefix))
        return null;
    const suffix = url.pathname.slice(prefix.length);
    const p = suffix === "" ? [] : suffix.startsWith("/") ? suffix.slice(1).split("/") : null;
    if (!p)
        return null;
    let output: z.ZodType = customPage, input: z.ZodType | undefined;
    let download = false, cas = false;
    if (p.length === 0 && method === "GET")
        output = customPage;
    else if (p.length === 0 && method === "POST" && !admin) {
        input = customInput;
        output = customReceipt;
    }
    else if (p.length === 1 && customId.safeParse(p[0]).success && method === "GET")
        output = customDetail;
    else if (p.length === 2 && customId.safeParse(p[0]).success && p[1] === "progress" && method === "POST" && admin) {
        input = customUpdate;
        output = customReceipt;
        cas = true;
    }
    else if (p.length === 3 && customId.safeParse(p[0]).success && p[1] === "files" && customId.safeParse(p[2]).success && method === "GET")
        download = true;
    else
        return null;
    if (url.search.length > 256)
        return null;
    for (const [k, v] of url.searchParams) {
        if (url.searchParams.getAll(k).length !== 1 || !(method === "GET" && (p.length === 0 && k === "cursor" && customId.safeParse(v).success || p.length === 1 && k === "after" && customVersion.safeParse(v).success)))
            return null;
    }
    return { admin, path: suffix, output, input, download, cas };
}
async function boundedBytes(response: Response, signal: AbortSignal) { const reader = response.body?.getReader(); if (!reader)
    throw new Error(); let size = 0; const chunks: Uint8Array[] = []; const cancel = () => { void reader.cancel().catch(() => undefined); }; signal.addEventListener("abort", cancel, { once: true }); try {
    while (true) {
        signal.throwIfAborted();
        const v = await reader.read();
        signal.throwIfAborted();
        if (v.done)
            break;
        size += v.value.length;
        if (size > 2 * 1024 * 1024)
            throw new Error();
        chunks.push(v.value);
    }
    const result = new Uint8Array(size);
    let at = 0;
    for (const v of chunks) {
        result.set(v, at);
        at += v.length;
    }
    return result;
}
catch (e) {
    cancel();
    throw e;
}
finally {
    signal.removeEventListener("abort", cancel);
    reader.releaseLock();
} }
export async function proxyAgentCustomization(request: Request, token: string, userId: string): Promise<Response> {
    if (!token || !userId)
        return customizationFailure(401, "AUTHENTICATION_REQUIRED");
    if (request.headers.get("X-Expected-User-ID") !== userId)
        return customizationFailure(409, "IDENTITY_CONTEXT_CHANGED");
    const url = new URL(request.url), route = customizationEndpoint(url, request.method);
    if (!route || request.url.endsWith("?") || request.headers.has("content-encoding"))
        return customizationFailure(400, "CUSTOMIZATION_INVALID");
    const write = !!route.input;
    if (write && !hasTrustedSameOriginWrite(request))
        return customizationFailure(403, "CUSTOMIZATION_FORBIDDEN");
    let org = "";
    if (!route.admin) {
        const cookies = (request.headers.get("cookie") ?? "").split(";").map(v => v.trim()).filter(v => v.startsWith(WORKBENCH_COOKIE_NAME + "="));
        try {
            if (cookies.length === 1)
                org = decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1));
        }
        catch { }
        if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org) || org !== request.headers.get("X-Expected-Organization-ID"))
            return customizationFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    }
    let origin: string;
    try {
        const u = new URL(process.env.LISTINGKIT_SERVICE_API_BASE ?? "");
        if (!["http:", "https:"].includes(u.protocol) || u.username || u.password || u.search || u.hash || !["/api/v1", "/api/v1/"].includes(u.pathname))
            throw new Error();
        origin = u.origin;
    }
    catch {
        return customizationFailure(503, "CUSTOMIZATION_UNAVAILABLE");
    }
    const controller = new AbortController(), abort = () => controller.abort();
    request.signal.addEventListener("abort", abort, { once: true });
    if (request.signal.aborted)
        abort();
    const timer = setTimeout(abort, 35000);
    let dispatched = false;
    try {
        const headers = new Headers({ Authorization: "Bearer " + token, Accept: route.download ? "application/octet-stream" : "application/json" });
        if (!route.admin)
            headers.set("X-Requested-Organization-ID", org);
        let body: string | undefined;
        if (write) {
            const key = request.headers.get("Idempotency-Key");
            if (!customId.safeParse(key).success || request.headers.has("If-None-Match"))
                return customizationFailure(400, "CUSTOMIZATION_INVALID");
            headers.set("Idempotency-Key", key!);
            if (route.cas) {
                const match = request.headers.get("If-Match");
                if (!match || !match.startsWith('"') || !match.endsWith('"') || !customVersion.safeParse(match.slice(1, -1)).success)
                    return customizationFailure(400, "CUSTOMIZATION_INVALID");
                headers.set("If-Match", match);
            }
            else if (request.headers.has("If-Match"))
                return customizationFailure(400, "CUSTOMIZATION_INVALID");
            const payload = await readBoundedStrictJSON(new Response(request.body, { headers: { "Content-Type": request.headers.get("Content-Type") ?? "" } }), route.cas ? 64 * 1024 : 9 * 1024 * 1024, controller.signal);
            const value = route.input!.safeParse(payload);
            if (!value.success)
                return customizationFailure(400, "CUSTOMIZATION_INVALID");
            body = JSON.stringify(value.data);
            headers.set("Content-Type", "application/json");
        }
        else if (!(await hasEmptyBody(request, controller.signal)))
            return customizationFailure(400, "CUSTOMIZATION_INVALID");
        controller.signal.throwIfAborted();
        dispatched = true;
        const response = await fetch(origin + `/api/v1/${route.admin ? "admin/" : ""}agent-customization/requests` + route.path + url.search, { method: request.method, headers, body, signal: controller.signal, cache: "no-store", redirect: "manual" });
        if (route.download && response.status === 200) {
            const mime = response.headers.get("Content-Type") ?? "";
            if (!["application/pdf", "image/png", "image/jpeg", "text/plain; charset=utf-8"].includes(mime))
                throw new Error();
            const bytes = await boundedBytes(response, controller.signal);
            return new Response(bytes.buffer as ArrayBuffer, { headers: { ...safe, "Content-Type": mime, "Content-Disposition": "attachment" } });
        }
        // A 20-row legal page can exceed 2 MiB after Go's HTML escaping.
        const raw = await readBoundedStrictJSON(response, 4 * 1024 * 1024, controller.signal);
        controller.signal.throwIfAborted();
        if (response.status !== 200) {
            const error = z.object({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).safeParse(raw);
            return customizationFailure(response.status >= 400 && response.status < 600 ? response.status : 502, write && (response.status >= 500 || !error.success) ? "OUTCOME_UNKNOWN" : error.success ? error.data.code : "CUSTOMIZATION_UNAVAILABLE");
        }
        const parsed = route.output.safeParse(raw);
        if (!parsed.success)
            throw new Error();
        if (route.output === customReceipt) {
            const receipt = customReceipt.parse(parsed.data);
            if (receipt.key !== request.headers.get("Idempotency-Key") || route.cas && receipt.requestId !== route.path.split("/")[1])
                throw new Error();
        }
        if (route.output === customDetail) {
            const detail = customDetail.parse(parsed.data);
            if (detail.request.id !== route.path.slice(1) || !route.admin && detail.request.organizationId !== org)
                throw new Error();
        }
        if (route.output === customPage && !route.admin && customPage.parse(parsed.data).items.some(v => v.organizationId !== org))
            throw new Error();
        return Response.json(parsed.data, { headers: safe });
    }
    catch {
        return customizationFailure(controller.signal.aborted ? 504 : dispatched ? 502 : 400, write && dispatched ? "OUTCOME_UNKNOWN" : dispatched ? "CUSTOMIZATION_UNAVAILABLE" : "CUSTOMIZATION_INVALID");
    }
    finally {
        clearTimeout(timer);
        request.signal.removeEventListener("abort", abort);
    }
}
