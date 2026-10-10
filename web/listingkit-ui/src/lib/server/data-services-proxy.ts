import { dataRoute, DATA_MAX_BYTES } from "@/lib/contracts/data-services";
import { hasTrustedSameOriginWrite, hasTrustedSameOriginRecovery } from "@/lib/server/same-origin-write";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { parseTree, type Node, type ParseError } from "jsonc-parser";
const bounded = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
export function dataFailure(code: string, status: number) { return Response.json({ error: { code, message: code } }, { status, headers: { "Cache-Control": "no-store" } }); }
async function bodyWithinLimit(request: Request, limit: number) {
    const reader = request.body?.getReader();
    if (!reader)
        throw Error();
    const cancel = () => { void reader.cancel().catch(() => { }); };
    request.signal.addEventListener("abort", cancel, { once: true });
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
        for (;;) {
            request.signal.throwIfAborted();
            const { value, done } = await reader.read();
            request.signal.throwIfAborted();
            if (done)
                break;
            size += value.byteLength;
            if (size > limit)
                throw Error();
            chunks.push(value);
        }
        const raw = new Uint8Array(size);
        let offset = 0;
        for (const v of chunks) {
            raw.set(v, offset);
            offset += v.byteLength;
        }
        return raw;
    }
    catch (e) {
        cancel();
        throw e;
    }
    finally {
        request.signal.removeEventListener("abort", cancel);
        reader.releaseLock();
    }
}
function strictJSON(raw: Uint8Array) {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(raw);
    const errors: ParseError[] = [];
    const root = parseTree(text, errors, { disallowComments: true, allowTrailingComma: false });
    if (!root || errors.length)
        throw Error();
    const walk = (n: Node, d: number) => {
        if (d > 24)
            throw Error();
        if (n.type === "object") {
            const keys = new Set();
            for (const p of n.children ?? []) {
                const key = p.children?.[0]?.value;
                if (keys.has(key))
                    throw Error();
                keys.add(key);
            }
        }
        for (const child of n.children ?? [])
            walk(child, d + 1);
    };
    walk(root, 0);
    return JSON.parse(text);
}
export async function buildDataRequest(request: Request, path: string[], token: string, actor: string, specialist = false) {
    const route = dataRoute(request.method, path, specialist);
    if (!route)
        return dataFailure("INVALID_DATA_REQUEST", 404);
    const actual = new URL(request.url);
    const browserBase = specialist ? "/api/platform/data-customization" : "/api/workbench/data-services";
    if (actual.pathname !== browserBase + (path.length ? `/${path.join("/")}` : "") || !token || !bounded.test(actor))
        return dataFailure("AUTHENTICATION_REQUIRED", 401);
    const mutation = request.method === "POST";
    if (mutation ? !hasTrustedSameOriginWrite(request) : !hasTrustedSameOriginRecovery(request))
        return dataFailure("FORBIDDEN", 403);
    if (request.headers.get("x-expected-user-id") !== actor)
        return dataFailure("IDENTITY_CONTEXT_CHANGED", 409);
    const headers = new Headers({ "Authorization": `Bearer ${token}`, "Accept": "application/json" });
    if (!specialist) {
        const expected = request.headers.get("x-expected-organization-id");
        const cookies = (request.headers.get("cookie") ?? "").split(";").map(v => v.trim()).filter(v => v.startsWith("shuomi_effective_organization="));
        let selected = "";
        try {
            if (cookies.length === 1)
                selected = decodeURIComponent(cookies[0]!.split("=").slice(1).join("="));
        }
        catch { }
        if (!expected || !bounded.test(expected) || selected !== expected)
            return dataFailure("ORGANIZATION_CONTEXT_CHANGED", 409);
        headers.set("X-Requested-Organization-ID", expected);
    }
    const allowed = route.action === "admin-list" ? ["cursor", "limit", "state"] : ["key-history", "job-results"].includes(route.action) ? ["cursor", "limit"] : route.action === "jobs" ? ["limit"] : [];
    if (actual.search.length > 2048)
        return dataFailure("INVALID_DATA_REQUEST", 400);
    for (const key of actual.searchParams.keys()) {
        if (!allowed.includes(key) || actual.searchParams.getAll(key).length !== 1)
            return dataFailure("INVALID_DATA_REQUEST", 400);
    }
    if (request.headers.has("content-encoding"))
        return dataFailure("INVALID_DATA_REQUEST", 400);
    let body: Uint8Array | undefined;
    if (mutation) {
        const command = request.headers.get("idempotency-key");
        if (!command || !/^[0-9a-f-]{36}$/i.test(command) || command.includes(","))
            return dataFailure("INVALID_DATA_REQUEST", 400);
        headers.set("Idempotency-Key", command);
        try {
            body = await bodyWithinLimit(request, route.binary ? DATA_MAX_BYTES : 64 * 1024);
            if (!body.byteLength)
                throw Error();
            if (route.binary) {
                if (request.headers.get("content-type") !== "application/octet-stream")
                    throw Error();
                for (const name of ["x-expected-revision", "x-spec-revision", "x-data-format"]) {
                    const v = request.headers.get(name);
                    if (!v || v.includes(",") || !(name === "x-data-format" ? /^(csv|json|xlsx)$/.test(v) : /^[1-9]\d{0,14}$/.test(v)))
                        throw Error();
                    headers.set(name, v);
                }
                headers.set("Content-Type", "application/octet-stream");
            }
            else {
                if (!/^application\/json(?:\s*;|$)/i.test(request.headers.get("content-type") ?? ""))
                    throw Error();
                const parsed = route.request!.parse(strictJSON(body));
                body = new TextEncoder().encode(JSON.stringify(parsed));
                headers.set("Content-Type", "application/json");
            }
        }
        catch {
            return dataFailure("INVALID_DATA_REQUEST", 400);
        }
    }
    else if (request.body || request.headers.has("idempotency-key"))
        return dataFailure("INVALID_DATA_REQUEST", 400);
    const configured = process.env.LISTINGKIT_SERVICE_API_BASE?.trim() || "http://localhost:8085/api/v1";
    let target: URL;
    try {
        const base = new URL(configured);
        if (!["http:", "https:"].includes(base.protocol) || base.username || base.password || base.search || base.hash || !/^\/api\/v1\/?$/.test(base.pathname))
            throw Error();
        target = new URL(`${specialist ? "platform/data-customization" : "workbench/data-services"}${path.length ? `/${path.join("/")}` : ""}${actual.search}`, `${base.origin}/api/v1/`);
    }
    catch {
        return dataFailure("DATA_UNAVAILABLE", 503);
    }
    return { url: target, route, mutation, init: { method: request.method, headers, body: body as BodyInit | undefined, cache: "no-store" as const } };
}
export async function dataBrowserResponse(response: Response, route: NonNullable<ReturnType<typeof dataRoute>>, mutation: boolean, signal?: AbortSignal) {
    try {
        const raw = await readBoundedStrictJSON(response, DATA_MAX_BYTES, signal);
        if (!response.ok) {
            const value = raw as {
                error?: {
                    code?: string;
                };
            };
            const code = value?.error?.code;
            const statuses: Record<string, number> = { INVALID_DATA_REQUEST: 400, FORBIDDEN: 403, DATA_NOT_FOUND: 404, DATA_CONFLICT: 409, DATA_UNKNOWN: 503, DATA_UNAVAILABLE: 503 };
            if (!code || statuses[code] !== response.status)
                throw Error();
            return dataFailure(code, response.status);
        }
        const successStatus = mutation && route.action === "job-create" ? 202 : 200;
        if (response.status !== successStatus)
            throw Error();
        const value = route.response.parse(raw);
        return Response.json(value, { status: successStatus, headers: { "Cache-Control": "no-store" } });
    }
    catch {
        return dataFailure(mutation ? "DATA_UNKNOWN" : "DATA_UNAVAILABLE", mutation ? 503 : 502);
    }
}
