import { Suspense } from "react";

import { SheinEnrollmentDashboardPage } from "@/components/listingkit/shein-enrollment/shein-enrollment-dashboard-page";

export default function SheinProductsRoute() {
  return (
    <Suspense>
      <SheinEnrollmentDashboardPage mode="products" />
    </Suspense>
  );
}
