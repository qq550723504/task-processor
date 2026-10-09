import {z} from "zod";
import {collectionID,collectionSourceKindSchema} from "./product-collection";
export const podID=collectionID;
const text=(n:number)=>z.string().refine(s=>new TextEncoder().encode(s).length<=n&&!s.includes("\0"));
const remote=z.string().regex(/^[1-9][0-9]{0,63}$/),hash=z.string().regex(/^[a-f0-9]{64}$/),version=z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v=>BigInt(v)<=BigInt("9223372036854775807")),integer=z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const image=text(2048).refine(s=>{try{const u=new URL(s);return u.protocol==="https:"&&u.hostname==="cdn.sdspod.com"&&!u.username&&!u.password&&!u.search&&!u.hash}catch{return false}});
const https=text(2048).refine(s=>{try{const u=new URL(s);return u.protocol==="https:"&&!u.username&&!u.password&&!u.hash}catch{return false}});
const metadata=text(8192).refine(s=>{try{const u=new URL(s);return !u.username&&!u.password&&!u.hash&&(u.hostname==="cdn.sdspod.com"&&u.protocol==="https:"||u.hostname==="e.sdspod.com"&&u.pathname==="/builds"&&["https:","http:"].includes(u.protocol))}catch{return false}});
export const podInputSchema=z.object({itemId:podID,revision:integer.positive(),source:z.object({productKey:text(128).min(1),publicationId:text(128).min(1),version,operationId:podID.optional(),kind:collectionSourceKindSchema}).strict()}).strict();
const templateVariant=z.object({id:remote,name:text(1024),sku:text(128),size:text(128),color:text(128),type:text(32),images:z.array(image).max(32)}).strict();
export const podTemplateSchema=z.object({id:remote,name:text(1024).min(1),sku:text(128),images:z.array(image).min(1).max(32),variants:z.array(templateVariant).max(256)}).strict();
export const podTemplatesSchema=z.object({items:z.array(podTemplateSchema).max(50),total:integer,page:integer.positive(),size:integer.positive().max(50)}).strict();
export const podTemplateDetailSchema=z.object({template:podTemplateSchema,hash}).strict();
export const podManifestSchema=z.object({manifest:z.object({parentId:remote,variantId:remote,prototypeId:remote,groupId:remote,type:z.literal("FREE"),layers:z.array(z.object({id:remote,width:integer.positive().max(20000),height:integer.positive().max(20000),printWidth:integer.positive().max(20000),printHeight:integer.positive().max(20000)}).strict()).min(1).max(16),renderFiles:z.array(z.object({id:remote,thumbnail:metadata}).strict()).min(1).max(32)}).strict(),hash}).strict();
const selection=z.object({itemId:podID,originalPublicationId:text(128),originalSnapshotVersion:integer.positive(),effectiveCatalogVersion:integer.positive(),applyReceiptId:podID.optional(),targetPlatform:z.literal("sds")}).strict();
export const podArtworkSchema=z.object({item:podInputSchema,selection,images:z.array(z.object({id:podID,url:https,width:integer,height:integer}).strict()).max(256)}).strict();
export const podApprovalSchema=z.object({actionId:podID,assetIds:z.array(text(128).min(1)).length(1),image:z.object({url:https,hash,width:integer.positive().max(10000),height:integer.positive().max(10000)}).strict().refine(v=>v.width*v.height<=40000000)}).strict();
export const podProgressSchema=z.object({id:podID,name:text(200).min(1),state:z.enum(["QUEUED","MATERIAL_PENDING","DESIGN_PENDING","VERIFYING","UNKNOWN","SAVED"]),createdAt:z.iso.datetime({offset:true}),finished:z.object({id:remote,number:text(128).min(1),images:z.array(image).min(1).max(32)}).strict().optional()}).strict().refine(p=>(p.state==="SAVED")===!!p.finished);
export const podReceiptSchema=z.object({operationId:podID,batchId:podID,itemId:podID,revision:integer.positive(),replayed:z.boolean()}).strict();
export const podDesignSchema=z.object({templateItem:podInputSchema,variantId:remote,manifestHash:hash,artworkHash:hash,artworkItem:podInputSchema,effectiveVersion:version,applyReceiptId:podID.optional(),actionId:text(128).min(1),assetId:text(128).min(1),transforms:z.array(z.object({layerId:remote,x:z.number().finite().min(0).max(1),y:z.number().finite().min(0).max(1),scale:z.number().finite().min(.05).max(4),angle:z.number().finite().min(-180).max(180)}).strict()).min(1).max(16),name:text(200).min(1).refine(s=>s.trim()===s&&!/[\r\n]/.test(s))}).strict();
const approveBody=z.object({actionId:podID,selection,images:z.array(z.object({id:podID,role:z.literal("design")}).strict()).length(1),approved:z.array(z.never()).length(0)}).strict();
const templateBody=z.object({id:remote,hash}).strict(),empty=z.object({}).strict();
export const podIntentSchema=z.discriminatedUnion("kind",[
 z.object({kind:z.literal("template"),key:podID,body:templateBody}).strict(),
 z.object({kind:z.literal("design"),key:podID,body:podDesignSchema}).strict(),
 z.object({kind:z.literal("approval"),key:podID,body:approveBody}).strict().refine(i=>i.key===i.body.actionId),
 z.object({kind:z.literal("finished"),key:podID,id:podID,body:empty}).strict(),
]);
export type PODIntent=z.infer<typeof podIntentSchema>;
export type PODProgress=z.infer<typeof podProgressSchema>;
export type PODInput=z.infer<typeof podInputSchema>;
export type PODArtwork=z.infer<typeof podArtworkSchema>;
export type PODManifest=z.infer<typeof podManifestSchema>;
export type PODApproval=z.infer<typeof podApprovalSchema>;
export type PODTemplate=z.infer<typeof podTemplateSchema>;
export type PODResult=z.infer<typeof podApprovalSchema>|z.infer<typeof podProgressSchema>|z.infer<typeof podReceiptSchema>;
export function podEndpoint(url:URL,method:string):{path:string;output:z.ZodType;input?:z.ZodType;approval?:boolean}|null{
 const prefix="/api/workbench/pod/";if(!url.pathname.startsWith(prefix))return null;const path=url.pathname.slice(prefix.length),p=path.split("/");if(p.some(s=>!s||/%/.test(s)))return null;
 const noQuery=()=>!url.search;const valid=(s?:string)=>podID.safeParse(s).success;
 if(method==="GET"){
  if(path==="templates"){for(const [k,v]of url.searchParams){if(!["page","size","keyword"].includes(k)||url.searchParams.getAll(k).length!==1)return null;if(k==="keyword"){if(new TextEncoder().encode(v).length>200||/[\x00\r\n]/.test(v))return null}else if(!/^\d+$/.test(v))return null}const page=Number(url.searchParams.get("page")??1),size=Number(url.searchParams.get("size")??10);if(page<1||page>10000||size<1||size>50)return null;return {path,output:podTemplatesSchema}}
  if(p.length===2&&p[0]==="templates"&&remote.safeParse(p[1]).success&&noQuery())return {path,output:podTemplateDetailSchema};
  if(p.length===2&&p[0]==="manifests"&&valid(p[1])&&url.searchParams.size===1&&remote.safeParse(url.searchParams.get("variant")).success)return {path,output:podManifestSchema};
  if(p.length===2&&p[0]==="artwork"&&valid(p[1])&&noQuery())return {path,output:podArtworkSchema};
  if(p.length===4&&p[0]==="artwork"&&valid(p[1])&&p[2]==="approvals"&&valid(p[3])&&noQuery())return {path,output:podApprovalSchema};
  if(p[0]==="designs"&&valid(p[1])&&(p.length===2||p.length===3&&p[2]==="verify")&&noQuery())return {path,output:podProgressSchema};
  if(p.length===2&&p[0]==="by-key"&&valid(p[1])&&noQuery())return {path,output:podProgressSchema};
  if(p.length===3&&p[0]==="imports"&&p[1]==="by-key"&&valid(p[2])&&noQuery())return {path,output:podReceiptSchema};
 }
 if(method==="POST"&&noQuery()){if(path==="templates/select")return {path,input:templateBody,output:podReceiptSchema};if(path==="designs")return {path,input:podDesignSchema,output:podProgressSchema};if(path==="approvals")return {path,input:approveBody,output:podApprovalSchema,approval:true};if(p.length===3&&p[0]==="designs"&&valid(p[1])&&p[2]==="select")return {path,input:empty,output:podReceiptSchema}}
 return null;
}
