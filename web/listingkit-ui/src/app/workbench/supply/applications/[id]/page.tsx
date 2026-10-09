import {connection} from "next/server";
import {notFound} from "next/navigation";
import {SupplyRecordPage} from "@/components/workbench/supply-market/record";
import {isAcquisitionUUID} from "@/lib/contracts/product-acquisition";
export default async function Page({params}:{params:Promise<{id:string}>}){await connection();const {id}=await params;if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true"||!isAcquisitionUUID(id))notFound();return <SupplyRecordPage id={id}/>}
