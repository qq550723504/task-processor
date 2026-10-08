import { z } from "zod";
import {ecoId,ecoPageSchema,ecoResultSchema,ecoCheckoutSchema,ecoFileSchema,ecoInputs,ecoMerchantSchema,ecoFinancialSchema,ecoMerchantInput} from "@/lib/api/ecoservices";
import {readBoundedStrictJSON} from "@/lib/api/strict-json-response";
import {hasTrustedSameOriginWrite} from "./same-origin-write";
import {hasEmptyBody} from "./members-proxy";
import {WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
const safeHeaders={"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"};
export function ecoservicesFailure(status:number,code:string){return Response.json({code,message:"生态服务请求未完成，请刷新状态后重试。",requestId:""},{status,headers:safeHeaders})}
type Route={path:string;admin:boolean;input?:z.ZodType;output:z.ZodType;cas:boolean;key:boolean;upload:boolean;download:boolean};
export function ecoservicesEndpoint(url:URL,method:string):Route|null{
 const admin=url.pathname.startsWith("/api/admin/ecoservices/");const prefix=admin?"/api/admin/ecoservices/":"/api/ecoservices/";
 if(!url.pathname.startsWith(prefix))return null;const path=url.pathname.slice(prefix.length),p=path.split("/");const uuid=(v:string)=>ecoId.safeParse(v).success;
 const route:Route={path,admin,output:ecoPageSchema,cas:false,key:false,upload:false,download:false};
 let input:z.ZodType|undefined;let list=false;
 if(method==="GET"){
  list=(admin?["applications","requests","due-orders"]:["catalog","applications","provider/listings","requests"]).includes(path);
  if(admin&&p.length===3&&p[0]==="requests"&&uuid(p[1])&&p[2]==="financial"){route.output=ecoFinancialSchema}else if(!admin&&p.length===3&&p[0]==="applications"&&uuid(p[1])&&p[2]==="merchant"){route.output=ecoMerchantSchema}else if(list){}else if(p.length===2&&(admin?p[0]==="requests":["catalog","requests"].includes(p[0]))&&uuid(p[1])){}else if((p.length===2&&p[0]==="files"&&uuid(p[1]))||(!admin&&p.length===3&&p[0]==="applications"&&p[1]==="files"&&uuid(p[2]))){route.download=true}else return null;
 }else if(method==="POST"||method==="PUT"){
  route.output=ecoResultSchema;route.key=true;
  if(!admin&&path==="applications"&&method==="POST")input=ecoInputs.application;
  else if(!admin&&p.length===3&&p[0]==="applications"&&uuid(p[1])&&p[2]==="agreement"&&method==="POST"){input=ecoInputs.agreement;route.cas=true}
  else if(!admin&&p.length===3&&p[0]==="applications"&&uuid(p[1])&&p[2]==="merchant"&&method==="POST"){input=ecoMerchantInput;route.output=ecoMerchantSchema;route.cas=true}
  else if(!admin&&p.length===3&&p[0]==="applications"&&uuid(p[1])&&p[2]==="files"&&method==="POST"){route.upload=true;route.output=ecoFileSchema}
  else if(!admin&&p.length===4&&p[0]==="applications"&&uuid(p[1])&&p[2]==="merchant"&&p[3]==="resume"&&method==="POST"){input=ecoInputs.empty;route.output=ecoMerchantSchema;route.cas=true}
  else if(!admin&&path==="provider/listings"&&method==="POST")input=ecoInputs.listing;
  else if(!admin&&p[0]==="provider"&&p[1]==="listings"&&uuid(p[2])&&((p.length===3&&method==="PUT")||(p.length===4&&p[3]==="publish"&&method==="POST"))){input=p.length===3?ecoInputs.listing:ecoInputs.empty;route.cas=true}
  else if(!admin&&p.length===3&&p[0]==="catalog"&&uuid(p[1])&&p[2]==="requests"&&method==="POST")input=ecoInputs.request;
  else if(!admin&&method==="POST"&&((p.length===3&&p[0]==="requests"&&uuid(p[1]))||(p.length===4&&p[0]==="provider"&&p[1]==="requests"&&uuid(p[2])))){
   const provider=p[0]==="provider",action=p.at(-1)!;
   if(action==="files"){route.upload=true;route.output=ecoFileSchema}else{
    const inputs=provider?{quote:ecoInputs.quote,start:ecoInputs.empty,delivery:ecoInputs.delivery,"refund-proposals":ecoInputs.refund,"refund-confirmation":ecoInputs.refundConfirm}:{"confirm-quote":ecoInputs.confirm,accept:ecoInputs.accept,reject:ecoInputs.reject,cancel:ecoInputs.empty,"refund-proposals":ecoInputs.refund,"refund-confirmation":ecoInputs.refundConfirm};
    input=(inputs as Record<string,z.ZodType|undefined>)[action];if(!input)return null;route.cas=true;
   }
  }else if(!admin&&p.length===3&&p[0]==="orders"&&uuid(p[1])&&p[2]==="checkout"&&method==="POST"){input=ecoInputs.empty;route.output=ecoCheckoutSchema;route.key=false}
  else if(!admin&&["applications/files","requests/files"].includes(path)&&method==="POST"){route.upload=true;route.output=ecoFileSchema}
  else if(admin&&p.length===3&&p[0]==="requests"&&uuid(p[1])&&p[2]==="fees"&&method==="POST"){input=z.object({date:z.string().regex(/^\d{4}-\d{2}-\d{2}$/)}).strict();route.output=ecoFinancialSchema;}
  else if(admin&&p.length===3&&uuid(p[1])&&method==="POST"){
   if(p[0]==="applications"&&["approve","reject"].includes(p[2]))input=ecoInputs.review;
   else if(p[0]==="requests"&&["refund-approve","refund-reject"].includes(p[2]))input=ecoInputs.refundReview;else return null;route.cas=true;
  }else return null;
  route.input=input;
 }else return null;
 if(url.search.length>2048||url.search&&!list||method!=="GET"&&url.search)return null;
 for(const [key,value]of url.searchParams){if(url.searchParams.getAll(key).length!==1||!["page","pageSize","search","category","state","side","from","to","group","stage"].includes(key))return null;if(["page","pageSize"].includes(key)&&(!/^[1-9][0-9]*$/.test(value)||Number(value)>(key==="page"?100000:100)))return null;if(key==="search"&&value.length>200)return null}
 return route;
}
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
