import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { workbenchProtocolError } from "./workbench-proxy";
import { proxyCommercialRead } from "./commercial-proxy";

const deadline=()=>workbenchProtocolError(504,"DEADLINE_EXCEEDED","Commercial read request ended before completion");
const authenticatedGET=serverAuth(async(request:NextRequest & {auth?:unknown})=> request.signal.aborted?deadline():proxyCommercialRead(request,readZitadelServerAccessToken(request.auth as never)));

export async function commercialGET(request:NextRequest):Promise<Response> {
  if(request.signal.aborted) return deadline();
  const controller=new AbortController(); const abort=()=>controller.abort();
  request.signal.addEventListener("abort",abort,{once:true});
  const timeout=setTimeout(abort,15000);
  let onAbort=()=>{};
  const ended=new Promise<Response>(resolve=>{onAbort=()=>resolve(deadline());controller.signal.addEventListener("abort",onAbort,{once:true});});
  try {
    const scoped=new NextRequest(request,{signal:controller.signal});
    return await Promise.race([Promise.resolve(authenticatedGET(scoped,{params:Promise.resolve({})})),ended]) ?? workbenchProtocolError(503,"DEPENDENCY_UNAVAILABLE","Authentication is unavailable");
  } catch { return controller.signal.aborted?deadline():workbenchProtocolError(503,"DEPENDENCY_UNAVAILABLE","Authentication is unavailable"); }
  finally { clearTimeout(timeout);request.signal.removeEventListener("abort",abort);controller.signal.removeEventListener("abort",onAbort); }
}

const reject=()=>workbenchProtocolError(405,"INVALID_REQUEST","Method is not allowed");
export const POST=reject;export const PUT=reject;export const PATCH=reject;export const DELETE=reject;export const HEAD=reject;export const OPTIONS=reject;
