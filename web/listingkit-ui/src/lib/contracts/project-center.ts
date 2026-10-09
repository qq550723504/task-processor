import { z } from "zod";
const id=z.string().uuid().refine(v=>v===v.toLowerCase() && v!=="00000000-0000-0000-0000-000000000000");
export const revision=z.number().int().positive().max(Number.MAX_SAFE_INTEGER);
export const kinds={STORE_OPERATIONS:"店铺经营",PRODUCT_DEVELOPMENT:"商品开发",PRODUCT_RESEARCH:"选品分析",BRAND_BUILDING:"品牌建设",OPC:"OPC创业",OTHER:"其他项目"} as const;
export const kind=z.enum(["STORE_OPERATIONS","PRODUCT_DEVELOPMENT","PRODUCT_RESEARCH","BRAND_BUILDING","OPC","OTHER"]);
const text=(max:number)=>z.string().min(1).refine(v=>new TextEncoder().encode(v).length<=max && v.trim().length>0 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(v));
const due=z.string().refine(v=>v==="" || /^\d{4}-\d{2}-\d{2}$/.test(v) && Number.isFinite(Date.parse(v+"T00:00:00Z")) && new Date(v+"T00:00:00Z").toISOString().slice(0,10)===v);
export const fields={title:text(256),goal:text(4096),kind,dueDate:due};
export const projectBody=z.strictObject({...fields,storeId:z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$|^$/).optional()});
export const referenceKind=z.enum(["CONVERSATION","BUSINESS_TASK","PRODUCT","KNOWLEDGE_BASE","KNOWLEDGE_SOURCE"]);
const href=z.string().regex(/^\/workbench\/[A-Za-z0-9/_?=.-]+$/).max(256);
export const reference=z.strictObject({slotId:id.or(z.literal("")),kind:referenceKind.or(z.literal("STORE")),available:z.boolean(),targetId:z.string().max(128).optional(),title:text(512).optional(),href:href.optional(),taskState:z.string().max(64).optional(),resultHref:href.optional()}).superRefine((v,c)=>{
 if(!v.available && (v.targetId || v.title || v.href || v.taskState || v.resultHref))c.addIssue({code:"custom",message:"Unavailable references must not expose protected data"});
 if(v.available && (!v.targetId || !v.title || !v.href))c.addIssue({code:"custom",message:"Incomplete reference"});
});
export const project=z.strictObject({id,...fields,archived:z.boolean(),revision,createdAt:z.string().datetime({offset:true}),updatedAt:z.string().datetime({offset:true}),storeScope:z.boolean(),store:reference.optional(),references:z.array(reference).max(100),taskTotal:z.number().int().min(0).max(100),taskCompleted:z.number().int().min(0).max(100),taskPending:z.number().int().min(0).max(100),taskSummaryAvailable:z.boolean()});
export const template=z.strictObject({id,name:text(256),...fields,revision,archived:z.boolean(),createdAt:z.string().datetime({offset:true})});
export const receipt=z.strictObject({id,revision,replayed:z.boolean()});
export const page=z.strictObject({projects:z.array(project).max(20),next:z.string().max(256)});
export const templates=z.strictObject({templates:z.array(template).max(20),next:z.string().max(256)});
export type Project=z.infer<typeof project>;
export type Template=z.infer<typeof template>;
export type Reference=z.infer<typeof reference>;
export type ProjectBody=z.infer<typeof projectBody>;
const empty=z.strictObject({});
export function projectEndpoint(url:URL,method:string){
 const prefix="/api/workbench/projects",p=url.pathname;
 if(p!==prefix && !p.startsWith(prefix+"/"))return null;
 const path=p.slice(prefix.length);let input:z.ZodType|undefined, output:z.ZodType;let expected=true;
 const uuid="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
 if(path==="" && method==="GET")output=page;
 else if(path==="" && method==="POST"){input=projectBody;output=receipt;expected=false;}
 else if(path==="/templates" && method==="GET")output=templates;
 else if(path==="/templates" && method==="POST"){input=z.strictObject({projectId:id,name:text(256)});output=receipt;}
 else if(new RegExp("^/"+uuid+"$").test(path) && method==="GET")output=project;
 else if(new RegExp("^/"+uuid+"$").test(path) && method==="PATCH"){input=projectBody;output=receipt;}
 else if(new RegExp("^/"+uuid+"/references$").test(path) && method==="POST"){input=z.strictObject({kind:referenceKind,targetId:id});output=receipt;}
 else if(new RegExp("^(/"+uuid+"/(archive|restore|visit)|/"+uuid+"/references/"+uuid+"/remove|/templates/"+uuid+"/archive)$").test(path) && method==="POST"){input=empty;output=receipt;expected=!path.endsWith("/visit");}
 else return null;
 const allowed=path==="" && method==="GET"?["mode","search","kind","workScope","storeId","after"]:path==="/templates" && method==="GET"?["after"]:[];
 for(const [key,v] of url.searchParams){if(!allowed.includes(key) || url.searchParams.getAll(key).length!==1 || v.length>256)return null;}
 return {path,input,output,expected};
}
export function referenceFromLink(value:string){
 try{const u=new URL(value,window.location.origin);if(u.origin!==window.location.origin || u.hash)return null;
  const routes:[RegExp,z.infer<typeof referenceKind>][]=[[/^\/workbench\/ai\/chat\/([^/]+)$/,"CONVERSATION"],[/^\/workbench\/ai\/tasks\/([^/]+)$/,"BUSINESS_TASK"],[/^\/workbench\/supply\/acquisition\/operation\/([^/]+)$/,"PRODUCT"],[/^\/workbench\/ai\/knowledge\/([^/]+)$/,"KNOWLEDGE_BASE"]];
  for(const [pattern,kind] of routes){const m=u.pathname.match(pattern);if(m && id.safeParse(m[1]).success && !u.search)return {kind,targetId:m[1]};}
 }catch{}return null;
}
