import {z} from "zod";
import * as c from "../contracts/supply-market";
import type {CollectionScope} from "./product-collection";
import {readBoundedStrictJSON} from "./strict-json-response";
export type MarketScope=CollectionScope;
export class MarketAPIError extends Error{constructor(public readonly code:string,public readonly status:number){super(code)}}
export type MarketIntent={key:string;command:c.MarketCommand;admin:boolean};
export const marketIntentSchema=z.object({key:c.marketID,command:c.marketCommandSchema,admin:z.boolean()}).strict().refine(i=>i.admin?["evaluate","request_supplement","approve","reject","confirm_plan","close","publish","revoke"].includes(i.command.action):["create_official_draft","submit_selected","submit_connection","supplement","select_release"].includes(i.command.action));
function marketRequest<T>(scope:MarketScope,path:string,schema:z.ZodType<T>,signal?:AbortSignal,admin=false,write?:{key:string;command?:c.MarketCommand;file?:File}):Promise<T>{
 const headers=new Headers({Accept:"application/json","X-Expected-User-ID":scope.userId});if(!admin)headers.set("X-Expected-Organization-ID",scope.organizationId);
 let body:string|File|undefined;if(write){headers.set("Idempotency-Key",c.marketID.parse(write.key));if(write.file){headers.set("Content-Type",write.file.type);body=write.file}else{const command=c.marketCommandSchema.parse(write.command);headers.set("Content-Type","application/json");if("id"in command)headers.set("If-Match",'"'+command.expectedRevision+'"');body=JSON.stringify(command)}}
 return (async()=>{const controller=new AbortController(),abort=()=>controller.abort();signal?.addEventListener("abort",abort,{once:true});if(signal?.aborted)abort();const timer=setTimeout(abort,37000);let sent=false;
  try{controller.signal.throwIfAborted();sent=true;const r=await fetch((admin?"/api/admin/supply-market/":"/api/workbench/supply-market/")+path,{method:write?"POST":"GET",headers,body,credentials:"same-origin",cache:"no-store",redirect:"error",signal:controller.signal});const raw=await readBoundedStrictJSON(r,2*1024*1024,controller.signal);
   if(!r.ok){const code=(raw as {code?:unknown})?.code;throw new MarketAPIError(typeof code==="string"&&/^[A-Z_]{1,80}$/.test(code)?code:write?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE",r.status)}const parsed=schema.safeParse(raw);if(r.status!==200||!parsed.success)throw new Error();return parsed.data;
  }catch(e){if(e instanceof MarketAPIError)throw e;throw new MarketAPIError(write&&sent?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE",502)}finally{clearTimeout(timer);signal?.removeEventListener("abort",abort)}
 })();
}
export type MarketQuery={after?:string;keyword?:string;kind?:"official"|"selected"|"connection";ended?:boolean};
const query=(q:MarketQuery)=>{const p=new URLSearchParams({limit:"20"});if(q.after)p.set("after",q.after);if(q.keyword)p.set("keyword",q.keyword);if(q.kind)p.set("kind",q.kind);if(q.ended!==undefined)p.set("ended",String(q.ended));return "?"+p};
export const listMarket=(s:MarketScope,q:MarketQuery,signal?:AbortSignal)=>marketRequest(s,"releases"+query(q),c.marketReleasesSchema,signal);
export const readMarket=(s:MarketScope,id:string,signal?:AbortSignal)=>marketRequest(s,"releases/"+c.marketID.parse(id),c.marketReleaseSchema,signal);
export const listMarketRecords=(s:MarketScope,q:MarketQuery,admin=false,signal?:AbortSignal)=>marketRequest(s,"records"+query(q),c.marketRecordsSchema,signal,admin);
export const readMarketRecord=(s:MarketScope,id:string,admin=false,signal?:AbortSignal)=>marketRequest(s,"records/"+c.marketID.parse(id),c.marketRecordSchema,signal,admin);
export const listMarketEvents=(s:MarketScope,id:string,after?:string,admin=false,signal?:AbortSignal)=>marketRequest(s,"records/"+c.marketID.parse(id)+"/events"+query({after}),c.marketEventsSchema,signal,admin);
export const listRecordReleases=(s:MarketScope,id:string,after?:string,admin=false,signal?:AbortSignal)=>marketRequest(s,"records/"+c.marketID.parse(id)+"/releases"+query({after}),c.marketReleasesSchema,signal,admin);
export const readMarketChoice=(s:MarketScope,id:string,signal?:AbortSignal)=>marketRequest(s,"choices/"+c.marketID.parse(id),c.marketChoiceSchema,signal);
export const readApplicationOverview=(s:MarketScope,signal?:AbortSignal)=>marketRequest(s,"application-overview",c.applicationOverviewSchema,signal);
export const writeMarket=(s:MarketScope,i:MarketIntent,signal?:AbortSignal)=>marketRequest(s,i.command.action==="select_release"?"select":"commands",c.marketReceiptSchema,signal,i.admin,{key:i.key,command:i.command});
export const resolveMarket=(s:MarketScope,i:MarketIntent,signal?:AbortSignal)=>marketRequest(s,"by-key/"+c.marketID.parse(i.key),c.marketReceiptSchema,signal,i.admin);
export const uploadQualification=(s:MarketScope,key:string,file:File,signal?:AbortSignal)=>marketRequest(s,"uploads",c.marketFileSchema,signal,false,{key,file});
export async function downloadQualification(s:MarketScope,record:string,id:string,admin=false){
 const headers={"X-Expected-User-ID":s.userId,...(!admin?{"X-Expected-Organization-ID":s.organizationId}:{})};
 const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),35000);
 try{const response=await fetch((admin?"/api/admin/supply-market/":"/api/workbench/supply-market/")+"records/"+c.marketID.parse(record)+"/files/"+c.marketID.parse(id),{headers,cache:"no-store",credentials:"same-origin",redirect:"error",signal:controller.signal});if(!response.ok)throw new MarketAPIError("PERMISSION_DENIED",response.status);const type=response.headers.get("Content-Type");if(!["image/jpeg","image/png","application/pdf"].includes(type??""))throw new Error();const reader=response.body?.getReader();if(!reader)throw new Error();const chunks:Uint8Array<ArrayBuffer>[]=[];let bytes=0;try{while(true){const r=await reader.read();if(r.done)break;bytes+=r.value.length;if(bytes>20*1024*1024)throw new Error();chunks.push(new Uint8Array(r.value))}}finally{void reader.cancel().catch(()=>undefined);reader.releaseLock()}const url=URL.createObjectURL(new Blob(chunks,{type:type!}));const a=document.createElement("a");a.href=url;a.download="qualification-"+id+({"image/png":".png","image/jpeg":".jpg","application/pdf":".pdf"} as Record<string,string>)[type!];a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}finally{clearTimeout(timer)}
}
