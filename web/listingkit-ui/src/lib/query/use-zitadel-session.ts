"use client";

import { useQuery } from "@tanstack/react-query";

import { listingKitSettingsKeys } from "@/lib/query/listingkit-settings";

export type ZitadelIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  username?: string;
  userType?: string | number;
  roles?: string[];
};

export function useZitadelSession() {
  return useQuery({
    queryKey: listingKitSettingsKeys.zitadelSession(),
    queryFn: async () => {
      const response = await fetch("/api/zitadel-auth/session", {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      const payload = (await response.json()) as {
        identity?: ZitadelIdentity;
        message?: string;
        error?: string;
      };
      if (!response.ok || !payload.identity) {
        throw new Error(payload.message || payload.error || "ZITADEL 会话不可用");
      }
      return payload.identity;
    },
    staleTime: 30_000,
  });
}
