"use client";

import { PromptTemplatesPanel } from "@/components/listingkit/prompts/prompt-templates-panel";
import { ListingKitSettingsShell } from "@/components/listingkit/settings/listingkit-settings-shell";

export function PromptManagementPage() {
  return (
    <ListingKitSettingsShell
      eyebrow="Prompt Ops"
      title="提示词管理"
      description="管理 Prompt 模板目录及其作用域覆盖。这里的模板直接影响分类、文案、属性和图片相关的 AI 链路。"
      backgroundClassName="isolate bg-[linear-gradient(180deg,#f8fafc_0%,#eef2f7_100%)]"
      sections={[
        {
          id: "prompts",
          label: "提示词模板",
          summary: "按 schema 定义的作用域维护模板内容、版本和启停状态。",
          render: () => <PromptTemplatesPanel />,
        },
      ]}
    />
  );
}
