import {z} from "zod";
import {collectionID,collectionReceiptSchema} from "./product-collection";
export const marketID=collectionID;
const text=(max:number,min=0)=>z.string().refine(s=>new TextEncoder().encode(s).length<=max&&!s.includes("\0")&&(min===0||s.trim().length>=min));
const revision=z.number().int().positive().max(Number.MAX_SAFE_INTEGER);
const count=z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const version=z.string().regex(/^[1-9][0-9]{0,18}$/).refine(s=>BigInt(s)<=BigInt("9223372036854775807"));
const time=z.iso.datetime({offset:true});
const https=text(2048,1).refine(s=>{try{const u=new URL(s);return u.protocol==="https:"&&!!u.hostname&&!u.username&&!u.password&&!u.hash}catch{return false}});
const image=https.refine(s=>!new URL(s).search);
const attrs=z.record(text(200,1),text(4000)).refine(v=>Object.keys(v).length<=256);
const marketProductSchema=z.object({title:text(2000,1),description:text(8192).optional(),brand:text(1000).optional(),attributes:attrs.optional(),images:z.array(image).min(1).max(40),variants:z.array(z.object({sourceId:text(128),title:text(2000).optional(),sku:text(1000).optional(),attributes:attrs.optional(),currency:text(16,1).optional(),price:z.number().finite().nonnegative().optional(),stock:count,images:z.array(image).max(40).optional()}).strict()).max(256).optional()}).strict();
const marketSupplySchema=z.object({stock:count.max(1e9),capacity:text(4000).optional(),minimumQuantity:revision.max(1e9),leadDays:count.max(3650),province:text(100,1),city:text(100,1),priceNote:text(4000).optional(),afterSaleNote:text(4000).optional()}).strict().refine(s=>s.stock>0||!!s.capacity?.trim());
const marketConnectionSchema=z.object({name:text(200,1),website:https,contact:text(100,1),telephone:text(100,1),categories:text(4000,1)}).strict();
const marketSelectionSchema=z.object({itemId:marketID,expectedRevision:revision,originalPublicationId:text(128,1),originalVersion:version,effectiveVersion:version,applyId:marketID.optional()}).strict();
const files=z.array(marketID).max(6).refine(v=>new Set(v).size===v.length);
export const marketCommandSchema=z.discriminatedUnion("action",[
 z.object({action:z.literal("create_official_draft"),selection:marketSelectionSchema,supply:marketSupplySchema,disclosure:z.literal(true)}).strict(),
 z.object({action:z.literal("submit_selected"),selection:marketSelectionSchema,supply:marketSupplySchema,fileIds:files.refine(v=>v.length>0),disclosure:z.literal(true)}).strict(),
 z.object({action:z.literal("submit_connection"),connection:marketConnectionSchema}).strict(),
 z.object({action:z.literal("supplement"),id:marketID,expectedRevision:revision,note:text(2000,1),fileIds:files.optional()}).strict(),
 z.object({action:z.literal("select_release"),id:marketID,expectedRevision:revision}).strict(),
 z.object({action:z.enum(["evaluate","request_supplement","reject","confirm_plan","close"]),id:marketID,expectedRevision:revision,note:text(2000,1)}).strict(),
 z.object({action:z.literal("approve"),id:marketID,expectedRevision:revision,note:text(2000,1),cooperationConfirmed:z.literal(true)}).strict(),
 z.object({action:z.enum(["publish","revoke"]),id:marketID,expectedRevision:revision,note:text(2000).optional()}).strict(),
]);
export type MarketCommand=z.infer<typeof marketCommandSchema>;
export const marketReleaseSchema=z.object({id:marketID,channel:z.enum(["official","selected"]),product:marketProductSchema,supply:marketSupplySchema,revision,active:z.boolean(),publishedAt:time}).strict();
const marketStageSchema=z.enum(["DRAFT","SUBMITTED","EVALUATING","SUPPLEMENT_REQUIRED","APPROVED","REJECTED","PLAN_CONFIRMED","CLOSED"]);
export const marketRecordSchema=z.object({id:marketID,kind:z.enum(["official","selected","connection"]),source:z.object({itemId:marketID,itemRevision:revision,productKey:text(128,1),originalPublicationId:text(128,1),originalVersion:version,publicationId:text(128,1),version,applyId:marketID.optional()}).strict().optional(),product:marketProductSchema.optional(),supply:marketSupplySchema.optional(),connection:marketConnectionSchema.optional(),fileIds:files.optional(),disclosure:z.boolean(),stage:marketStageSchema,revision,cooperationConfirmed:z.boolean(),createdAt:time}).strict();
const marketEventSchema=z.object({id:marketID,recordId:marketID,actorId:text(128,1),action:text(64,1),note:text(2000),fileIds:files.optional(),stage:marketStageSchema,createdAt:time}).strict();
export const marketReceiptSchema=z.object({operationId:marketID,recordId:marketID.optional(),releaseId:marketID.optional(),collection:collectionReceiptSchema.optional(),revision:count,replayed:z.boolean()}).strict();
export const marketFileSchema=z.object({id:marketID,contentType:z.enum(["image/jpeg","image/png","application/pdf"]),sha256:z.string().regex(/^[a-f0-9]{64}$/),size:revision.max(20*1024*1024),createdAt:time}).strict();
const page=<T extends z.ZodType>(schema:T)=>z.object({items:z.array(schema).max(50).nullable().transform(v=>v??[]),total:count,nextCursor:marketID.optional()}).strict();
export const marketReleasesSchema=page(marketReleaseSchema),marketRecordsSchema=page(marketRecordSchema),marketEventsSchema=page(marketEventSchema);
export type MarketReceipt=z.infer<typeof marketReceiptSchema>;
export const marketChoiceSchema=z.object({selection:marketSelectionSchema,product:marketProductSchema,optimized:z.boolean()}).strict();
export const applicationOverviewSchema=z.object({eligibleProducts:count,reviewing:count,supplementRequired:count,approved:count,products:z.array(z.object({itemId:marketID,title:text(2000,1),thumbnailUrl:image,groupName:text(200,1),source:z.literal("own")}).strict()).max(3)}).strict().refine(v=>v.products.length<=v.eligibleProducts);
export function marketEndpoint(url:URL,method:string){
 const m=/^\/api\/(workbench|admin)\/supply-market\/([a-z0-9/-]+)$/.exec(url.pathname);if(!m)return null;
 const admin=m[1]==="admin",path=m[2]!;let output:z.ZodType<unknown>|undefined,upload=false,download=false;
 if(method==="POST"&&!url.search&&["commands",...(!admin?["select","uploads"]:[])].includes(path)){upload=path==="uploads";output=upload?marketFileSchema:marketReceiptSchema}
 if(method==="GET"){
  if(!admin&&path==="application-overview")output=applicationOverviewSchema;
  if(!admin&&path==="releases")output=marketReleasesSchema;
  if(!admin&&/^releases\/[a-f0-9-]{36}$/.test(path)&&marketID.safeParse(path.split("/")[1]).success)output=marketReleaseSchema;
  if(!admin&&/^choices\/[a-f0-9-]{36}$/.test(path)&&marketID.safeParse(path.split("/")[1]).success)output=marketChoiceSchema;
  if(path==="records")output=marketRecordsSchema;
  if(/^records\/[a-f0-9-]{36}\/releases$/.test(path)&&marketID.safeParse(path.split("/")[1]).success)output=marketReleasesSchema;
  if(/^records\/[a-f0-9-]{36}(\/events)?$/.test(path)&&marketID.safeParse(path.split("/")[1]).success)output=path.endsWith("/events")?marketEventsSchema:marketRecordSchema;
  if(/^records\/[a-f0-9-]{36}\/files\/[a-f0-9-]{36}$/.test(path)&&marketID.safeParse(path.split("/")[1]).success&&marketID.safeParse(path.split("/")[3]).success){output=marketFileSchema;download=true}
  if(/^by-key\/[a-f0-9-]{36}$/.test(path)&&marketID.safeParse(path.split("/")[1]).success)output=marketReceiptSchema;
  const listing=path==="records"||path==="releases"||path.endsWith("/events")||path.endsWith("/releases");
  for(const [k,v] of url.searchParams){if(!listing||url.searchParams.getAll(k).length!==1||!(["after","keyword","kind","limit","ended"].includes(k))||k==="after"&&!marketID.safeParse(v).success||k==="keyword"&&!text(80).safeParse(v).success||k==="kind"&&!["official","selected","connection"].includes(v)||k==="limit"&&!/^(?:[1-9]|[1-4][0-9]|50)$/.test(v)||k==="ended"&&!["true","false"].includes(v))return null}
 }
 return output?{path,admin,output,upload,download}:null;
}
