"use client";

import { useListingKitSettingsSections } from "@/components/listingkit/settings/listingkit-settings-registry";
import { ListingKitSettingsShell } from "@/components/listingkit/settings/listingkit-settings-shell";

export function ListingKitSettingsPage() {
  const { sections } = useListingKitSettingsSections();

  return (
      <ListingKitSettingsShell
      eyebrow="租户配置"
      title="ListingKit 设置"
      description="管理当前租户和当前登录用户的会话、配置健康检查与 AI 模型接口配置。这里的配置会被后续任务、SHEIN 提交和图片链路直接读取。"
      backgroundClassName="isolate bg-[linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]"
      sections={sections}
    />
  );
}
