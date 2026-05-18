"use client";

import { useMemo } from "react";

import { AIClientSettingsCard } from "@/components/listingkit/settings/ai-client-settings-card";
import { StoreRoutingSettingsCard } from "@/components/listingkit/settings/store-routing-settings-card";
import {
  ListingKitSettingsSectionDefinition,
} from "@/components/listingkit/settings/listingkit-settings-shell";
import { ZitadelSessionCard } from "@/components/listingkit/settings/zitadel-session-card";
import { useListingKitSettingsNamespaces } from "@/lib/query/use-listingkit-settings-metadata";
import type { ListingKitSettingsNamespaceSchema } from "@/lib/types/listingkit";

const staticSectionSummary: Record<string, string> = {
  session: "当前 ZITADEL 登录态和角色。",
  ai: "租户和用户级模型 endpoint、key 与模型。",
  "store-routing": "多店铺默认选店策略和 fallback 配置。",
};

function namespaceBadges(schema?: ListingKitSettingsNamespaceSchema) {
  if (!schema?.supported_scopes?.length) {
    return undefined;
  }
  return schema.supported_scopes.map((scope) => scope.label);
}

function resolveSummary(id: string, schema?: ListingKitSettingsNamespaceSchema) {
  return schema?.description ?? staticSectionSummary[id] ?? "";
}

export function useListingKitSettingsSections() {
  const metadata = useListingKitSettingsNamespaces();
  const byNamespace = useMemo(
    () =>
      new Map(
        (metadata.data?.items ?? []).map((schema) => [schema.namespace, schema]),
      ),
    [metadata.data?.items],
  );

  const sections: ListingKitSettingsSectionDefinition[] = [
    {
      id: "session",
      label: "会话",
      summary: resolveSummary("session"),
      render: () => <ZitadelSessionCard />,
    },
    {
      id: "ai",
      label: byNamespace.get("ai")?.label ?? "AI 配置",
      summary: resolveSummary("ai", byNamespace.get("ai")),
      badges: namespaceBadges(byNamespace.get("ai")),
      render: () => <AIClientSettingsCard />,
    },
    {
      id: "store-routing",
      label: "店铺路由",
      summary: resolveSummary("store-routing"),
      render: () => <StoreRoutingSettingsCard />,
    },
  ];

  return {
    sections,
    isLoading: metadata.isPending,
    isError: metadata.isError,
  };
}
