"use client";

import { useMemo } from "react";

import { buildSheinLoginStatusMap } from "@/components/listingkit/stores/store-login-status";
import { useSheinLoginAccounts } from "@/lib/query/use-shein-login";
import { useStoreProfiles, enabledStoreProfiles } from "@/lib/query/use-store-profiles";
import { useStoreRouting } from "@/lib/query/use-store-routing";

export function useSheinStoreSelector(selectedStoreId?: string) {
  const profiles = useStoreProfiles();
  const routing = useStoreRouting();
  const sheinLoginAccounts = useSheinLoginAccounts();

  const enabledProfiles = useMemo(
    () => enabledStoreProfiles(profiles.data),
    [profiles.data],
  );
  const sheinLoginStatusMap = useMemo(
    () => buildSheinLoginStatusMap(sheinLoginAccounts.data),
    [sheinLoginAccounts.data],
  );

  const recommendedStoreId = useMemo(() => {
    const fallbackStoreID = routing.data?.fallback_store_id;
    if (
      fallbackStoreID &&
      enabledProfiles.some((item) => item.store_id === fallbackStoreID)
    ) {
      return String(fallbackStoreID);
    }
    if (enabledProfiles.length === 1) {
      return String(enabledProfiles[0].store_id);
    }
    return "";
  }, [enabledProfiles, routing.data?.fallback_store_id]);

  const recommendedReason = useMemo(() => {
    if (!recommendedStoreId) {
      return "";
    }
    const profile = enabledProfiles.find(
      (item) => String(item.store_id) === recommendedStoreId,
    );
    if (!profile) {
      return "";
    }
    if (
      routing.data?.fallback_store_id &&
      String(routing.data.fallback_store_id) === recommendedStoreId
    ) {
      return "当前会优先使用 fallback 店铺作为推荐值。";
    }
    if (enabledProfiles.length === 1) {
      return "当前只有一个已启用店铺，任务会默认落到这个店铺。";
    }
    if (routing.data?.selection_strategy === "priority") {
      return "当前按优先级路由；这里预选的是最高优先级店铺。";
    }
    return "";
  }, [enabledProfiles, recommendedStoreId, routing.data?.fallback_store_id, routing.data?.selection_strategy]);

  const effectiveStoreId = (selectedStoreId ?? "").trim() || recommendedStoreId;
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
    routing,
    sheinLoginAccounts,
    enabledProfiles,
    recommendedStoreId,
    recommendedReason,
    selectedStoreLoginStatus,
    loggedInStoreCount,
    anyLoggedInStore: loggedInStoreCount > 0,
  };
}
