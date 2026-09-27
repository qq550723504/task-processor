import { notFound } from "next/navigation";
import { connection } from "next/server";
import { AcquisitionPage } from "@/components/workbench/acquisition/acquisition-page";
import { isProductAcquisitionAvailable } from "@/lib/server/product-acquisition-availability";

export default async function Page({ params }: { params: Promise<{ operation_id: string }> }) {
  await connection();
  if (!isProductAcquisitionAvailable()) notFound();
  const { operation_id } = await params;
  return <AcquisitionPage operationId={operation_id} agentEnabled={process.env.LISTINGKIT_PRODUCT_AGENT_ENABLED==="true"} />;
}
