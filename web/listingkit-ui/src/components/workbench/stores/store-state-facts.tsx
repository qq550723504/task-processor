import type { WorkbenchStore } from "@/lib/api/workbench-stores";

const serviceLabels = {
  pending_activation: "待激活",
  active: "有效",
  expired: "已到期",
  suspended: "已暂停",
};

export function StoreStateFacts({ store }: { store: WorkbenchStore }) {
  return (
    <div className="mt-3 text-sm text-muted-foreground">
      <p>店铺服务：{store.serviceStatus === null ? "尚未建立" : serviceLabels[store.serviceStatus]}</p>
      {store.serviceStartedAt && store.serviceExpiresAt ? (
        <p>
          服务期限：<time dateTime={store.serviceStartedAt}>{store.serviceStartedAt}</time>
          {" 至 "}<time dateTime={store.serviceExpiresAt}>{store.serviceExpiresAt}</time>
        </p>
      ) : null}
      <p>平台连接及店铺服务激活、续费尚未开放。</p>
    </div>
  );
}
