import { Suspense } from "react";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

export default async function SheinEnrollmentStoreRoute({
  params,
  searchParams,
}: {
  params: Promise<{ storeId: string }>;
  searchParams: Promise<{ tab?: string; activityType?: string }>;
}) {
  const [{ storeId }, { tab, activityType }] = await Promise.all([
    params,
    searchParams,
  ]);

  return (
    <Suspense>
      <SheinEnrollmentStoreWorkbench
        storeId={Number(storeId)}
        initialActivityType={activityType}
        initialTab={tab}
      />
    </Suspense>
  );
}
