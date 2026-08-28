import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: vi.fn() }));

import { isPublicRoute } from "./application-frame";

describe("isPublicRoute", () => {
  it("keeps every published policy document outside the authenticated workspace shell", () => {
    for (const pathname of ["/privacy-policy", "/user-agreement", "/ai-compute-billing", "/service-agreement"]) {
      expect(isPublicRoute(pathname)).toBe(true);
    }

    expect(isPublicRoute("/listing-kits/home")).toBe(false);
  });
});
