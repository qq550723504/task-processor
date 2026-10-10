import { z } from "zod";
import { DATA_MAX_BYTES, dataRoute } from "@/lib/contracts/data-services";
import { readBoundedStrictJSON } from "./strict-json-response";
export type DataScope = {
    userId: string;
    organizationId: string;
};
export type DataIntent = DataScope & {
    key: string;
    path: string;
    specialist: boolean;
    body?: unknown;
    binary?: string;
    headers?: Record<string, string>;
};
export class DataAPIError extends Error {
    constructor(public code: string, public status: number) { super(code); }
}
const safe = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
function storageKey(scope: DataScope, specialist: boolean) { return `data-services-command-v1:${scope.userId}:${specialist ? "platform" : scope.organizationId}`; }
export function validDataIntent(input: unknown): input is DataIntent {
    const parsed = z.object({ userId: z.string().regex(safe), organizationId: z.string().regex(safe), key: z.string().uuid(), path: z.string().max(160), specialist: z.boolean(), body: z.unknown().optional(), binary: z.string().max(Math.ceil(DATA_MAX_BYTES / 3) * 4).optional(), headers: z.record(z.string(), z.string()).optional() }).strict().safeParse(input);
    if (!parsed.success)
        return false;
    const i = parsed.data;
    const route = dataRoute("POST", i.path.split("/"), i.specialist);
    if (!route)
        return false;
    if (route.binary)
        return !i.body && !!i.binary && /^[A-Za-z0-9+/]+={0,2}$/.test(i.binary) && !!i.headers && Object.keys(i.headers).length === 3 && /^(csv|json|xlsx)$/.test(i.headers["X-Data-Format"] ?? "") && /^[1-9]\d{0,14}$/.test(i.headers["X-Expected-Revision"] ?? "") && /^[1-9]\d{0,14}$/.test(i.headers["X-Spec-Revision"] ?? "");
    return !i.binary && !i.headers && route.request!.safeParse(i.body).success;
}
export function saveDataIntent(intent: DataIntent) {
    try {
        if (!validDataIntent(intent))
            return false;
        const key = storageKey(intent, intent.specialist), raw = JSON.stringify(intent), existing = sessionStorage.getItem(key);
        if (existing && existing !== raw)
            return false;
        sessionStorage.setItem(key, raw);
        return sessionStorage.getItem(key) === raw;
    }
    catch {
        return false;
    }
}
export function loadDataIntent(scope: DataScope, specialist: boolean): DataIntent | null {
    try {
        const raw = sessionStorage.getItem(storageKey(scope, specialist));
        if (!raw)
            return null;
        const input: unknown = JSON.parse(raw);
        if (!validDataIntent(input) || input.userId !== scope.userId || input.organizationId !== scope.organizationId || input.specialist !== specialist)
            return null;
        return input;
    }
    catch {
        return null;
    }
}
export function clearDataIntent(scope: DataScope, specialist: boolean) { sessionStorage.removeItem(storageKey(scope, specialist)); }
export function encodeDataFile(bytes: Uint8Array) {
    let raw = "";
    for (let i = 0; i < bytes.length; i += 8192)
        raw += String.fromCharCode(...bytes.subarray(i, i + 8192));
    return btoa(raw);
}
function decodeFile(raw: string) { return Uint8Array.from(atob(raw), c => c.charCodeAt(0)); }
export async function dataRequest<T>(scope: DataScope, path: string, schema: z.ZodType<T>, intent?: DataIntent, signal?: AbortSignal, specialist = false): Promise<T> {
    if (!safe.test(scope.userId) || !safe.test(scope.organizationId) || intent && !validDataIntent(intent))
        throw new DataAPIError("INVALID_DATA_REQUEST", 400);
    if (intent && (intent.userId !== scope.userId || intent.organizationId !== scope.organizationId || intent.specialist !== specialist || intent.path !== path))
        throw new DataAPIError("IDENTITY_CONTEXT_CHANGED", 409);
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted)
        abort();
    const timer = setTimeout(abort, 24000);
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": scope.userId, "X-Expected-Organization-ID": scope.organizationId });
    let body: BodyInit | undefined;
    if (intent) {
        headers.set("Idempotency-Key", intent.key);
        if (intent.binary) {
            headers.set("Content-Type", "application/octet-stream");
            for (const [key, value] of Object.entries(intent.headers ?? {}))
                headers.set(key, value);
            body = decodeFile(intent.binary) as BodyInit;
        }
        else {
            headers.set("Content-Type", "application/json");
            body = JSON.stringify(intent.body);
        }
    }
    try {
        const response = await fetch(`${specialist ? "/api/platform/data-customization" : "/api/workbench/data-services"}${path ? (path.startsWith("?") ? path : `/${path}`) : ""}`, { method: intent ? "POST" : "GET", body, headers, cache: "no-store", credentials: "same-origin", redirect: "error", signal: controller.signal });
        const raw = await readBoundedStrictJSON(response, DATA_MAX_BYTES, controller.signal);
        if (!response.ok) {
            const error = z.object({ error: z.object({ code: z.string().max(80), message: z.string().max(160).optional() }) }).safeParse(raw);
            if (error.success)
                throw new DataAPIError(error.data.error.code, response.status);
            throw Error();
        }
        ;
        if (response.status !== (intent && !specialist && path === "amazon/jobs" ? 202 : 200))
            throw Error();
        return schema.parse(raw);
    }
    catch (e) {
        if (e instanceof DataAPIError)
            throw e;
        throw new DataAPIError(intent ? "DATA_UNKNOWN" : "DATA_UNAVAILABLE", intent ? 503 : 502);
    }
    finally {
        clearTimeout(timer);
        signal?.removeEventListener("abort", abort);
    }
}
export async function executeDataIntent(intent: DataIntent, signal?: AbortSignal) { const route = dataRoute("POST", intent.path.split("/"), intent.specialist)!; return dataRequest<unknown>(intent, intent.path, route.response, intent, signal, intent.specialist); }
export async function recoverDataIntent(intent: DataIntent, signal?: AbortSignal) {
	// Amazon creation replays its original write; command GET requires separate Collection read permission.
    let read: string | undefined;
    if (intent.specialist)
        read = `by-command/${intent.key}`;
    else if (intent.path === "keys")
        read = `keys/by-command/${intent.key}`;
    else if (intent.path === "custom")
        read = `custom/by-command/${intent.key}`;
    if (!read)
        return executeDataIntent(intent, signal);
    const route = dataRoute("GET", read.split("/"), intent.specialist)!;
    try {
        return await dataRequest<unknown>(intent, read, route.response, undefined, signal, intent.specialist);
    }
    catch (e) {
        if (e instanceof DataAPIError && e.code === "DATA_NOT_FOUND")
            return executeDataIntent(intent, signal);
        throw e;
    }
}
