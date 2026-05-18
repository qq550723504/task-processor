"use client";

import Link from "next/link";

import { StoreRoutingSettingsCard } from "@/components/listingkit/settings/store-routing-settings-card";
import { StoreProfileSettingsPanel } from "@/components/listingkit/stores/store-profile-settings-panel";

export function TenantStoreSettingsPage() {
  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <h1 className="text-2xl font-semibold text-zinc-950">我的店铺配置</h1>
        <p className="mt-1 max-w-3xl text-sm text-zinc-500">
          这里维护当前租户自己的 SHEIN 店铺发布配置和默认选店策略。平台管理员仍然可以在
          <Link
            className="mx-1 text-zinc-950 underline decoration-zinc-300 underline-offset-4"
            href="/listing-kits/admin/stores"
          >
            平台店铺管理
          </Link>
          里维护店铺主数据。
        </p>
      </section>

      <StoreProfileSettingsPanel />
      <StoreRoutingSettingsCard />
    </div>
  );
}
