import {connection} from "next/server";
import {notFound} from "next/navigation";
import {SupplyApplicationForm} from "@/components/workbench/supply-market/applications";
export default async function Page(){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true")notFound();return <SupplyApplicationForm/>}
