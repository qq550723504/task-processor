import {z} from "zod";
import {isAcquisitionUUID} from "./product-acquisition";
import {templateRefSchema} from "./agent-configuration";

const uuid=z.string().refine(isAcquisitionUUID), id=z.string().min(1).max(192), hash=z.string().regex(/^[a-f0-9]{64}$/);
const integer=z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER), positive=integer.min(1);
const url=z.string().max(4096).url().refine(value=>/^https?:\/\//.test(value));
const imageSetHeadSchema=z.strictObject({action_id:z.string().max(192),payload_hash:z.string().max(64)});
const officialImagePositionSchema=z.strictObject({Group:z.enum(["spu","skc","sku","detail"]),SKC:integer,SKU:integer,Type:positive,Sort:positive.max(100),Site:id});
const position=z.strictObject({group:z.enum(["spu","skc","sku","detail"]),skc:integer,sku:integer,type:positive,sort:positive.max(100),site:id});
const imageSetChoiceSchema=z.strictObject({kind:z.enum(["source","approved","generated","manual"]),manual_media:z.strictObject({hash,bytes:positive.max(3*1024*1024)}).optional(),source_id:id.optional(),approval_action_id:id.optional(),asset_id:id.optional(),run_id:uuid.optional(),plan_revision:positive.optional(),slot_id:id.optional(),attempt:positive.optional(),result_digest:hash.optional(),generic_head:imageSetHeadSchema.optional(),presentation:z.strictObject({group:z.enum(["carousel","detail"]),order:positive.max(40),variant_id:id.optional()}),official_placement:position.optional()});
export type ImageSetChoice=z.infer<typeof imageSetChoiceSchema>;
const imageSetPrepareSchema=z.strictObject({template:templateRefSchema.optional(),target:z.strictObject({Platform:z.enum(["product","shein"]),StoreID:z.string().max(192).optional(),Site:z.string().max(192).optional(),CategoryID:positive.optional(),RecordID:z.string().max(192).optional()}),sharedOriginalIds:z.array(id).max(8).optional(),carouselOriginalIds:z.array(id).max(8).optional(),detailOriginalIds:z.array(id).max(8).optional(),selectedTaskIds:z.array(id).max(32).optional(),officialPlacements:z.record(id,officialImagePositionSchema).optional(),effectiveCatalogVersion:positive.optional(),applyReceiptId:uuid.optional()});
export type ImageSetPrepare=z.infer<typeof imageSetPrepareSchema>;
const imageSetSelectionSchema=z.strictObject({actionId:uuid,planRevision:positive,resultDigest:hash,expectedHead:imageSetHeadSchema,choices:z.array(imageSetChoiceSchema).min(1).max(40),selectionDigest:hash.optional()});
export type ImageSetSelection=z.infer<typeof imageSetSelectionSchema>;
const source=z.object({ContextKind:z.enum(["acquisition","supply"]),ProductID:id,OperationID:id,OriginalPublicationID:id,OriginalVersion:positive,EffectiveVersion:positive,ApplyReceiptID:uuid.optional()});
const placement=z.object({Group:z.enum(["carousel","detail"]),Order:positive.max(32),VariantID:id.optional()});
const recipe=z.object({Purpose:id,Background:z.string().max(1024),Language:id,Placement:placement,OfficialPlacement:officialImagePositionSchema.optional(),References:z.array(z.object({AssetID:id})).min(1).max(8),Quote:z.object({Points:positive})});
const original=z.object({ID:id,DisplayURL:url,Width:integer,Height:integer});
const status=z.enum(["planning","awaiting_plan_approval","executing","evaluating","repairing","awaiting_final_approval","blocked","completed","failed","cancelled"]);
export const imageSetRunSchema=z.object({runId:uuid,status,confirmationActionId:z.string().max(192),generationAdmitted:z.boolean(),planRevision:positive,planDigest:hash,quoteDigest:hash,images:positive.max(32),points:positive,settledPoints:integer,resultDigest:z.string().max(64),approvalAvailable:z.boolean(),candidateSelectionAvailable:z.boolean(),regenerationAvailable:z.boolean(),plan:z.object({Source:source,Target:z.object({Platform:z.enum(["product","shein"]),StoreID:z.string(),Site:z.string(),CategoryID:integer,RecordID:z.string().optional()}),Regeneration:z.object({RunID:uuid}).optional()}),slots:z.array(z.object({slotId:id,status:z.enum(["pending","executing","evaluating","accepted","rejected","blocked"]),attempt:integer,errorCode:z.string().max(192),recipe,candidates:z.array(z.object({assetId:id,url,width:positive,height:positive})).max(1),closure:z.object({Kind:id,Points:integer}).nullable()})).min(1).max(32),originals:z.array(original).max(16),recoverableEffects:z.array(z.object({SlotID:id,Attempt:positive,Code:id})).max(32).nullable(),pendingCommand:z.object({ActionID:id,Phase:z.string(),Kind:id}).nullable(),block:z.object({Code:id}).nullable()});
export type ImageSetRun=z.infer<typeof imageSetRunSchema>;
export const imageSetSourcesSchema=z.object({contextKind:z.enum(["acquisition","supply"]),contextId:uuid,source,manualReplacementAvailable:z.boolean(),originals:z.array(z.object({id,displayUrl:url,width:integer,height:integer})).max(4096),evidence:z.record(z.string(),z.string())});
const approved=z.object({id,role:id,url,width:integer.optional(),height:integer.optional(),presentation:z.object({group:z.enum(["carousel","detail"]),order:positive,variant_id:id.optional()}).optional(),official_placement:position.optional(),selection_receipt:z.object({action_id:id,asset_id:id}).optional()});
const inventory=z.object({head:imageSetHeadSchema,assets:z.array(approved).max(40)});
export const imageSetInventorySchema=z.object({target:inventory,generic:inventory.nullable()});
export type ImageSetInventory=z.infer<typeof imageSetInventorySchema>;
export const imageSetPreviewSchema=z.object({digest:hash,head:imageSetHeadSchema,assets:z.array(approved).min(1).max(40)});
export const imageSetRecentSchema=z.object({items:z.array(z.object({runId:uuid,contextKind:z.enum(["acquisition","supply"]),contextId:uuid,status,targetPlatform:id,createdAt:z.string().datetime({offset:true})})).max(40),nextCursor:z.string().max(192)});
export const imageSetAcceptedSchema=z.strictObject({runId:uuid,status:z.literal("accepted")});
export const imageSetRoutes=["sources","recent","requirements","verify-prepare","prepare","read","inventory","approval","confirm","regenerate","preview","approve","cancel","recover","resume"] as const;
export type ImageSetRoute=typeof imageSetRoutes[number];
export const imageSetMethods:Record<ImageSetRoute,"GET"|"POST">={sources:"GET",recent:"GET",requirements:"POST","verify-prepare":"GET",prepare:"POST",read:"GET",inventory:"GET",approval:"GET",confirm:"POST",regenerate:"POST",preview:"POST",approve:"POST",cancel:"POST",recover:"POST",resume:"POST"};
export function imageSetPath(path:string[],action:ImageSetRoute):string|null {
 let rest:string[];
 if(path[0]==="sourcing"&&path[1]==="1688"&&path[2]==="acquisitions"&&isAcquisitionUUID(path[3]??"")&&path[4]==="images")rest=path.slice(5);
 else if(path[0]==="supply-preparations"&&path[1]==="sources"&&isAcquisitionUUID(path[2]??"")&&path[3]==="images")rest=path.slice(4);
 else return null;
 const match=action==="approval"?rest.length===4&&rest[0]==="runs"&&isAcquisitionUUID(rest[1]??"")&&rest[2]==="approvals"&&isAcquisitionUUID(rest[3]??""):action==="verify-prepare"?rest.length===2&&rest[0]==="by-key"&&isAcquisitionUUID(rest[1]??""):action==="sources"||action==="prepare"||action==="requirements"?rest.length===1&&rest[0]===action:action==="recent"?rest.length===1&&rest[0]==="runs":action==="read"?rest.length===2&&rest[0]==="runs"&&isAcquisitionUUID(rest[1]??""):rest.length===3&&rest[0]==="runs"&&isAcquisitionUUID(rest[1]??"")&&rest[2]===action;
 return match?path.join("/"):null;
}
export function imageSetRequestSchema(action:ImageSetRoute):z.ZodType {
 if(action==="prepare"||action==="regenerate"||action==="requirements")return imageSetPrepareSchema;
 if(action==="preview"||action==="approve")return imageSetSelectionSchema;
 if(action==="confirm")return z.strictObject({actionId:uuid,planRevision:positive,planDigest:hash,quoteDigest:hash});
 if(action==="recover")return z.strictObject({actionId:uuid,planRevision:positive,slotId:id,attempt:positive});
 if(action==="resume")return z.strictObject({actionId:uuid});
 if(action==="cancel")return z.strictObject({actionId:uuid,planRevision:positive});
 return z.never();
}
export function imageSetResponseSchema(action:ImageSetRoute):z.ZodType {
 if(action==="sources")return imageSetSourcesSchema;
 if(action==="requirements")return imageSetRequirementsSchema;
 if(action==="recent")return imageSetRecentSchema;
 if(action==="inventory")return imageSetInventorySchema;
 if(action==="approval")return imageSetApprovalSchema;
 if(action==="preview")return imageSetPreviewSchema;
 if(action==="prepare"||action==="regenerate"||action==="read"||action==="verify-prepare")return imageSetRunSchema;
 return imageSetAcceptedSchema;
}
export const imageSetSuccessStatus=(action:ImageSetRoute)=>action==="prepare"||action==="regenerate"?201:action==="preview"||action==="requirements"||imageSetMethods[action]==="GET"?200:202;
export const imageSetErrorStatuses:Record<string,number>={INVALID_IMAGE_REQUEST:400,INVALID_IMAGE_SELECTION:400,INVALID_REQUEST:400,FORBIDDEN:403,IMAGE_NOT_FOUND:404,IMAGE_CONFLICT:409,IMAGE_BLOCKED:409,IMAGE_SELECTION_CHANGED:409,IMAGE_ASSETS_NOT_READY:409,IMAGE_AGENT_DISABLED:409,IMAGE_CONFIGURATION_CHANGED:409,IMAGE_UNAVAILABLE:503,OUTCOME_UNKNOWN:503,IDENTITY_CONTEXT_CHANGED:409,ORGANIZATION_CONTEXT_CHANGED:409};

export const imageSetRequirementsSchema=z.object({groups:z.array(z.object({group:z.enum(["spu","skc","sku","detail"]),skc:integer,sku:integer,types:z.array(z.object({type:positive,minimum:integer,maximum:integer,nativeCompatible:z.boolean()})).max(20)})).max(10000),platform:z.enum(["product","shein"]),site:z.string(),categoryId:integer,version:z.string(),nativeWidth:positive,nativeHeight:positive});
export type ImageSetRequirements=z.infer<typeof imageSetRequirementsSchema>;

export const imageSetApprovalSchema=z.object({actionId:uuid,selectionDigest:hash,assets:z.array(approved).min(1).max(40)});

export const imageSetResponseBindingSchema=z.strictObject({kind:z.enum(["acquisition","supply"]),contextId:uuid,runId:uuid.optional(),approvalId:uuid.optional()});
export type ImageSetResponseBinding=z.infer<typeof imageSetResponseBindingSchema>;
export function imageSetResponseBinding(path:string[],action:ImageSetRoute):ImageSetResponseBinding{
 const acquisition=path[0]==="sourcing",tail=path.slice(acquisition?5:4);
 return {kind:acquisition?"acquisition":"supply",contextId:path[acquisition?3:2],...tail[0]==="runs"&&tail.length>1?{runId:tail[1]}:{},...action==="approval"?{approvalId:tail[3]}:{}};
}
export function imageSetResponseMatches(action:ImageSetRoute,data:unknown,expected:ImageSetResponseBinding):boolean{
 if(!data||typeof data!=="object")return false;
 const value=data as Record<string,unknown>;
 if(action==="sources"){
  const source=z.object({ContextKind:z.string(),OperationID:z.string()}).safeParse(value.source);
  return value.contextKind===expected.kind&&value.contextId===expected.contextId&&source.success&&source.data.ContextKind===expected.kind&&source.data.OperationID===expected.contextId;
 }
 if(action==="recent")return Array.isArray(value.items)&&value.items.every(item=>item.contextKind===expected.kind&&item.contextId===expected.contextId);
 if(action==="approval")return value.actionId===expected.approvalId;
 if("runId" in value&&!["prepare","regenerate","verify-prepare"].includes(action)&&value.runId!==expected.runId)return false;
 if("plan" in value){
  const plan=z.object({Source:z.object({ContextKind:z.string(),OperationID:z.string()})}).safeParse(value.plan);
  return plan.success&&plan.data.Source.ContextKind===expected.kind&&plan.data.Source.OperationID===expected.contextId;
 }
 return true;
}
