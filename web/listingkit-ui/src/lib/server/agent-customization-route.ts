import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { customizationFailure, proxyAgentCustomization } from "./agent-customization-proxy";
const authenticated = serverAuth(async (request: NextRequest & {
    auth?: unknown;
}) => { const identity = readZitadelIdentityFromSession(request.auth as never); return proxyAgentCustomization(request, readZitadelServerAccessToken(request.auth as never), String(identity?.userId ?? "")); });
export async function handleAgentCustomization(request: NextRequest) {
    const controller = new AbortController(), abort = () => controller.abort();
    request.signal.addEventListener("abort", abort, { once: true });
    if (request.signal.aborted)
        abort();
    const timer = setTimeout(abort, 40000);
    const failure = () => customizationFailure(504, request.method === "GET" ? "CUSTOMIZATION_UNAVAILABLE" : "OUTCOME_UNKNOWN");
    let finish = () => { };
    const ended = new Promise<Response>(resolve => { finish = () => resolve(failure()); controller.signal.addEventListener("abort", finish, { once: true }); if (controller.signal.aborted)
        finish(); });
    try {
        const response = await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]);
        return controller.signal.aborted ? failure() : response ?? customizationFailure(503, "CUSTOMIZATION_UNAVAILABLE");
    }
    catch {
        return customizationFailure(503, request.method === "GET" ? "CUSTOMIZATION_UNAVAILABLE" : "OUTCOME_UNKNOWN");
    }
    finally {
        clearTimeout(timer);
        request.signal.removeEventListener("abort", abort);
        controller.signal.removeEventListener("abort", finish);
    }
}
export const rejectAgentCustomization = () => customizationFailure(405, "CUSTOMIZATION_INVALID");
