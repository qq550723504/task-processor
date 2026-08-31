"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { StoreForm } from "@/components/workbench/stores/store-form";
import { StoreLifecycleActions } from "@/components/workbench/stores/store-lifecycle-actions";
import { Button } from "@/components/ui/button";
import { WorkbenchAPIError, type WorkbenchStore } from "@/lib/api/workbench-stores";
import { useWorkbenchStore } from "@/lib/query/use-workbench-stores";
import type { WorkbenchStoreUpdateInput } from "@/lib/validation/workbench-store";

type RecoveryState =
  | { state: "idle" }
  | { state: "loading" | "failed"; base: WorkbenchStore; draft: WorkbenchStoreUpdateInput }
  | { state: "ready"; base: WorkbenchStore; draft: WorkbenchStoreUpdateInput; latest: WorkbenchStore; changedFields: ("name" | "region")[] };

export function StoreDetailPage({ storeId }: { storeId: string }) {
  const router = useRouter();
  const context = useWorkbenchContext();
  const organizationId = context.effectiveOrganization?.id ?? "";
  const [initialOrganizationId] = useState(organizationId);
  const organizationChanged = organizationId !== initialOrganizationId;
  useEffect(() => {
    if (!organizationId || !organizationChanged) return;
    router.replace("/workbench/stores");
  }, [organizationChanged, organizationId, router]);
  if (organizationChanged) {
    return (
      <section
        className="mx-auto max-w-2xl px-4 py-8"
        role="status"
      >
        正在切换企业...
      </section>
    );
  }
  return (
    <StoreDetailContent
      key={`${organizationId}:${storeId}`}
      canUpdate={context.roles.some((role) => updateRoles.has(role))}
      storeId={storeId}
    />
  );
}

const updateRoles = new Set(["listingkit_operator", "listingkit_admin", "platform_admin"]);
const lifecycleLabels: Record<WorkbenchStore["lifecycleStatus"], string> = {
  provisioning: "开通中",
  active: "已启用",
  disabled: "已停用",
  deleting: "删除中",
};

function StoreDetailContent({ canUpdate, storeId }: { canUpdate: boolean; storeId: string }) {
  const router = useRouter();
  const storeQuery = useWorkbenchStore(storeId);
  const [displayedStore, setDisplayedStore] = useState<WorkbenchStore | null>(null);
  const [recovery, setRecovery] = useState<RecoveryState>({ state: "idle" });
  const store = displayedStore ?? (recovery.state === "idle" ? storeQuery.data : recovery.base);
  if (storeQuery.isPending && !store) return <section className="mx-auto max-w-2xl px-4 py-8" role="status">正在加载店铺...</section>;
  if (!store) {
    const code = (storeQuery.error as Partial<WorkbenchAPIError> | undefined)?.code;
    return <DetailError code={code} retry={() => void storeQuery.refetch()} />;
  }

  const loadLatest = async (
    draft: WorkbenchStoreUpdateInput,
    base: WorkbenchStore,
  ) => {
    setRecovery({ state: "loading", base, draft });
    const result = await storeQuery.refetch();
    if (!result.isSuccess || result.isError || !result.data) {
      setRecovery({ state: "failed", base, draft });
      return;
    }
    setRecovery({ state: "ready", base, draft, latest: result.data, changedFields: changedFields(base, result.data) });
  };
  const refreshLifecycleStore = async () => {
    const result = await storeQuery.refetch();
    if (!result.isSuccess || result.isError || !result.data) return null;
    setDisplayedStore(result.data);
    return result.data;
  };
  return <>
    <section className="mx-auto mt-6 max-w-2xl rounded-xl border bg-card p-4">
      <p className="text-sm text-muted-foreground">店铺中心</p>
      <h1 className="mt-1 text-2xl font-semibold tracking-tight">{store.name}</h1>
      <p className="mt-2 text-sm text-muted-foreground">店铺状态：{lifecycleLabels[store.lifecycleStatus]}</p>
      <div className="mt-4"><StoreLifecycleActions onDeleted={() => router.push("/workbench/stores?notice=store-deleted")} onRefreshStore={refreshLifecycleStore} onStoreUpdated={(next) => { setDisplayedStore(next); setRecovery({ state: "idle" }); }} store={store} /></div>
    </section>
    {recovery.state === "failed" ? <section className="mx-auto mt-6 max-w-2xl rounded-xl border bg-card p-4" role="alert"><p>无法确认店铺最新版本，草稿已保留。</p><Button className="mt-3" onClick={() => void loadLatest(recovery.draft, recovery.base)} variant="outline">重试获取最新版本</Button></section> : null}
    {canUpdate && store.lifecycleStatus !== "deleting" ? <StoreForm conflict={recovery.state === "ready" ? { latest: recovery.latest, changedFields: recovery.changedFields } : null} mode="edit" onConflict={(draft, baseline) => void loadLatest(draft, baseline)} onSaved={(next) => { setDisplayedStore(next); setRecovery({ state: "idle" }); }} recoveryState={recovery.state === "loading" || recovery.state === "failed" ? recovery.state : "idle"} store={store} /> : null}
  </>;
}

function changedFields(oldStore: WorkbenchStore, latest: WorkbenchStore): ("name" | "region")[] {
  return (["name", "region"] as const).filter((field) => oldStore[field] !== latest[field]);
}

function DetailError({ code, retry }: { code?: string; retry: () => void }) {
  const message = code === "STORE_NOT_FOUND" || code === "ORGANIZATION_ACCESS_DENIED" || code === "ORGANIZATION_ACCESS_REVOKED" ? "店铺不存在或已不可访问" : code === "PERMISSION_DENIED" ? "没有编辑当前企业店铺的权限" : "店铺服务暂时不可用，请稍后重试";
  return <section className="mx-auto mt-8 max-w-2xl rounded-xl border bg-card p-6" role="alert"><h1 className="font-semibold">{message}</h1><Button className="mt-4" onClick={retry} variant="outline">重试</Button></section>;
}
