"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, RefreshCw, XCircle } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  formatSubscriptionApiError,
  getCurrentSubscription,
  type SubscriptionEntitlementView,
  type SubscriptionStatus,
} from "@/lib/api/subscription";

const STATUS_LABEL: Record<SubscriptionStatus, string> = {
  active: "已开通",
  trialing: "试用中",
  expired: "已过期",
  disabled: "已停用",
};

export function SubscriptionPage() {
  const query = useQuery({
    queryKey: ["listingkit-subscription"],
    queryFn: getCurrentSubscription,
  });

  const summary = query.data;
  const visibleError = query.error ? formatSubscriptionApiError(query.error) : "";

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <CardTitle className="text-2xl">订阅</CardTitle>
            <CardDescription className="mt-1">
              当前租户 {summary?.tenant_id ?? "-"}，按模块开通 ListingKit 能力。
            </CardDescription>
            <p className="mt-2 inline-flex rounded-md bg-zinc-100 px-2.5 py-1 text-sm font-medium text-zinc-700">
              当前套餐：{summary?.current_plan?.plan.name ?? "未配置"}
            </p>
          </div>
          <Button
            type="button"
            onClick={() => void query.refetch()}
            variant="secondary"
          >
            <RefreshCw className={`size-4 ${query.isFetching ? "animate-spin" : ""}`} />
            刷新
          </Button>
        </CardHeader>
        {visibleError ? (
          <CardContent>
            <Alert variant="destructive">
              <AlertDescription>{visibleError}</AlertDescription>
            </Alert>
          </CardContent>
        ) : null}
      </Card>

      <Card className="overflow-hidden p-0">
        <Table className="min-w-full">
            <TableHeader className="bg-zinc-50">
              <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
                <TableHead>模块</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>有效期</TableHead>
                <TableHead>额度</TableHead>
                <TableHead>用量</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {query.isLoading ? (
                <TableRow>
                  <TableCell className="py-6 text-zinc-500" colSpan={5}>
                    加载中...
                  </TableCell>
                </TableRow>
              ) : (summary?.entitlements ?? []).length === 0 ? (
                <TableRow>
                  <TableCell className="py-6 text-zinc-500" colSpan={5}>
                    暂无模块
                  </TableCell>
                </TableRow>
              ) : (
                summary?.entitlements.map((view) => (
                  <TableRow key={view.module.code} className="align-top">
                    <TableCell>
                      <div className="font-medium text-zinc-950">{view.module.name}</div>
                      <div className="font-mono text-xs text-zinc-500">
                        {view.module.code}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge view={view} />
                    </TableCell>
                    <TableCell className="text-zinc-700">
                      {formatDate(view.entitlement?.expires_at)}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-zinc-600">
                      {formatRecord(view.limits)}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-zinc-600">
                      {formatRecord(view.used)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
      </Card>
    </div>
  );
}

function StatusBadge({ view }: { view: SubscriptionEntitlementView }) {
  const active = view.allowed;
  return (
    <Badge className="gap-1" variant={active ? "success" : "neutral"}>
      {active ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
      {view.entitlement ? STATUS_LABEL[view.entitlement.status] : "未开通"}
    </Badge>
  );
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatRecord(value?: Record<string, number>) {
  if (!value || Object.keys(value).length === 0) {
    return "-";
  }
  return Object.entries(value)
    .map(([key, count]) => `${key}: ${formatMetricValue(key, count)}`)
    .join(", ");
}

function formatMetricValue(key: string, value: number) {
  if (key === "storage_bytes" || key.endsWith("_bytes")) {
    return formatBytes(value);
  }
  return String(value);
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const maximumFractionDigits = unitIndex === 0 ? 0 : 1;
  return `${new Intl.NumberFormat("zh-CN", { maximumFractionDigits }).format(size)} ${units[unitIndex]}`;
}
