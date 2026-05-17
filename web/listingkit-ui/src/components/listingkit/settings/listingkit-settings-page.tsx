"use client";

import { useListingKitSettingsSections } from "@/components/listingkit/settings/listingkit-settings-registry";
import { ListingKitSettingsShell } from "@/components/listingkit/settings/listingkit-settings-shell";

export function ListingKitSettingsPage() {
  const { sections } = useListingKitSettingsSections();

  return (
    <ListingKitSettingsShell
      eyebrow="客户配置"
      title="ListingKit 设置"
      description="管理当前客户的 ZITADEL 会话信息和 AI 模型接口配置。这里的配置会被后续任务直接读取。"
      backgroundClassName="isolate bg-[linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]"
      sections={sections}
    />
  );
}
