import { Suspense } from "react";

import { CanonicalProductListPage } from "@/components/listingkit/canonical/canonical-product-list-page";

export default function ListingKitCanonicalProductsPage() {
  return (
    <Suspense>
      <CanonicalProductListPage />
    </Suspense>
  );
}
