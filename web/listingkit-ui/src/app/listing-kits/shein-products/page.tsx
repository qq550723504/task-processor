import { Suspense } from "react";

import { SheinProductsDashboardPage } from "@/components/listingkit/shein-products/shein-products-dashboard-page";

export default function SheinProductsRoute() {
  return (
    <Suspense>
      <SheinProductsDashboardPage />
    </Suspense>
  );
}
