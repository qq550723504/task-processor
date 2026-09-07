// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { configuredProductReviewOrigin, hasTrustedReviewWriteOrigin, reviewSelectedOrganization } from "./product-title-review-request";

afterEach(() => vi.unstubAllEnvs());
const request = (headers: HeadersInit = {}) => new Request("https://untrusted-host.invalid/api/product/text-proposals", { method: "POST", headers });

it("requires a fixed server API origin without fallback or URL components", () => {
  vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", "");
  expect(configuredProductReviewOrigin()).toBeNull();
  for (const value of ["https://api.test/path", "https://user:secret@api.test", "https://api.test?x", "https://api.test#x", "file:///tmp/a"]) {
    vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", value);
    expect(configuredProductReviewOrigin()).toBeNull();
  }
  vi.stubEnv("PRODUCT_REVIEW_API_ORIGIN", "https://api.test/");
  expect(configuredProductReviewOrigin()).toBe("https://api.test");
});

it("trusts configured public origin, never Host or Forwarded, and rejects cross-site writes", () => {
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://console.test");
  expect(hasTrustedReviewWriteOrigin(request({ Origin: "https://console.test", "Sec-Fetch-Site": "same-origin" }))).toBe(true);
  expect(hasTrustedReviewWriteOrigin(request({ Origin: "https://console.test" }))).toBe(true);
  const invalidHeaders: HeadersInit[] = [
    {}, { Origin: "null" }, { Origin: "https://evil.test" },
    { Origin: "https://console.test", "Sec-Fetch-Site": "same-site" },
    { Origin: "https://console.test", "Sec-Fetch-Site": "cross-site" },
    { Origin: "https://evil.test", Host: "evil.test", Forwarded: "host=evil.test;proto=https", "X-Forwarded-Host": "evil.test" },
  ];
  for (const headers of invalidHeaders) expect(hasTrustedReviewWriteOrigin(request(headers))).toBe(false);
});

it("fails closed without a configured public origin", () => {
  for (const key of ["LISTINGKIT_PUBLIC_BASE_URL", "TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL", "NEXT_PUBLIC_APP_URL", "APP_URL"]) vi.stubEnv(key, "");
  expect(hasTrustedReviewWriteOrigin(request({ Origin: "http://localhost:3000" }))).toBe(false);
});

it("accepts only one canonical effective-org cookie, regardless of authority headers", () => {
  expect(reviewSelectedOrganization(request({ Cookie: "shuomi_effective_organization=B", "X-Requested-Organization-ID": "A" }))).toBe("B");
  for (const cookie of ["", "shuomi_effective_organization=", "shuomi_effective_organization=%20B", "shuomi_effective_organization=%xx", "shuomi_effective_organization=B; shuomi_effective_organization=A"]) {
    expect(reviewSelectedOrganization(request({ Cookie: cookie }))).toBeNull();
  }
});
