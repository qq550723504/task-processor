import { AccountReadError } from "@/lib/api/account";
import { parseVerification, verificationInputSchema, verificationErrorCode } from "@/lib/api/subject-verification";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { accountFailure, readAccountRequestBody, serviceOrigin, type AccountDispatchState } from "./account-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

export async function proxyVerification(request: Request, token: string, user: string, dispatch: AccountDispatchState = {forwarded:false}): Promise<Response> {
  if(!token || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(user)) return accountFailure(401,"AUTHENTICATION_REQUIRED");
  if(request.headers.get("X-Expected-User-ID")!==user) return accountFailure(409,"IDENTITY_CONTEXT_CHANGED");
  const write=request.method==="POST",url=new URL(request.url);
  if(!["GET","POST"].includes(request.method)||url.pathname!==`/api/account/verification${write?"/applications":""}`||url.search||request.url.endsWith("?")||!write&&(request.body!==null||request.headers.has("transfer-encoding"))) return accountFailure(400,"INVALID_REQUEST");
  if(write&&(request.headers.get("origin")!==url.origin||request.headers.get("content-type")?.split(";",1)[0]!=="application/json")) return accountFailure(403,"PERMISSION_DENIED");
  const cookies=(request.headers.get("cookie")??"").split(";").map(v=>v.trim()).filter(v=>v.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  let org="";try{if(cookies.length===1)org=decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length+1));}catch{}
  if(!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org)||request.headers.get("X-Expected-Organization-ID")!==org) return accountFailure(409,"ORGANIZATION_CONTEXT_CHANGED");
  const origin=serviceOrigin();if(!origin)return accountFailure(503,"VERIFICATION_UNAVAILABLE");
  const signal=AbortSignal.any([request.signal,AbortSignal.timeout(15000)]);
  try {
    const body=write?JSON.stringify(verificationInputSchema.parse(JSON.parse(await readAccountRequestBody(request,8192,signal)))):undefined;
    const headers=new Headers({Accept:"application/json",Authorization:`Bearer ${token}`,"X-Requested-Organization-ID":org});if(write)headers.set("Content-Type","application/json");
    signal.throwIfAborted();dispatch.forwarded=write;
    const response=await fetch(`${origin}/api/v1/account/organization/verification${write?"/applications":""}`,{method:request.method,headers,...(body?{body}:{}),cache:"no-store",redirect:"manual",signal});
    const payload=await readBoundedStrictJSON(response,16384,signal);signal.throwIfAborted();
    if(!response.ok) return write&&response.status>=500?accountFailure(502,"RESULT_UNVERIFIED","unknown"):accountFailure(response.status,verificationErrorCode(payload));
    return Response.json(parseVerification(payload,user,org),{headers:{"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"}});
  } catch(error) {
    if(dispatch.forwarded)return accountFailure(502,"RESULT_UNVERIFIED","unknown");
    if(error instanceof AccountReadError)return accountFailure(error.status,error.code);
    return accountFailure(write?400:503,write?"INVALID_REQUEST":"VERIFICATION_UNAVAILABLE");
  }
}
