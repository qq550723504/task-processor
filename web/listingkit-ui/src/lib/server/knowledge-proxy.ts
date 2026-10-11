import { z } from "zod";
import { baseSchema,basesSchema,sourceSchema,sourcesSchema,previewSchema,resultSchema,knowledgeId } from "@/lib/api/knowledge";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { officialListSchema, officialArticleSchema, officialArticleID, officialRevision } from "@/lib/api/official-knowledge";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasEmptyBody } from "./members-proxy";
const safeHeaders = {"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"};
export function knowledgeFailure(status:number,code:string) {return Response.json({code,message:"知识库请求未完成",requestId:"",fieldErrors:[]},{status,headers:safeHeaders});}
function endpoint(url:URL,method:string) {
 const parts = url.pathname.slice("/api/workbench/".length).split("/");
 const [root,id,action,revision,preview] = parts;
 if(!["knowledge-bases","knowledge-sources","official-knowledge"].includes(root)) return null;
 const official=root==="official-knowledge";
 const mutation = method !== "GET";
 let schema:z.ZodType; let upload=false, json=false, cas=false;
 if(official && parts.length===1 && method==="GET") schema=officialListSchema;
 else if(official && parts.length===4 && action==="revisions" && officialArticleID.safeParse(id).success && officialRevision.safeParse(revision).success && method==="GET") schema=officialArticleSchema;
 else if(root === "knowledge-bases" && parts.length === 1 && ["GET","POST"].includes(method)) {schema=method==="GET"?basesSchema:resultSchema; json=mutation;}
 else if(root==="knowledge-bases" && knowledgeId.safeParse(id).success && parts.length===2 && ["GET","PUT"].includes(method)) {schema=mutation?resultSchema:baseSchema; json=mutation; cas=mutation;}
 else if(root==="knowledge-bases" && knowledgeId.safeParse(id).success && parts.length===3 && action==="sources" && ["GET","POST"].includes(method)) {schema=mutation?resultSchema:sourcesSchema; upload=mutation;}
 else if(root==="knowledge-bases" && knowledgeId.safeParse(id).success && parts.length===3 && action==="disable" && method==="POST") {schema=resultSchema;cas=true;}
 else if(root==="knowledge-sources" && knowledgeId.safeParse(id).success && parts.length===2 && method==="GET") schema=sourceSchema;
 else if(root==="knowledge-sources" && knowledgeId.safeParse(id).success && parts.length===3 && ["disable","revisions"].includes(action) && method==="POST") {schema=resultSchema;upload=action==="revisions";cas=true;}
 else if(root==="knowledge-sources" && knowledgeId.safeParse(id).success && parts.length===5 && action==="revisions" && knowledgeId.safeParse(revision).success && preview==="preview" && method==="GET") schema=previewSchema;
 else return null;
 if(url.search && !(root==="knowledge-bases" && parts.length===1 && method==="GET")) return null;
 if(url.search.length>256) return null;
 for(const [key,value] of url.searchParams) {if(!["page","pageSize"].includes(key) || url.searchParams.getAll(key).length!==1 || !/^[1-9][0-9]*$/.test(value) || Number(value)>(key==="page"?100000:100)) return null;}
 return {path:parts.join("/"),schema,upload,json,cas,mutation,official,id,revision};
}
async function boundedBytes(request:Request,signal:AbortSignal):Promise<Uint8Array> {
 const reader=request.body?.getReader(); if(!reader) throw new Error("missing body");
 const chunks:Uint8Array[]=[];let size=0;const cancel=()=>{void reader.cancel().catch(()=>undefined)};
 signal.addEventListener("abort",cancel,{once:true});
 try {while(true){signal.throwIfAborted();const {done,value}=await reader.read();signal.throwIfAborted();if(done)break;size+=value.length;if(size>10*1024*1024)throw new Error("too large");chunks.push(value);}
 const bytes=new Uint8Array(size);let at=0;for(const chunk of chunks){bytes.set(chunk,at);at+=chunk.length;}return bytes;
 }catch(error){cancel();throw error;}finally{signal.removeEventListener("abort",cancel);reader.releaseLock();}
}
export async function proxyKnowledge(request:Request,token:string,userId:string):Promise<Response> {
 if(!token || !userId) return knowledgeFailure(401,"AUTHENTICATION_REQUIRED");
 if(request.headers.get("X-Expected-User-ID")!==userId) return knowledgeFailure(409,"IDENTITY_CONTEXT_CHANGED");
 const url=new URL(request.url), route=endpoint(url,request.method);
 if(!route || request.url.endsWith("?") || request.headers.has("content-encoding")) return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");
 if(route.official && (request.body!==null || request.headers.has("transfer-encoding") || request.headers.has("content-length") && request.headers.get("content-length")!=="0")) return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");
 if(route.mutation && !hasTrustedSameOriginWrite(request)) return knowledgeFailure(403,"PERMISSION_DENIED");
 const cookies=(request.headers.get("cookie")??"").split(";").map(v=>v.trim()).filter(v=>v.startsWith(WORKBENCH_COOKIE_NAME+"="));
 let organization="";
 try{if(cookies.length===1) organization=decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length+1));}catch{}
 if(!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(organization) || organization!==request.headers.get("X-Expected-Organization-ID")) return knowledgeFailure(409,"ORGANIZATION_CONTEXT_CHANGED");
 let origin:string;
 try{const upstream=new URL(process.env.LISTINGKIT_SERVICE_API_BASE??"");if(!["http:","https:"].includes(upstream.protocol)||upstream.username||upstream.password||upstream.search||upstream.hash||!["/api/v1","/api/v1/"].includes(upstream.pathname))throw new Error();origin=upstream.origin;}
 catch{return knowledgeFailure(503,"KNOWLEDGE_UNAVAILABLE");}
 const controller=new AbortController();const abort=()=>controller.abort();request.signal.addEventListener("abort",abort,{once:true});if(request.signal.aborted)abort();const timer=setTimeout(abort,route.official?10000:40000);let dispatched=false;
 try{
 const headers=new Headers({Accept:"application/json",Authorization:"Bearer "+token,"X-Requested-Organization-ID":organization});
 let body:string|ArrayBuffer|undefined;
 if(route.mutation){
 const key=request.headers.get("Idempotency-Key");if(!knowledgeId.safeParse(key).success)return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");headers.set("Idempotency-Key",key!);
 if(route.cas){const cas=request.headers.get("If-Match");if(!cas || !/^"[1-9][0-9]*"$/.test(cas)||Number(cas.slice(1,-1))>Number.MAX_SAFE_INTEGER)return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");headers.set("If-Match",cas);}
 }
 if(route.json){
 const payload=await readBoundedStrictJSON(new Response(request.body,{headers:{"Content-Type":request.headers.get("Content-Type")??""}}),8192,controller.signal);
 const input=z.object({name:z.string().min(1).max(512)}).strict().safeParse(payload);if(!input.success)return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");
 body=JSON.stringify(input.data);headers.set("Content-Type","application/json");
 }else if(route.upload){
 const contentType=request.headers.get("Content-Type")??"";if(!/^multipart\/form-data;\s*boundary=/i.test(contentType))return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");
 const bytes=await boundedBytes(request,controller.signal);body=bytes.buffer as ArrayBuffer;headers.set("Content-Type",contentType);
 }else if(!route.official && !(await hasEmptyBody(request,controller.signal)))return knowledgeFailure(400,"KNOWLEDGE_INVALID_REQUEST");
 controller.signal.throwIfAborted();dispatched=true;
 const response=await fetch(origin+"/api/v1/workbench/"+route.path+url.search,{method:request.method,headers,body,signal:controller.signal,cache:"no-store",redirect:"manual"});
 const payload=await readBoundedStrictJSON(response,route.official?(route.schema===officialListSchema?128*1024:256*1024):route.schema===previewSchema?13*1024*1024:1024*1024,controller.signal);
 controller.signal.throwIfAborted();
 if(![200,201,202].includes(response.status)){
 if(route.mutation && response.status>=500)return knowledgeFailure(response.status,"OUTCOME_UNKNOWN");
 const error=z.object({code:z.string().regex(/^[A-Z_]{1,80}$/),message:z.string(),requestId:z.string(),fieldErrors:z.array(z.unknown()).max(0)}).strict().safeParse(payload);
 return knowledgeFailure(response.status>=400 && response.status<600 ? response.status : 502,error.success?error.data.code:(route.mutation?"OUTCOME_UNKNOWN":"INVALID_UPSTREAM_RESPONSE"));
 }
 const parsed=route.schema.safeParse(payload);if(!parsed.success)throw new Error("invalid response");
 if(route.schema===officialArticleSchema) {const article=officialArticleSchema.parse(parsed.data);if(article.id!==route.id || article.revision!==route.revision)throw new Error("wrong official version");}
 return Response.json(parsed.data,{status:response.status,headers:safeHeaders});
 }catch{return knowledgeFailure(controller.signal.aborted?504:dispatched?502:400,dispatched&&route.mutation?"OUTCOME_UNKNOWN":dispatched?"KNOWLEDGE_UNAVAILABLE":"KNOWLEDGE_INVALID_REQUEST");}
 finally{clearTimeout(timer);request.signal.removeEventListener("abort",abort);}
}
