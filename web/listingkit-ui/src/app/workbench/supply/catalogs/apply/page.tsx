import {connection} from "next/server";
import {notFound} from "next/navigation";
import {SupplyCatalogsPage} from "@/components/workbench/supply-market/catalogs";
export default async function Page(){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true")notFound();return <SupplyCatalogsPage view="apply"/>}
