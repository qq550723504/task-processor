// This is a serving-deployment assertion, not an authorization decision. The
// Go runtime still mounts acquisition only when its independent database is set.
export function isProductAcquisitionAvailable(): boolean {
  return process.env.LISTINGKIT_PRODUCT_ACQUISITION_ENABLED === "true";
}
