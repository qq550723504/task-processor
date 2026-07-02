import { Suspense } from "react";

import { SheinProductsStoreWorkbench } from "@/components/listingkit/shein-products/shein-products-store-workbench";

export default async function SheinProductsStoreRoute({
  params,
  searchParams,
}: {
  params: Promise<{ storeId: string }>;
  searchParams: Promise<{ tab?: string }>;
}) {
  const [{ storeId }, { tab }] = await Promise.all([params, searchParams]);

  return (
    <Suspense>
      <SheinProductsStoreWorkbench storeId={Number(storeId)} initialTab={tab} />
    </Suspense>
  );
}
