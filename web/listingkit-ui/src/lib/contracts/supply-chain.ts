import { z } from "zod";
import { collectionID, collectionItemSchema, collectionDetailSchema } from "./product-collection";
import { isAcquisitionUUID } from "./product-acquisition";

export const SUPPLY_MAX_BYTES=2*1024*1024;
const text=(max:number)=>z.string().refine(v=>new TextEncoder().encode(v).length<=max && !v.includes("\0"));
const integer=z.number().int().min(0).max(Number.MAX_SAFE_INTEGER);
const revision=integer.min(1);
const time=z.iso.datetime({offset:true});
const version=z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v=>BigInt(v)<=BigInt("9223372036854775807"));
const list=<T extends z.ZodType>(value:T,max=4096)=>z.array(value).max(max).nullable().transform(v=>v??[]);
const page=<T extends z.ZodType>(value:T)=>z.object({items:list(value,100),total:integer,nextCursor:collectionID.optional()});
export const applicationModeSchema=z.enum(["self_operated","semi_managed","fully_managed"]);
const language=z.object({language:text(32),name:text(1024*1024)}).strict();
const attribute=z.object({attribute_id:integer,attribute_value_id:integer.optional(),attribute_extra_value:text(8192).optional(),custom_attribute_value:text(8192).optional(),language:text(32).optional()}).strict();
const image=z.object({image_sort:integer,image_type:integer,image_url:text(2048)}).strict();
const imageInfo=z.object({image_info_list:list(image,40)}).strict();
const quantity=z.object({quantity_type:integer,quantity_unit:integer,quantity:integer}).strict();
export const officialSKUSchema=z.object({supplier_sku:text(200),length:text(32).optional(),width:text(32).optional(),height:text(32).optional(),weight:z.number().finite().optional(),weight_unit:text(32).optional(),length_width_height_unit:text(32).optional(),mall_state:integer,
 price_info_list:list(z.object({base_price:z.number().finite(),currency:text(16),sub_site:text(128)}).strict(),20).optional(),stock_info_list:list(z.object({inventory_num:integer,supplier_warehouse_id:text(128).optional(),supplier_warehouse_name:text(256).optional()}).strict(),20),sale_attribute_list:list(attribute,256),sku_scope_attribute_list:list(attribute,256).optional(),image_info:imageInfo.optional(),quantity_info:quantity.optional(),package_type:text(128).optional(),competing_product_link:text(2048).optional(),cost_info:z.object({cost_price:text(32),currency:text(16)}).strict().optional(),minimum_stock_quantity:text(32).optional(),stop_purchase:integer.optional()}).strict();
export const officialSKCSchema=z.object({supplier_code:text(200).optional(),sale_attribute:attribute,image_info:imageInfo,skc_multi_language_name_list:list(language,32).optional(),shelf_way:text(128).optional(),shelf_require:text(8192).optional(),hope_on_sale_date:text(64).optional(),suggested_retail_price:z.object({price:z.number().finite(),currency:text(16)}).strict().optional(),site_detail_image_info_list:list(z.object({site_abbr_list:list(text(128),20),image_info_list:list(z.object({image_sort:integer,image_url:text(2048)}).strict(),40)}).strict(),20).optional(),proof_of_stock_list:list(z.object({file_name:text(256),type:text(64),url:text(2048)}).strict(),40).optional(),sku_list:list(officialSKUSchema,1000)}).strict();
export const officialProductSchema=z.object({category_id:integer,product_type_id:integer,brand_code:text(128).optional(),source_system:text(128),suit_flag:text(32),is_spu_pic:z.boolean(),supplier_code:text(200).optional(),multi_language_name_list:list(language,32),multi_language_desc_list:list(language,32).optional(),product_attribute_list:list(attribute,256),site_list:list(z.object({main_site:text(128),sub_site_list:list(text(128),20)}).strict(),20).optional(),skc_list:list(officialSKCSchema,40),image_info:imageInfo.optional(),size_attribute_list:list(z.object({attribute_id:integer,attribute_extra_value:text(8192),relate_sale_attribute_id:integer.optional(),relate_sale_attribute_value_id:integer.optional()}).strict(),256).optional(),
 sample_info:z.object({sample_spec:z.object({main_spec:z.object({attribute_id:integer,attribute_value_id:integer}).strict(),sub_spec_list:list(z.object({attribute_id:text(128),attribute_value_id:text(128)}).strict(),256)}).strict(),sample_judge_type:integer,reserve_sample_flag:integer,spot_flag:integer}).strict().optional(),fill_configuration_info:z.object({filled_quantity_to_sku:z.boolean()}).strict().optional(),fill_configuration_tags:list(text(256),256).optional()}).strict();
