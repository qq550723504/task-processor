import type { OfficialProduct,SupplySourceDetail,SupplyTargetInput } from "@/lib/contracts/supply-chain";
export type SupplyDraft=SupplyTargetInput["draft"];
export const emptySKU=()=>({supplier_sku:"",mall_state:0,stock_info_list:[],sale_attribute_list:[]});
export const emptySKC=()=>({supplier_code:"",sale_attribute:{attribute_id:0},image_info:{image_info_list:[]},sku_list:[emptySKU()]});
export function initialSupplyDraft(source:SupplySourceDetail):SupplyDraft{
 const product=source.product;
 return {product:{category_id:0,product_type_id:0,source_system:"OpenAPI",suit_flag:"0",is_spu_pic:false,multi_language_name_list:[{language:"en",name:product.title??""}],multi_language_desc_list:[{language:"en",name:product.description??""}],product_attribute_list:[],skc_list:[{...emptySKC(),sku_list:product.variants?.length?product.variants.map(v=>({...emptySKU(),supplier_sku:v.sku??""})):[emptySKU()]}]},images:[]};
}
export function applyApplicationMode(value:SupplyDraft,mode:"self_operated"|"semi_managed"|"fully_managed"):SupplyDraft{
 const draft=structuredClone(value);
 if(mode==="fully_managed")delete draft.product.site_list;else draft.product.site_list=[{main_site:"shein",sub_site_list:["shein-us"]}];
 for(const skc of draft.product.skc_list)for(const sku of skc.sku_list){delete sku.price_info_list;delete sku.cost_info;delete sku.stop_purchase;sku.stock_info_list=[]}
 return draft;
}
export type SKU=OfficialProduct["skc_list"][number]["sku_list"][number];
export type AttributeValue=OfficialProduct["product_attribute_list"][number];
export function sampleForSKU(draft:SupplyDraft,skcIndex:number,skuIndex:number):OfficialProduct["sample_info"]{
 const skc=draft.product.skc_list[skcIndex],sku=skc?.sku_list[skuIndex];
 const main=skc?.sale_attribute,sub=sku?.sale_attribute_list??[];
 if(!sku||!main||!main.attribute_id||!main.attribute_value_id||sub.length>2||sub.some(s=>!s.attribute_id||!s.attribute_value_id))return;
 return {sample_judge_type:2,reserve_sample_flag:2,spot_flag:0,sample_spec:{main_spec:{attribute_id:main.attribute_id,attribute_value_id:main.attribute_value_id},sub_spec_list:sub.map(s=>({attribute_id:String(s.attribute_id),attribute_value_id:String(s.attribute_value_id)}))}};
}
export function categoryLeaves(nodes:{category_id:number;product_type_id:number;category_name:string;last_category:boolean|null;children:typeof nodes}[]):{id:number;type:number;name:string}[]{
 return nodes.flatMap(n=>n.last_category===true?[{id:n.category_id,type:n.product_type_id,name:n.category_name}]:categoryLeaves(n.children));
}
