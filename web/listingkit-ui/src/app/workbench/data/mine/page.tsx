import { connection } from "next/server";
import { notFound } from "next/navigation";
import { CollectionPage } from "@/components/workbench/collections/collection-page";

export default async function Page() {
  await connection();
  if (process.env.LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED !== "true") notFound();
  return <CollectionPage supplyAvailable={process.env.LISTINGKIT_SUPPLY_CHAIN_ENABLED === "true"} />;
}