const imageSlot=z.object({group:z.enum(["spu","skc","sku","detail"]),skc:integer,sku:integer,asset_id:text(128).min(1),sort:integer,type:integer}).strict();
export const officialDraftInputSchema=z.object({product:officialProductSchema,images:list(imageSlot,10000)}).strict();
export const targetInputSchema=z.object({sourceId:collectionID,storeId:collectionID,expectedRevision:integer,effectiveVersion:version,applyReceiptId:collectionID.optional(),draft:officialDraftInputSchema}).strict();
export const merchantSchema=z.object({organization_id:text(128),store_id:collectionID,site:z.literal("shein-us"),store_version:revision,connection_revision:revision,application_revision:text(128),application_id:text(128),application_type:applicationModeSchema,supplier_identity_hash:z.string().regex(/^[0-9a-f]{64}$/),service_expires_at:time});
export const preparationSchema=z.object({id:collectionID,sourceBatchId:collectionID,sourceRevision:revision,name:text(200),count:integer,revision,createdAt:time});
export const sourceSchema=z.object({id:collectionID,preparationId:collectionID,collectionItemId:collectionID,collectionRevision:revision,source:collectionItemSchema.shape.source});
export const sourceDetailSchema=z.object({source:sourceSchema,product:collectionDetailSchema.shape.product,images:list(z.object({id:collectionID,url:text(2048),width:integer,height:integer}),4096)});
export const targetRecordSchema=z.object({id:collectionID,targetId:collectionID,revision,source:sourceSchema,effectiveVersion:version,applyReceiptId:collectionID.optional(),productHash:z.string().regex(/^[0-9a-f]{64}$/),inventoryHash:z.string().regex(/^[0-9a-f]{64}$/),rulesHash:z.string().regex(/^[0-9a-f]{64}$/),merchant:merchantSchema,input:targetInputSchema,result:z.object({product:officialProductSchema,images:list(imageSlot,10000),issues:list(z.object({field:text(8192),code:text(128),message:text(8192)}),4096),ready_for_upload:z.boolean()}),createdAt:time});
export const targetReceiptSchema=z.object({record:targetRecordSchema,replayed:z.boolean()});
export const transferInputSchema=z.object({batchId:collectionID,expectedRevision:revision,itemIds:z.array(collectionID).max(1000).optional()}).strict();
export const transferReceiptSchema=z.object({preparation:preparationSchema,replayed:z.boolean()});
export const sourceSelectionSchema=z.object({itemId:collectionID,originalPublicationId:text(128).min(1),originalSnapshotVersion:revision,effectiveCatalogVersion:revision,applyReceiptId:collectionID.optional(),targetPlatform:z.literal("shein")}).strict();
const role=z.enum(["main","white_background","gallery","design"]);
export const imageApprovalSchema=z.object({actionId:collectionID.optional(),selection:sourceSelectionSchema,images:z.array(z.object({id:collectionID,role}).strict()).max(40),approved:z.array(z.object({actionId:text(128),assetId:text(128)}).strict()).max(40)}).strict().refine(v=>v.images.length+v.approved.length>0 && v.images.length+v.approved.length<=40);
export const imageApprovalReceiptSchema=z.object({action_id:collectionID,asset_ids:z.array(text(128)).min(1).max(40)});
export const inventorySchema=z.object({scope:z.object({tenant_id:text(128),product_key:text(128),target_platform:z.literal("shein"),source_snapshot_version:revision}),assets:list(z.object({id:text(128).min(1),role,url:text(2048),width:integer.optional(),height:integer.optional(),source_asset_id:text(128).optional()}),40)});
export const operationInputSchema=z.object({preparationId:collectionID,expectedRevision:revision,action:z.enum(["adapt","upload","optimize"]),storeId:collectionID,sourceIds:z.array(collectionID).max(1000).optional(),categoryId:integer.optional(),titleTemplateId:text(128).optional(),imageTemplateId:text(128).optional()}).strict().refine(v=>v.action==="optimize"?!!(v.titleTemplateId||v.imageTemplateId):!v.titleTemplateId&&!v.imageTemplateId).refine(v=>v.action!=="upload"||!v.categoryId);
export const operationSchema=z.object({id:collectionID,input:operationInputSchema,count:integer,completed:integer,status:z.enum(["pending","running","completed","cancelled"]),execution:z.enum(["pending","started","start_unknown"]),createdAt:time}).refine(v=>v.completed<=v.count);
export const operationReceiptSchema=z.object({operation:operationSchema,replayed:z.boolean()});
export const operationItemSchema=z.object({sourceId:collectionID,recordId:collectionID.optional(),recordRevision:revision.optional(),status:z.enum(["pending","running","succeeded","missing","review","unknown","denied","failed","cancelled"]),resultReference:text(2048).optional(),note:text(8192).optional()});
type Category={category_id:number;product_type_id:number;parent_category_id:number;category_name:string;last_category:boolean|null;children:Category[]};
const category:z.ZodType<Category>=z.lazy(()=>z.object({category_id:integer,product_type_id:integer,parent_category_id:integer,category_name:text(8192),last_category:z.boolean().nullable(),children:list(category,4096)}));
const nullableFlag=z.number().int().nullable();
const ruleAttribute=z.object({attribute_id:integer,attribute_name:text(8192),attribute_is_show:nullableFlag,attribute_type:nullableFlag,attribute_label:nullableFlag,attribute_mode:nullableFlag,attribute_input_num:nullableFlag,attribute_status:nullableFlag,data_dimension:nullableFlag,value_info_list:list(z.object({attribute_value_id:integer,attribute_value:text(8192),is_show:nullableFlag,is_custom_attribute_value:nullableFlag,supplier_id:integer})),rule_info_list:list(z.object({id:integer,condition_operator:nullableFlag,condition_type:nullableFlag,value:text(8192)}))});
export const rulesSchema=z.object({merchant:merchantSchema,rules:z.object({application_type:applicationModeSchema,categories:list(category),brands:list(z.object({brand_code:text(128),brand_name:text(8192)})),sites:list(z.object({main_site:text(128),main_site_name:text(8192),sub_site_list:list(z.object({site_abbr:text(128),site_name:text(8192),site_status:nullableFlag,store_type:nullableFlag,currency:text(16)}),256)}),256),warehouses:list(z.object({warehouseCode:text(128),warehouseName:text(256),saleCountryList:list(text(2),256),warehouseType:z.union([z.literal(1),z.literal(2)])}),20),
 fill:z.object({fill_in_standard_list:list(z.object({field_key:text(128),module:text(128),required:z.boolean().nullable(),show:z.boolean().nullable()})),currency:text(16).nullable(),default_language:text(32),default_language_title_max_length:integer.nullable(),language_title_max_length_list:list(z.object({language:text(32),max_length:z.number().finite()})),picture_config_list:list(z.object({field_key:text(128),is_true:z.boolean().nullable()})),supplier_code_in_spu_dimension:z.boolean().nullable(),support_duplicate_sale_attr_value:z.boolean().nullable()}),attributes:z.object({product_type_id:integer,main_attribute_status:nullableFlag,attribute_infos:list(ruleAttribute)}),linked:list(z.object({group_id:text(128),link_rule_attribute_list:list(z.object({attribute_id:integer,attribute_value_list:list(integer),attribute_value_pre_fill_list:list(integer)}))}))})});
