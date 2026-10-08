import { connection } from "next/server";
import { notFound } from "next/navigation";
import { SupplyPage } from "@/components/workbench/supply/supply-page";
import { stageSchema } from "@/lib/contracts/supply-chain";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
export default async function Page({params,searchParams}:{params:Promise<{stage:string}>;searchParams:Promise<Record<string,string|string[]|undefined>>}){
 await connection();
 if(process.env.LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED!=="true"||process.env.LISTINGKIT_SUPPLY_CHAIN_ENABLED!=="true")notFound();
 const stage=stageSchema.safeParse((await params).stage);if(!stage.success||stage.data==="all")notFound();
 const query=await searchParams;const read=(name:string)=>{const value=query[name];if(value===undefined)return undefined;if(typeof value!=="string"||!isAcquisitionUUID(value))notFound();return value};
 return <SupplyPage initialStage={stage.data} initialPreparation={read("preparation")} initialStore={read("store")} initialOperation={read("operation")}/>;
}
