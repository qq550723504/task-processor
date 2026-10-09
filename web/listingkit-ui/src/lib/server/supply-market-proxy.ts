import {marketEndpoint,marketCommandSchema,marketID} from "../contracts/supply-market";
import {readBoundedStrictJSON} from "../api/strict-json-response";
import {hasTrustedSameOriginWrite} from "./same-origin-write";
import {hasEmptyBody} from "./members-proxy";
import {WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
const headers={"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"};
export const marketFailure=(status:number,code:string)=>Response.json({code},{status,headers});
async function boundedBytes(body:ReadableStream<Uint8Array>|null,max:number,signal:AbortSignal){
 if(!body)throw new Error("empty file");const reader=body.getReader(),chunks:Uint8Array[]=[];let length=0;const cancel=()=>{void reader.cancel().catch(()=>undefined)};signal.addEventListener("abort",cancel,{once:true});
 try{while(true){signal.throwIfAborted();const r=await reader.read();signal.throwIfAborted();if(r.done)break;length+=r.value.length;if(length>max)throw new Error("file too large");chunks.push(r.value)}const data=new Uint8Array(length);let offset=0;for(const c of chunks){data.set(c,offset);offset+=c.length}return data.buffer}catch(e){cancel();throw e}finally{signal.removeEventListener("abort",cancel);reader.releaseLock()}
}
export async function proxySupplyMarket(request:Request,token:string,userId:string){
 if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true")return marketFailure(503,"DEPENDENCY_UNAVAILABLE");
 if(!token||!userId)return marketFailure(401,"AUTHENTICATION_REQUIRED");
 if(request.headers.get("X-Expected-User-ID")!==userId)return marketFailure(409,"IDENTITY_CONTEXT_CHANGED");
 const url=new URL(request.url),route=marketEndpoint(url,request.method),write=request.method!=="GET";
 if(!route||request.url.endsWith("?")||request.headers.has("content-encoding"))return marketFailure(400,"INVALID_REQUEST");
 if(write&&!hasTrustedSameOriginWrite(request))return marketFailure(403,"PERMISSION_DENIED");
 const upstreamHeaders=new Headers({Authorization:"Bearer "+token,Accept:route.download?"application/octet-stream":"application/json"});
 if(!route.admin){let org="";const cookies=(request.headers.get("cookie")??"").split(";").map(s=>s.trim()).filter(s=>s.startsWith(WORKBENCH_COOKIE_NAME+"="));try{if(cookies.length===1)org=decodeURIComponent(cookies[0]!.slice(WORKBENCH_COOKIE_NAME.length+1))}catch{}
  if(!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org)||org!==request.headers.get("X-Expected-Organization-ID"))return marketFailure(409,"ORGANIZATION_CONTEXT_CHANGED");upstreamHeaders.set("X-Requested-Organization-ID",org);
 }
 let origin:string;try{const base=new URL(process.env.LISTINGKIT_SERVICE_API_BASE??"");if(!["https:","http:"].includes(base.protocol)||base.username||base.password||base.search||base.hash||!["/api/v1","/api/v1/"].includes(base.pathname))throw new Error();origin=base.origin}catch{return marketFailure(503,"DEPENDENCY_UNAVAILABLE")}
 const controller=new AbortController(),abort=()=>controller.abort();request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();const timeout=setTimeout(abort,33000);let dispatched=false;
 try{
  let body:string|ArrayBuffer|undefined;
  if(write){const key=request.headers.get("Idempotency-Key");if(!marketID.safeParse(key).success)return marketFailure(400,"INVALID_REQUEST");upstreamHeaders.set("Idempotency-Key",key!);
   if(route.upload){const type=request.headers.get("Content-Type")??"";if(!["image/png","image/jpeg","application/pdf"].includes(type)||request.headers.has("If-Match"))return marketFailure(400,"INVALID_REQUEST");body=await boundedBytes(request.body,20*1024*1024,controller.signal);upstreamHeaders.set("Content-Type",type)}
   else{const raw=await readBoundedStrictJSON(new Response(request.body,{headers:{"Content-Type":request.headers.get("Content-Type")??""}}),65536,controller.signal),input=marketCommandSchema.safeParse(raw);
    if(!input.success)return marketFailure(400,"INVALID_REQUEST");const i=input.data;
    if(route.admin&&!(["evaluate","request_supplement","approve","reject","confirm_plan","close","publish","revoke"].includes(i.action))||!route.admin&&(route.path==="select"?i.action!=="select_release":["select_release","evaluate","request_supplement","approve","reject","confirm_plan","close","publish","revoke"].includes(i.action)))return marketFailure(400,"INVALID_REQUEST");
    if("id" in i){if(request.headers.get("If-Match")!=='"'+i.expectedRevision+'"')return marketFailure(400,"INVALID_REQUEST");upstreamHeaders.set("If-Match",request.headers.get("If-Match")!)}else if(request.headers.has("If-Match"))return marketFailure(400,"INVALID_REQUEST");
    body=JSON.stringify(i);upstreamHeaders.set("Content-Type","application/json");
   }
  }else if(!(await hasEmptyBody(request,controller.signal)))return marketFailure(400,"INVALID_REQUEST");
  controller.signal.throwIfAborted();dispatched=true;
  const response=await fetch(origin+(route.admin?"/api/v1/admin/supply-market/":"/api/v1/workbench/supply-market/")+route.path+url.search,{method:request.method,headers:upstreamHeaders,body,signal:controller.signal,cache:"no-store",redirect:"manual"});
  if(route.download&&response.status===200){const type=response.headers.get("Content-Type")??"",fileId=route.path.split("/")[3],extension=({"image/png":"png","image/jpeg":"jpg","application/pdf":"pdf"} as Record<string,string>)[type];if(!extension)throw new Error();const data=await boundedBytes(response.body,20*1024*1024,controller.signal);return new Response(data,{headers:{...headers,"Content-Type":type,"Content-Disposition":'attachment; filename="qualification-'+fileId+"."+extension+'"'}})}
  const payload=await readBoundedStrictJSON(response,2*1024*1024,controller.signal);controller.signal.throwIfAborted();
  if(!response.ok){const code=(payload as {code?:unknown})?.code;if(write&&response.status>=500)return marketFailure(response.status,"OUTCOME_UNKNOWN");return marketFailure(response.status>=400&&response.status<600?response.status:502,typeof code==="string"&&/^[A-Z_]{1,80}$/.test(code)?code:"DEPENDENCY_UNAVAILABLE")}
  const parsed=route.output.safeParse(payload);if(response.status!==200||!parsed.success)throw new Error();
  if((route.path.startsWith("releases/")||/^records\/[a-f0-9-]{36}$/.test(route.path))&&(parsed.data as {id?:string}).id!==route.path.split("/")[1])throw new Error();
  return Response.json(parsed.data,{headers});
 }catch{return marketFailure(dispatched?502:400,write&&dispatched?"OUTCOME_UNKNOWN":dispatched?"DEPENDENCY_UNAVAILABLE":"INVALID_REQUEST")}finally{clearTimeout(timeout);request.signal.removeEventListener("abort",abort)}
}
