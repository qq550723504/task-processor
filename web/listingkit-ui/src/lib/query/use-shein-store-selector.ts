"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { buildSheinLoginStatusMap } from "@/components/listingkit/stores/store-login-status";
import { getListingKitSettings } from "@/lib/api/listingkit-settings";
import { useSheinLoginAccounts } from "@/lib/query/use-shein-login";
import { listingKitSettingsKeys } from "@/lib/query/listingkit-settings";
import { useStoreProfiles, enabledStoreProfiles } from "@/lib/query/use-store-profiles";
import type {
  ListingKitSheinSettings,
  ListingKitStoreCatalogOption,
  ListingKitStoreProfile,
} from "@/lib/types/listingkit";

export function buildSheinStoreOptions(
  availableStores: ListingKitStoreCatalogOption[] | undefined,
  profiles: ListingKitStoreProfile[] | undefined,
) {
  const enabledProfiles = enabledStoreProfiles(profiles);
  const profilesByStoreID = new Map(
    enabledProfiles.map((profile) => [profile.store_id, profile]),
  );

  return (availableStores ?? []).reduce<ListingKitStoreProfile[]>((options, store) => {
    if (store.id <= 0 || (store.status !== undefined && store.status !== 0)) {
      return options;
    }
    const profile = profilesByStoreID.get(store.id);
    options.push({
      ...profile,
      store_id: store.id,
      enabled: profile?.enabled ?? true,
      store: {
        ...store,
        id: store.id,
      },
    });
    return options;
  }, []);
}

export function useSheinStoreSelector(selectedStoreId?: string) {
  const profiles = useStoreProfiles();
  const sheinLoginAccounts = useSheinLoginAccounts();
  const sheinSettings = useQuery({
    queryKey: listingKitSettingsKeys.namespace("shein"),
    queryFn: () => getListingKitSettings<ListingKitSheinSettings>("shein"),
    staleTime: 30_000,
  });

  const enabledProfiles = useMemo(
    () => enabledStoreProfiles(profiles.data),
    [profiles.data],
  );
  const storeOptions = useMemo(
    () => buildSheinStoreOptions(sheinSettings.data?.available_stores, profiles.data),
    [profiles.data, sheinSettings.data?.available_stores],
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
        (item) => item.has_cookie,
      ).length,
    [sheinLoginAccounts.data],
  );

  return {
    profiles,
    sheinSettings,
    sheinLoginAccounts,
    enabledProfiles,
    storeOptions: storeOptions.length > 0 ? storeOptions : enabledProfiles,
    selectedStoreLoginStatus,
    loggedInStoreCount,
    anyLoggedInStore: loggedInStoreCount > 0,
  };
}
