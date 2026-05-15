"use client";

import { PromptSettingsCard } from "@/components/listingkit/settings/prompt-settings-card";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";

export function PromptManagementPage() {
  return (
    <ListingKitPageShell backgroundClassName="isolate bg-[linear-gradient(180deg,#f8fafc_0%,#eef2f7_100%)]">
      <section className="rounded-[2rem] border border-white/70 bg-white/78 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)]">
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
          Prompt Ops
        </p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
          提示词管理
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
          管理当前 ZITADEL 租户的提示词模板。这里的模板直接影响分类、文案和属性相关的 AI 链路。
        </p>
      </section>

      <PromptSettingsCard />
    </ListingKitPageShell>
  );
}
