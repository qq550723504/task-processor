"use client";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { StoreForm } from "@/components/workbench/stores/store-form";
import { canCreateWorkbenchStore } from "@/lib/workbench/permissions";

export default function NewWorkbenchStorePage() {
  const context = useWorkbenchContext();
  if (context.isLoading) {
    return <section className="mx-auto w-full max-w-2xl px-4 py-8" role="status">正在加载企业权限...</section>;
  }
  if (!context.effectiveOrganization || !canCreateWorkbenchStore(context.roles)) {
    return <section className="mx-auto w-full max-w-2xl px-4 py-8" role="alert">没有创建当前企业店铺的权限。</section>;
  }
  return <StoreForm mode="create" />;
}
