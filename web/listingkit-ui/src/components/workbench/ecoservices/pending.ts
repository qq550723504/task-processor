import {z} from "zod";
import {ecoId,ecoMerchantSchema,ecoResultSchema,ecoCheckoutSchema,ecoFinancialSchema} from "@/lib/api/ecoservices";
import {ecoservicesEndpoint} from "@/lib/api/ecoservices-routes";

// The BFF's same allowlist validates routing, not authorization. No merchant
// PII, upload bytes, response URLs or credentials are saved in this command.
export function intentRoute(i:{path:string;admin?:boolean;method?:"POST"|"PUT"}){
 return ecoservicesEndpoint(new URL((i.admin?"/api/admin/ecoservices/":"/api/ecoservices/")+i.path,"https://pending.invalid"),i.method??"POST");
}
export const ecoPendingSchema=z.object({
 path:z.string().max(256),key:ecoId,body:z.string().max(131072),
 version:z.string().regex(/^[1-9][0-9]{0,18}$/).refine(v=>BigInt(v)<=BigInt("9223372036854775807")).optional(),
 output:z.enum(["checkout","financial"]).optional(),admin:z.boolean().optional(),method:z.enum(["POST","PUT"]).optional(),
}).strict().refine(i=>{
 const r=intentRoute(i);
 if(!r||r.path!==i.path||r.admin!==!!i.admin||!r.input||r.upload||r.output===ecoMerchantSchema||r.cas&&!i.version)return false;
 const output=i.output==="checkout"?ecoCheckoutSchema:i.output==="financial"?ecoFinancialSchema:ecoResultSchema;
 if(r.output!==output)return false;
 try{return r.input.safeParse(JSON.parse(i.body)).success}catch{return false}
},"Invalid original ecosystem command");
