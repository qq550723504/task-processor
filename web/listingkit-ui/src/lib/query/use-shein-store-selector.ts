"use client";

import { useMemo } from "react";

import { buildSheinLoginStatusMap } from "@/components/listingkit/stores/store-login-status";
import { useSheinLoginAccounts } from "@/lib/query/use-shein-login";
import { useStoreProfiles, enabledStoreProfiles } from "@/lib/query/use-store-profiles";

export function useSheinStoreSelector(selectedStoreId?: string) {
  const profiles = useStoreProfiles();
  const sheinLoginAccounts = useSheinLoginAccounts();

  const enabledProfiles = useMemo(
    () => enabledStoreProfiles(profiles.data),
    [profiles.data],
  );
  const sheinLoginStatusMap = useMemo(
    () => buildSheinLoginStatusMap(sheinLoginAccounts.data),
    [sheinLoginAccounts.data],
  );

  const effectiveStoreId = (selectedStoreId ?? "").trim();
  const selectedStoreLoginStatus = useMemo(() => {
    const parsed = Number.parseInt(effectiveStoreId, 10);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return null;
    }
    return sheinLoginStatusMap.get(parsed) ?? null;
  }, [effectiveStoreId, sheinLoginStatusMap]);

  const loggedInStoreCount = useMemo(
    () =>
      (sheinLoginAccounts.data ?? []).filter(
        (item) => item.has_cookie || (item.cookie_ttl ?? 0) > 0,
      ).length,
    [sheinLoginAccounts.data],
  );

  return {
    profiles,
    sheinLoginAccounts,
    enabledProfiles,
    selectedStoreLoginStatus,
    loggedInStoreCount,
    anyLoggedInStore: loggedInStoreCount > 0,
  };
}
