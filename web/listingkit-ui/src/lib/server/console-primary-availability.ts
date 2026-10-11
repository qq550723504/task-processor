import { isProductAcquisitionAvailable } from "./product-acquisition-availability";

// Same serving assertions as RootLayout and the destination pages, never grants.
export function consolePrimaryDeploymentCapabilities() {
  return {
    productAcquisitionAvailable: isProductAcquisitionAvailable(),
    supplyMarketAvailable: process.env.LISTINGKIT_SUPPLY_MARKET_ENABLED === "true",
    supplyChainAvailable: process.env.LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED === "true" && process.env.LISTINGKIT_SUPPLY_CHAIN_ENABLED === "true",
    dataServicesAvailable: process.env.LISTINGKIT_DATA_SERVICES_ENABLED === "true",
    productCollectionsAvailable: process.env.LISTINGKIT_PRODUCT_COLLECTIONS_ENABLED === "true",
  };
}
