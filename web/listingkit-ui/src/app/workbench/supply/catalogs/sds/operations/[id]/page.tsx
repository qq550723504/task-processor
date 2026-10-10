import {connection} from "next/server";
import {notFound} from "next/navigation";
import {PODOperationPage} from "@/components/workbench/pod/operation";
import {podID} from "@/lib/contracts/pod";
export default async function Page({params}:{params:Promise<{id:string}>}){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true"||process.env.LISTINGKIT_SDS_POD_ENABLED!=="true")notFound();const {id}=await params;if(!podID.safeParse(id).success)notFound();return <PODOperationPage id={id}/>}