export const preparationPageSchema=page(preparationSchema);
export const sourcePageSchema=page(sourceSchema);
export const operationItemPageSchema=page(operationItemSchema);
export const operationPageSchema=page(operationSchema);
export const publicationSchema=z.object({sourceId:collectionID,storeId:collectionID,recordId:collectionID.optional(),receiptId:text(128).optional(),observedAt:time.optional(),product:z.object({spu_name:text(128).min(1),skc_list:list(z.object({skc_name:text(128).min(1),sku_list:list(z.object({sku_code:text(128).min(1),supplier_sku:text(200).min(1)}),400)}),40)}).optional()}).refine(v=>v.product?!!v.receiptId&&!!v.observedAt:!v.receiptId&&!v.observedAt&&!v.recordId);
export type SupplyRoute="publication"|"preparation"|"operations"|"list"|"transfer"|"transfer-read"|"sources"|"source"|"target"|"save-target"|"rules"|"target-command"|"record"|"approve"|"inventory"|"create-operation"|"operation-key"|"operation"|"operation-items"|"ensure"|"cancel";
export const supplyRouteMethods:ReadonlyArray<readonly["GET"|"POST",SupplyRoute]>=[["GET","publication"],["GET","preparation"],["GET","operations"],["GET","list"],["POST","transfer"],["GET","transfer-read"],["GET","sources"],["GET","source"],["GET","target"],["POST","save-target"],["POST","rules"],["GET","target-command"],["GET","record"],["POST","approve"],["POST","inventory"],["POST","create-operation"],["GET","operation-key"],["GET","operation"],["GET","operation-items"],["POST","ensure"],["POST","cancel"]];
export function supplyPath(method:string,p:string[]):SupplyRoute|null{
 if(p[0]!=="supply-preparations")return null;
 if(method==="GET"){
  if(p.length===1)return "list";
  if(p.length===2 && isAcquisitionUUID(p[1]!))return "preparation";
  if(p.length===3 && isAcquisitionUUID(p[1]!) && p[2]==="operations")return "operations";
  if(p.length===3 && isAcquisitionUUID(p[1]!) && p[2]==="sources")return "sources";
  if(p.length===3 && isAcquisitionUUID(p[2]!))return p[1]==="sources"?"source":p[1]==="records"?"record":p[1]==="target-commands"?"target-command":p[1]==="operations"?"operation":null;
  if(p.length===4 && p[2]==="by-key" && isAcquisitionUUID(p[3]!))return p[1]==="transfers"?"transfer-read":p[1]==="operations"?"operation-key":null;
  if(p.length===4 && p[1]==="operations" && isAcquisitionUUID(p[2]!) && p[3]==="items")return "operation-items";
  if(p.length===5 && p[1]==="sources" && isAcquisitionUUID(p[2]!) && isAcquisitionUUID(p[4]!))return p[3]==="targets"?"target":p[3]==="publications"?"publication":null;
 }
 if(method==="POST"){
  if(p.length===2)return p[1]==="transfer"?"transfer":p[1]==="targets"?"save-target":p[1]==="target-rules"?"rules":p[1]==="operations"?"create-operation":null;
  if(p.length===3 && p[1]==="images")return p[2]==="approve"?"approve":p[2]==="inventory"?"inventory":null;
  if(p.length===4 && p[1]==="operations" && isAcquisitionUUID(p[2]!))return p[3]==="ensure"?"ensure":p[3]==="cancel"?"cancel":null;
 }
 return null;
}
export const supplyRequestSchema=(route:SupplyRoute)=>route==="transfer"?transferInputSchema:route==="save-target"||route==="rules"?targetInputSchema:route==="approve"?imageApprovalSchema:route==="inventory"?sourceSelectionSchema:route==="create-operation"?operationInputSchema:null;
export const supplyUsesKey=(route:SupplyRoute)=>["transfer","save-target","approve","create-operation"].includes(route);
export const supplyMutates=(route:SupplyRoute)=>supplyUsesKey(route)||route==="ensure"||route==="cancel";
export function supplyResponseSchema(route:SupplyRoute):z.ZodType {
 return route==="publication"?publicationSchema:route==="preparation"?preparationSchema:route==="operations"?operationPageSchema:route==="list"?preparationPageSchema:route==="sources"?sourcePageSchema:route==="source"?sourceDetailSchema:route==="transfer"||route==="transfer-read"?transferReceiptSchema:route==="target"||route==="record"?targetRecordSchema:route==="save-target"||route==="target-command"?targetReceiptSchema:route==="rules"?rulesSchema:route==="approve"?imageApprovalReceiptSchema:route==="inventory"?inventorySchema:route==="create-operation"?operationReceiptSchema:route==="operation-items"?operationItemPageSchema:operationSchema;
}
export type SupplyPreparation=z.infer<typeof preparationSchema>;
export type SupplySource=z.infer<typeof sourceSchema>;
export type SupplySourceDetail=z.infer<typeof sourceDetailSchema>;
export type SupplyTarget=z.infer<typeof targetRecordSchema>;
export type SupplyTargetInput=z.infer<typeof targetInputSchema>;
export type SupplyRules=z.infer<typeof rulesSchema>;
export type SupplyOperation=z.infer<typeof operationSchema>;
export type SupplyOperationItem=z.infer<typeof operationItemSchema>;
export type OfficialProduct=z.infer<typeof officialProductSchema>;

export function parseSupplyResponse(route:SupplyRoute,payload:unknown,expected?:string):unknown|null {
 const result=supplyResponseSchema(route).safeParse(payload);if(!result.success)return null;
 if(expected){
  if(route==="preparation" && preparationSchema.parse(result.data).id!==expected)return null;
  if(route==="source" && sourceDetailSchema.parse(result.data).source.id!==expected)return null;
  if(route==="record" && targetRecordSchema.parse(result.data).id!==expected)return null;
  if((route==="operation"||route==="ensure"||route==="cancel") && operationSchema.parse(result.data).id!==expected)return null;
  if(route==="publication"){const value=publicationSchema.parse(result.data);if(`${value.sourceId}:${value.storeId}`!==expected)return null;}
  if(route==="target"){const value=targetRecordSchema.parse(result.data);if(`${value.source.id}:${value.merchant.store_id}`!==expected)return null;}
 }
 return result.data;
}

export type SupplyPublication=z.infer<typeof publicationSchema>;
