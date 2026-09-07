import { NextResponse } from "next/server";
import { COMMERCIAL_MAX_BYTES, parseCommercialOverview, parseCommercialReadFailure } from "@/lib/api/commercial";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { WORKBENCH_COOKIE_NAME, workbenchProtocolError } from "./workbench-proxy";
import { newRequestLogId } from "./request-log";

function configuredOrigin(): string|null {
  const raw=process.env.COMMERCIAL_API_ORIGIN;
  if(!raw) return null;
  try { const url=new URL(raw); return ["http:","https:"].includes(url.protocol) && !url.username && !url.password && (raw===url.origin || raw===`${url.origin}/`)?url.origin:null; } catch { return null; }
}
function selectedOrganization(request:Request):string|null {
  const cookies=(request.headers.get("cookie")??"").split(";").map(v=>v.trim()).filter(v=>v.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if(cookies.length!==1) return null;
  try { const value=decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length+1)); return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value)?value:null; } catch { return null; }
}
const failure=(status:number,code:string)=>workbenchProtocolError(status,code,"Commercial read could not be completed");
const safeJSON=(body:unknown,status:number)=>NextResponse.json(body,{status,headers:{"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"}});

export async function proxyCommercialRead(request:Request,accessToken:string):Promise<Response> {
  if(request.signal.aborted) return failure(504,"DEADLINE_EXCEEDED");
  if(request.method!=="GET") return failure(405,"INVALID_REQUEST");
  const url=new URL(request.url);
  if(url.pathname!=="/api/workbench/commercial/overview" || url.search!=="" || url.href.endsWith("?") || request.body!==null || (request.headers.has("content-length") && request.headers.get("content-length")!=="0") || request.headers.has("transfer-encoding")) { void request.body?.cancel().catch(()=>undefined); return failure(400,"INVALID_REQUEST"); }
  if(!accessToken) return failure(401,"AUTHENTICATION_REQUIRED");
  const organization=selectedOrganization(request);
  if(!organization || request.headers.get("X-Expected-Organization-ID")!==organization) return failure(409,"ORGANIZATION_CONTEXT_CHANGED");
  const origin=configuredOrigin(); if(!origin) return failure(503,"DEPENDENCY_UNAVAILABLE");
  try {
    const upstream=await fetch(new URL("/api/v1/workbench/commercial/overview",origin),{method:"GET",headers:{Accept:"application/json",Authorization:`Bearer ${accessToken}`,"X-Requested-Organization-ID":organization,"X-Request-ID":newRequestLogId()},cache:"no-store",redirect:"manual",signal:request.signal});
    if(upstream.status>=300 && upstream.status<400) { void upstream.body?.cancel().catch(()=>undefined); return failure(502,"INVALID_UPSTREAM_RESPONSE"); }
    let payload:unknown;
    try { payload=await readBoundedStrictJSON(upstream,upstream.status===200?COMMERCIAL_MAX_BYTES:8192,request.signal); } catch { return request.signal.aborted?failure(504,"DEADLINE_EXCEEDED"):failure(502,"INVALID_UPSTREAM_RESPONSE"); }
    request.signal.throwIfAborted();
    if(upstream.status===200) { const result=parseCommercialOverview(payload); return result?.organization_id===organization?safeJSON(result,200):failure(502,"INVALID_UPSTREAM_RESPONSE"); }
    const error=parseCommercialReadFailure(payload,upstream.status); if(!error) return failure(502,"INVALID_UPSTREAM_RESPONSE");
    const response=safeJSON(error,upstream.status);
    if(["ORGANIZATION_ACCESS_REVOKED","ORGANIZATION_ACCESS_DENIED"].includes(error.code)) response.cookies.set(WORKBENCH_COOKIE_NAME,"",{httpOnly:true,sameSite:"lax",path:"/",secure:process.env.NODE_ENV!=="development",maxAge:0});
    return response;
  } catch { return request.signal.aborted?failure(504,"DEADLINE_EXCEEDED"):failure(503,"DEPENDENCY_UNAVAILABLE"); }
}
