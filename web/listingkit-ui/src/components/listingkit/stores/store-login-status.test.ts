import { describe, expect, it } from "vitest";

import {
  hasUsableSheinCookie,
  sheinLoginStatusLabel,
} from "@/components/listingkit/stores/store-login-status";
import type { SheinLoginAccountStatus } from "@/lib/types/shein-login";

function status(overrides: Partial<SheinLoginAccountStatus> = {}): SheinLoginAccountStatus {
  return {
    account: {
      store_id: 985,
      tenant_id: 246,
      store_name: "店铺 985",
    },
    has_cookie: false,
    cookie_ttl: 30 * 24 * 60 * 60,
    waiting_for_verify_code: false,
    login_in_progress: false,
    ...overrides,
  };
}

describe("SHEIN login status", () => {
  it("does not treat an expiring empty cookie payload as usable", () => {
    const item = status();

    expect(hasUsableSheinCookie(item)).toBe(false);
    expect(sheinLoginStatusLabel(item)).toEqual({
      label: "Cookie无效",
      variant: "danger",
    });
  });

  it("treats a backend-validated cookie as usable", () => {
    const item = status({ has_cookie: true });

    expect(hasUsableSheinCookie(item)).toBe(true);
    expect(sheinLoginStatusLabel(item)).toEqual({
      label: "已登录",
      variant: "success",
    });
  });
});
