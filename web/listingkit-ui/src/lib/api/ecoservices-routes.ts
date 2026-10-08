import { z } from "zod";
import {ecoId,ecoPageSchema,ecoResultSchema,ecoCheckoutSchema,ecoFileSchema,ecoInputs,ecoMerchantSchema,ecoFinancialSchema,ecoMerchantInput} from "@/lib/api/ecoservices";
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
