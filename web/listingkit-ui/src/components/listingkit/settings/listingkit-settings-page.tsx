"use client";

import { AIClientSettingsCard } from "@/components/listingkit/settings/ai-client-settings-card";
import { ZitadelSessionCard } from "@/components/listingkit/settings/zitadel-session-card";
import { SheinSettingsCard } from "@/components/listingkit/shein/shein-settings-card";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";

export function ListingKitSettingsPage() {
  return (
    <ListingKitPageShell backgroundClassName="isolate bg-[linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]">
      <section className="rounded-[2rem] border border-white/70 bg-white/78 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)]">
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
          客户配置
        </p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
          ListingKit 设置
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
          管理当前客户的 AI 模型接口、SHEIN 店铺和价格规则。这里的配置会被后续任务直接读取。
        </p>
      </section>

      <ZitadelSessionCard />
      <AIClientSettingsCard />
      <SheinSettingsCard />
    </ListingKitPageShell>
  );
}
