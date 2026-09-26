import { AccountReadError } from "@/lib/api/account";
import { parsePersonalVerification, personalInputSchema, personalRefreshSchema, personalErrorCode } from "@/lib/api/personal-verification";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { accountFailure,readAccountRequestBody,serviceOrigin,type AccountDispatchState } from "./account-proxy";

export async function proxyPersonalVerification(request:Request,token:string,user:string,dispatch:AccountDispatchState={forwarded:false}):Promise<Response>{
 if(!token||!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(user))return accountFailure(401,"AUTHENTICATION_REQUIRED");
 if(request.headers.get("X-Expected-User-ID")!==user)return accountFailure(409,"IDENTITY_CONTEXT_CHANGED");
 const u=new URL(request.url),base="/api/account/personal-verification",suffix=u.pathname.slice(base.length),write=request.method==="POST";
 if(!u.pathname.startsWith(base)||!(write?["/applications","/refresh"].includes(suffix):request.method==="GET"&&suffix==="")||u.search||request.url.endsWith("?")||!write&&(request.body!==null||request.headers.has("transfer-encoding")))return accountFailure(400,"INVALID_REQUEST");
 if(write&&(request.headers.get("origin")!==u.origin||request.headers.get("content-type")?.split(";",1)[0]!=="application/json"))return accountFailure(403,"PERMISSION_DENIED");
 const origin=serviceOrigin();if(!origin)return accountFailure(503,"VERIFICATION_UNAVAILABLE");
 const signal=AbortSignal.any([request.signal,AbortSignal.timeout(22000)]);
 try{
 const body=write?JSON.stringify((suffix==="/applications"?personalInputSchema:personalRefreshSchema).parse(JSON.parse(await readAccountRequestBody(request,16384,signal)))):undefined;
 signal.throwIfAborted();dispatch.forwarded=write;
 const r=await fetch(`${origin}/api/v1/account/verification${suffix}`,{method:request.method,headers:{Accept:"application/json",Authorization:`Bearer ${token}`,...(write?{"Content-Type":"application/json"}:{})},...(body?{body}:{}),cache:"no-store",redirect:"manual",signal});
 const payload=await readBoundedStrictJSON(r,16384,signal);signal.throwIfAborted();
 if(!r.ok){if(suffix==="/applications"&&r.status>=500)return accountFailure(502,"RESULT_UNVERIFIED","unknown");const response=accountFailure(r.status,personalErrorCode(payload));const retry=r.headers.get("retry-after");if(r.status===429&&retry&&/^[0-9]{1,6}$/.test(retry))response.headers.set("Retry-After",retry);return response;}
 return Response.json(parsePersonalVerification(payload,user),{headers:{"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"}});
 }catch(e){if(dispatch.forwarded)return accountFailure(502,"RESULT_UNVERIFIED","unknown");if(e instanceof AccountReadError)return accountFailure(e.status,e.code);return accountFailure(write?400:503,write?"INVALID_REQUEST":"VERIFICATION_UNAVAILABLE");}
}
