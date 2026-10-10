import {connection} from "next/server";
import {notFound} from "next/navigation";
import {PODCatalogPage} from "@/components/workbench/pod/catalog";
export default async function Page(){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true"||process.env.LISTINGKIT_SDS_POD_ENABLED!=="true")notFound();return <PODCatalogPage/>}
