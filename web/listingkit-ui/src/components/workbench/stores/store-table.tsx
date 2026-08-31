import Link from "next/link";

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

export function StoreTable({ stores }: { stores: WorkbenchStore[] }) {
  return (
    <div className="overflow-x-auto rounded-xl border bg-card">
      <table aria-label="我的店铺列表" className="w-full min-w-[900px] text-sm">
        <thead className="border-b bg-muted/40 text-left text-muted-foreground">
          <tr>
            {[
              "店铺名称",
              "平台",
              "区域",
              "外部店铺 ID",
              "店铺状态",
              "连接状态",
              "更新时间",
              "操作",
            ].map((column) => (
              <th className="px-4 py-3 font-medium" key={column} scope="col">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {stores.map((store) => {
            const timestamp = formatStoreTimestamp(store.updatedAt);
            return (
              <tr className="border-b last:border-0" key={store.id}>
                <td className="px-4 py-3 font-medium">{store.name}</td>
                <td className="px-4 py-3">SHEIN</td>
                <td className="px-4 py-3">{store.region}</td>
                <td className="px-4 py-3">{store.externalStoreId || "未设置"}</td>
                <td className="px-4 py-3">
                  <StatusBadge>{lifecycleLabels[store.lifecycleStatus]}</StatusBadge>
                </td>
                <td className="px-4 py-3">
                  <StatusBadge>{connectionLabels[store.connectionStatus]}</StatusBadge>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  <time dateTime={timestamp ? store.updatedAt : ""}>{timestamp || "—"}</time>
                </td>
                <td className="px-4 py-3">
                  <Link
                    aria-label={`查看${store.name}`}
                    className="text-primary underline-offset-4 hover:underline"
                    href={`/workbench/stores/${store.id}`}
                  >
                    查看
                  </Link>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function StatusBadge({ children }: { children: string }) {
  return <span className="rounded-full bg-muted px-2 py-1 text-xs font-medium">{children}</span>;
}

function formatStoreTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const part = (number: number) => String(number).padStart(2, "0");
  return `${date.getUTCFullYear()}-${part(date.getUTCMonth() + 1)}-${part(date.getUTCDate())} ${part(date.getUTCHours())}:${part(date.getUTCMinutes())} UTC`;
}
