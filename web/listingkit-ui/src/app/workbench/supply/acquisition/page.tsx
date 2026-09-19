import { connection } from "next/server";
import { notFound } from "next/navigation";
import { AcquisitionPage } from "@/components/workbench/acquisition/acquisition-page";
import { isProductAcquisitionAvailable } from "@/lib/server/product-acquisition-availability";

export default async function Page() {
  await connection();
  if (!isProductAcquisitionAvailable()) notFound();
  return <AcquisitionPage />;
}
