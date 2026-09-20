import { afterEach, expect, it, vi } from "vitest";
import { isProductAcquisitionAvailable } from "./product-acquisition-availability";

afterEach(() => vi.unstubAllEnvs());

it.each([undefined, "", "false", "TRUE", " true ", "1"])("fails closed unless the explicit deployment switch is exactly true: %j", value => {
  if (value === undefined) vi.stubEnv("LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", undefined);
  else vi.stubEnv("LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", value);
  expect(isProductAcquisitionAvailable()).toBe(false);
});

it("opens only when the serving deployment explicitly enables acquisition", () => {
  vi.stubEnv("LISTINGKIT_PRODUCT_ACQUISITION_ENABLED", "true");
  expect(isProductAcquisitionAvailable()).toBe(true);
});
