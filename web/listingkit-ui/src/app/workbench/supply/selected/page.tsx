import {connection} from "next/server";
import {notFound} from "next/navigation";
import {SupplyMarketPage} from "@/components/workbench/supply-market/market";
export default async function Page(){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true")notFound();return <SupplyMarketPage channel="selected"/>}
