"use client";

import { useMemo } from "react";

import { AIClientSettingsCard } from "@/components/listingkit/settings/ai-client-settings-card";
import {
  ListingKitSettingsSectionDefinition,
} from "@/components/listingkit/settings/listingkit-settings-shell";
import { SettingsHealthCard } from "@/components/listingkit/settings/settings-health-card";
import { ZitadelSessionCard } from "@/components/listingkit/settings/zitadel-session-card";
import { useListingKitSettingsNamespaces } from "@/lib/query/use-listingkit-settings-metadata";
import type { ListingKitSettingsNamespaceSchema } from "@/lib/types/listingkit";

const staticSectionSummary: Record<string, string> = {
  session: "当前 ZITADEL 登录态和角色。",
  health: "新任务、SHEIN 提交和图片链路的配置预检。",
  ai: "租户和用户级模型 endpoint、key 与模型。",
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
      id: "health",
      label: "健康检查",
      summary: resolveSummary("health"),
      badges: ["新任务预检", "影响范围"],
      render: () => <SettingsHealthCard />,
    },
    {
      id: "ai",
      label: byNamespace.get("ai")?.label ?? "AI 配置",
      summary: resolveSummary("ai", byNamespace.get("ai")),
      badges: namespaceBadges(byNamespace.get("ai")),
      render: () => <AIClientSettingsCard />,
    },
  ];

  return {
    sections,
    isLoading: metadata.isPending,
    isError: metadata.isError,
  };
}
