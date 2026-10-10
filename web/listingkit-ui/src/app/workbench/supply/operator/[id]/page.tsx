import {connection} from "next/server";
import {notFound} from "next/navigation";
import {SupplyRecordPage} from "@/components/workbench/supply-market/record";
import {isAcquisitionUUID} from "@/lib/contracts/product-acquisition";
import {requireAccountUserId} from "@/lib/server/account-page-auth";
export default async function Page({params}:{params:Promise<{id:string}>}){await connection();const {id}=await params;if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true"||!isAcquisitionUUID(id))notFound();const expectedUserId=await requireAccountUserId("/workbench/supply/operator/"+id);return <SupplyRecordPage id={id} admin expectedUserId={expectedUserId}/>}
