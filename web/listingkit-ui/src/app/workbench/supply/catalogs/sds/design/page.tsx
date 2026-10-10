import {connection} from "next/server";
import {notFound} from "next/navigation";
import {PODDesignPage} from "@/components/workbench/pod/design";
import {podID} from "@/lib/contracts/pod";
export default async function Page({searchParams}:{searchParams:Promise<Record<string,string|string[]|undefined>>}){await connection();if(process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED!=="true"||process.env.LISTINGKIT_SDS_POD_ENABLED!=="true")notFound();const q=await searchParams;if(Object.keys(q).some(k=>k!=="template"&&k!=="variant")||q.template!==undefined&&!podID.safeParse(q.template).success||q.variant!==undefined&&(typeof q.variant!=="string"||!/^\d{1,64}$/.test(q.variant)))notFound();return <PODDesignPage templateID={q.template as string|undefined} variantID={q.variant as string|undefined}/>}
