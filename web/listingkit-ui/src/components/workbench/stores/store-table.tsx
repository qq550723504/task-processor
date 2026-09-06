"use client";

import Link from "next/link";

import { StoreLifecycleActions } from "@/components/workbench/stores/store-lifecycle-actions";
import type { WorkbenchStore } from "@/lib/api/workbench-stores";

const lifecycleLabels: Record<WorkbenchStore["lifecycleStatus"], string> = {
  provisioning: "开通中",
  active: "已启用",
  disabled: "已停用",
  deleting: "删除中",
};

const connectionLabels: Record<WorkbenchStore["connectionStatus"], string> = {
  disconnected: "未连接",
  connected: "已连接",
  expired: "授权已过期",
  unavailable: "暂时无法检查",
};

export function StoreTable({ stores, onDeleted, onRefreshStore }: { stores: WorkbenchStore[]; onDeleted?: () => void; onRefreshStore?: (storeId: string) => Promise<WorkbenchStore | null | undefined> }) {
  return <ul aria-label="我的店铺列表" className="console-store-list">{stores.map((store) => {
    const timestamp = formatStoreTimestamp(store.updatedAt);
    return <li key={store.id} className="console-store-card">
      <div className="console-store-card-header"><div className="console-store-identity"><span aria-hidden="true" className="console-store-avatar">S</span><div><h2>{store.name}</h2><p className="console-description">外部店铺 ID：<span>{store.externalStoreId || "未设置"}</span></p></div></div>
        <div className="console-store-badges"><span>SHEIN</span><span>{store.region}</span><span>{lifecycleLabels[store.lifecycleStatus]}</span><span>{connectionLabels[store.connectionStatus]}</span></div>
      </div>
      <div className="console-store-card-footer"><dl><dt>更新时间</dt><dd><time dateTime={timestamp ? store.updatedAt : ""}>{timestamp || "—"}</time></dd></dl>
        <div className="console-store-actions"><StoreLifecycleActions onDeleted={onDeleted} onRefreshStore={onRefreshStore ? () => onRefreshStore(store.id) : undefined} store={store} /><Link aria-label={`查看${store.name}`} className="console-store-detail" href={`/workbench/stores/${store.id}`}>进入店铺 →</Link></div>
      </div>
    </li>;
  })}</ul>;
}
function formatStoreTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const part = (number: number) => String(number).padStart(2, "0");
  return `${date.getUTCFullYear()}-${part(date.getUTCMonth() + 1)}-${part(date.getUTCDate())} ${part(date.getUTCHours())}:${part(date.getUTCMinutes())} UTC`;
}
