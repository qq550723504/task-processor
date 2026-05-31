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
import {
  formatSubscriptionDate,
  formatSubscriptionRecord,
  subscriptionModuleSummary,
} from "@/components/listingkit/subscription/subscription-display";

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
        <CardHeader className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <CardTitle className="text-2xl">当前租户订阅</CardTitle>
            <CardDescription className="mt-1">
              查看当前租户 {summary?.tenant_id ?? "-"} 的套餐与模块开通状态。
            </CardDescription>
            <p className="mt-2 inline-flex rounded-md bg-zinc-100 px-2.5 py-1 text-sm font-medium text-zinc-700">
              当前套餐：{summary?.current_plan?.plan.name ?? "未配置"}
            </p>
          </div>
          <Button
            type="button"
            onClick={() => void query.refetch()}
            variant="secondary"
            className="w-full sm:w-auto"
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
        <div className="overflow-x-auto">
        <Table className="min-w-[44rem]">
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
                      <div className="mt-1 text-xs text-zinc-500">
                        {subscriptionModuleSummary(view.module.code, view.module.description)}
                      </div>
                      <div className="font-mono text-xs text-zinc-500">
                        {view.module.code}
                      </div>
                    </TableCell>
                    <TableCell>
                      <StatusBadge view={view} />
                    </TableCell>
                    <TableCell className="text-zinc-700">
                      {formatSubscriptionDate(view.entitlement?.expires_at)}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-zinc-600">
                      {formatSubscriptionRecord(view.limits)}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-zinc-600">
                      {formatSubscriptionRecord(view.used)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
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
