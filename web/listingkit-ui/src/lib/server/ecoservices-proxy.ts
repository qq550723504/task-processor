import { z } from "zod";
import {ecoId} from "@/lib/api/ecoservices";
import {ecoservicesEndpoint} from "@/lib/api/ecoservices-routes";
export {ecoservicesEndpoint} from "@/lib/api/ecoservices-routes";
import {readBoundedStrictJSON} from "@/lib/api/strict-json-response";
import {hasTrustedSameOriginWrite} from "./same-origin-write";
import {hasEmptyBody} from "./members-proxy";
import {WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
const safeHeaders={"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"};
export function ecoservicesFailure(status:number,code:string){return Response.json({code,message:"生态服务请求未完成，请刷新状态后重试。",requestId:""},{status,headers:safeHeaders})}

async function bytes(response:Response,limit:number,signal:AbortSignal):Promise<Uint8Array>{
 const reader=response.body?.getReader();if(!reader)throw new Error("missing body");const chunks:Uint8Array[]=[];let size=0;const cancel=()=>{void reader.cancel().catch(()=>undefined)};signal.addEventListener("abort",cancel,{once:true});
 try{while(true){signal.throwIfAborted();const v=await reader.read();signal.throwIfAborted();if(v.done)break;size+=v.value.length;if(size>limit)throw new Error("too large");chunks.push(v.value)}const out=new Uint8Array(size);let at=0;for(const v of chunks){out.set(v,at);at+=v.length}return out}catch(e){cancel();throw e}finally{signal.removeEventListener("abort",cancel);reader.releaseLock()}
}
export async function proxyEcoservices(request:Request,token:string,userId:string):Promise<Response>{
 if(!token||!userId)return ecoservicesFailure(401,"AUTHENTICATION_REQUIRED");if(request.headers.get("X-Expected-User-ID")!==userId)return ecoservicesFailure(409,"IDENTITY_CONTEXT_CHANGED");
 const url=new URL(request.url),route=ecoservicesEndpoint(url,request.method);if(!route||request.url.endsWith("?")||request.headers.has("content-encoding"))return ecoservicesFailure(400,"ECOSERVICES_INVALID");
 const write=request.method!=="GET";if(write&&!hasTrustedSameOriginWrite(request))return ecoservicesFailure(403,"PERMISSION_DENIED");
 let organization="";if(!route.admin){const cookies=(request.headers.get("cookie")??"").split(";").map(v=>v.trim()).filter(v=>v.startsWith(WORKBENCH_COOKIE_NAME+"="));try{if(cookies.length===1)organization=decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length+1))}catch{}
 if(!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(organization)||organization!==request.headers.get("X-Expected-Organization-ID"))return ecoservicesFailure(409,"ORGANIZATION_CONTEXT_CHANGED")}
 let origin:string;try{const upstream=new URL(process.env.LISTINGKIT_SERVICE_API_BASE??"");if(!["http:","https:"].includes(upstream.protocol)||upstream.username||upstream.password||upstream.search||upstream.hash||!["/api/v1","/api/v1/"].includes(upstream.pathname))throw new Error();origin=upstream.origin}catch{return ecoservicesFailure(503,"ECOSERVICES_UNAVAILABLE")}
 const controller=new AbortController();const abort=()=>controller.abort();request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();const timer=setTimeout(abort,35000);let dispatched=false;
 try{const headers=new Headers({Authorization:"Bearer "+token,Accept:route.download?"application/octet-stream":"application/json"});if(!route.admin)headers.set("X-Requested-Organization-ID",organization);
 let body:string|ArrayBuffer|undefined;
 if(route.key){const key=request.headers.get("Idempotency-Key");if(!ecoId.safeParse(key).success)return ecoservicesFailure(400,"ECOSERVICES_INVALID");headers.set("Idempotency-Key",key!)}
 if(route.cas||write&&!route.admin&&route.path==="applications"&&request.headers.has("If-Match")){const cas=request.headers.get("If-Match");if(!cas||!/^"[1-9][0-9]{0,18}"$/.test(cas)||BigInt(cas.slice(1,-1))>BigInt("9223372036854775807"))return ecoservicesFailure(400,"ECOSERVICES_INVALID");headers.set("If-Match",cas)}
 if(route.input){const value=await readBoundedStrictJSON(new Response(request.body,{headers:{"Content-Type":request.headers.get("Content-Type")??""}}),1024*1024,controller.signal);const parsed=route.input.safeParse(value);if(!parsed.success)return ecoservicesFailure(400,"ECOSERVICES_INVALID");body=JSON.stringify(parsed.data);headers.set("Content-Type","application/json")}
 else if(route.upload){const type=request.headers.get("Content-Type")??"";if(!/^multipart\/form-data;\s*boundary=/i.test(type))return ecoservicesFailure(400,"ECOSERVICES_INVALID");body=(await bytes(new Response(request.body),10*1024*1024+64*1024,controller.signal)).buffer as ArrayBuffer;headers.set("Content-Type",type)}
 else if(!(await hasEmptyBody(request,controller.signal)))return ecoservicesFailure(400,"ECOSERVICES_INVALID");
 controller.signal.throwIfAborted();dispatched=true;const response=await fetch(origin+(route.admin?"/api/v1/admin/ecoservices/":"/api/v1/ecoservices/")+route.path+url.search,{method:request.method,headers,body,signal:controller.signal,cache:"no-store",redirect:"manual"});
 if(route.download&&response.ok){const type=response.headers.get("Content-Type")??"";if(!["application/pdf","image/png","image/jpeg","text/plain; charset=utf-8"].includes(type))throw new Error("invalid file");const content=await bytes(response,10*1024*1024,controller.signal);return new Response(content.buffer as ArrayBuffer,{headers:{...safeHeaders,"Content-Type":type,"Content-Disposition":response.headers.get("Content-Disposition")??"attachment"}})}
 const payload=await readBoundedStrictJSON(response,1024*1024,controller.signal);controller.signal.throwIfAborted();
 if(!response.ok){if(write&&response.status>=500)return ecoservicesFailure(response.status,"OUTCOME_UNKNOWN");const error=z.object({code:z.string().regex(/^[A-Z_]{1,80}$/),message:z.string(),requestId:z.string()}).strict().safeParse(payload);return ecoservicesFailure(response.status>=400&&response.status<600?response.status:502,error.success?error.data.code:"INVALID_UPSTREAM_RESPONSE")}
 const value=route.output.safeParse(payload);if(!value.success)throw new Error("invalid response");return Response.json(value.data,{status:response.status,headers:safeHeaders});
 }catch{return ecoservicesFailure(controller.signal.aborted?504:dispatched?502:400,write&&dispatched?"OUTCOME_UNKNOWN":dispatched?"ECOSERVICES_UNAVAILABLE":"ECOSERVICES_INVALID")}finally{clearTimeout(timer);request.signal.removeEventListener("abort",abort)}
}
