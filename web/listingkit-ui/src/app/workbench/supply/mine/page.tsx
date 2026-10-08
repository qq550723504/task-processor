import { connection } from "next/server";
import { notFound } from "next/navigation";
import { SupplyPage } from "@/components/workbench/supply/supply-page";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";
export default async function Page({searchParams}:{searchParams:Promise<Record<string,string|string[]|undefined>>}){
 await connection();
 if(process.env.LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED!=="true"||process.env.LISTINGKIT_SUPPLY_CHAIN_ENABLED!=="true")notFound();
 const params=await searchParams;
 const read=(name:string)=>{const value=params[name];if(value===undefined)return undefined;if(typeof value!=="string"||!isAcquisitionUUID(value))notFound();return value};
 return <SupplyPage initialPreparation={read("preparation")} initialStore={read("store")} initialOperation={read("operation")}/>;
}
