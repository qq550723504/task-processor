import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { buildDataRequest, dataBrowserResponse, dataFailure } from "./data-services-proxy";
type Context = {
    params: Promise<{
        path?: string[];
    }>;
    specialist: boolean;
    signal: AbortSignal;
    state: {
        sent: boolean;
        mutation: boolean;
    };
};
const authenticated = serverAuth(async (req, ctx) => {
    const request = req;
    const context = ctx as unknown as Context;
    const identity = readZitadelIdentityFromSession(request.auth);
    const token = readZitadelServerAccessToken(request.auth);
    if (!token || typeof identity?.userId !== "string")
        return dataFailure("AUTHENTICATION_REQUIRED", 401);
    const { path = [] } = await context.params;
    const mapped = await buildDataRequest(request, path, token, identity.userId, context.specialist);
    if (mapped instanceof Response)
        return mapped;
    if (context.signal.aborted)
        return dataFailure("DATA_UNAVAILABLE", 504);
    try {
        context.state.sent = true;
        const response = await fetch(mapped.url, { ...mapped.init, signal: context.signal, redirect: "manual" });
        return await dataBrowserResponse(response, mapped.route, mapped.mutation, context.signal);
    }
    catch {
        return dataFailure(mapped.mutation ? "DATA_UNKNOWN" : "DATA_UNAVAILABLE", mapped.mutation ? 503 : 502);
    }
}) as (request: NextRequest, ctx: Context) => Promise<Response | void> | Response | void;
export async function dispatchData(request: NextRequest, context: {
    params: Promise<{
        path?: string[];
    }>;
}, specialist = false) {
    const controller = new AbortController();
    const state = { sent: false, mutation: request.method === "POST" };
    const abort = () => controller.abort();
    request.signal.addEventListener("abort", abort, { once: true });
    if (request.signal.aborted)
        abort();
    let resolveAbort = () => { };
    const timed = new Promise<Response>(resolve => { resolveAbort = () => resolve(dataFailure(state.sent && state.mutation ? "DATA_UNKNOWN" : "DATA_UNAVAILABLE", state.sent && state.mutation ? 503 : 504)); controller.signal.addEventListener("abort", resolveAbort, { once: true }); });
    const timer = setTimeout(abort, 22000);
    try {
        if (controller.signal.aborted)
            return dataFailure("DATA_UNAVAILABLE", 504);
        return (await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { ...context, specialist, signal: controller.signal, state }), timed])) ?? dataFailure("AUTHENTICATION_REQUIRED", 401);
    }
    finally {
        clearTimeout(timer);
        request.signal.removeEventListener("abort", abort);
        controller.signal.removeEventListener("abort", resolveAbort);
    }
}
