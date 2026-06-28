"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type {
  SheinActivityEnrollmentItemRecord,
  SheinActivityEnrollmentRunRecord,
} from "@/lib/types/listingkit/shein-enrollment";

export function SheinEnrollmentRunsTable({
  detailItems = [],
  detailLoading = false,
  isLoading,
  items,
  onViewDetails,
  selectedRunId,
}: {
  detailItems?: SheinActivityEnrollmentItemRecord[];
  detailLoading?: boolean;
  isLoading: boolean;
  items: SheinActivityEnrollmentRunRecord[];
  onViewDetails?: (runId: number) => void;
  selectedRunId?: number | null;
}) {
  const selectedRun = items.find((item) => item.id === selectedRunId);

  return (
    <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>活动</TableHead>
            <TableHead>触发方式</TableHead>
            <TableHead>状态</TableHead>
            <TableHead>候选/成功/失败</TableHead>
            <TableHead>开始时间</TableHead>
            <TableHead>结束时间</TableHead>
            <TableHead>错误摘要</TableHead>
            <TableHead className="w-24 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={8}>
                加载中...
              </TableCell>
            </TableRow>
          ) : items.length === 0 ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={8}>
                暂无报名记录
              </TableCell>
            </TableRow>
          ) : (
            items.map((item) => (
              <TableRow key={item.id}>
                <TableCell>
                  <div className="font-medium text-zinc-950">
                    {item.activity_type || "-"}
                  </div>
                  <div className="text-xs text-zinc-500">{item.activity_key || "-"}</div>
                </TableCell>
                <TableCell>{item.trigger_mode || "-"}</TableCell>
                <TableCell>{item.status || "-"}</TableCell>
                <TableCell>
                  {(item.candidate_count ?? 0).toString()}/
                  {(item.succeeded_count ?? 0).toString()}/
                  {(item.failed_count ?? 0).toString()}
                </TableCell>
                <TableCell>{item.started_at || "-"}</TableCell>
                <TableCell>{item.finished_at || "-"}</TableCell>
                <TableCell className="max-w-xl whitespace-normal break-words">
                  {item.error_summary || "-"}
                </TableCell>
                <TableCell className="w-24 text-right align-middle">
                  <button
                    className={
                      item.id === selectedRunId
                        ? "h-8 min-w-[4.5rem] whitespace-nowrap rounded-lg bg-zinc-950 px-3 text-xs font-medium text-white"
                        : "h-8 min-w-[4.5rem] whitespace-nowrap rounded-lg border border-zinc-200 px-3 text-xs font-medium text-zinc-700 hover:border-zinc-300 hover:bg-zinc-50"
                    }
                    disabled={!item.id}
                    onClick={() => item.id && onViewDetails?.(item.id)}
                    type="button"
                  >
                    查看详情
                  </button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
      {selectedRun ? (
        <div className="border-t border-zinc-200 bg-zinc-50 px-4 py-4">
          <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm font-medium text-zinc-950">报名明细</div>
            <div className="text-xs text-zinc-500">
              批次 #{selectedRun.id} · {detailItems.length} 条
            </div>
          </div>
          <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>SKC</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>候选ID</TableHead>
                  <TableHead>失败原因</TableHead>
                  <TableHead>更新时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {detailLoading ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={5}>
                      明细加载中...
                    </TableCell>
                  </TableRow>
                ) : detailItems.length === 0 ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={5}>
                      暂无报名明细
                    </TableCell>
                  </TableRow>
                ) : (
                  detailItems.map((item) => (
                    <TableRow key={item.id ?? `${item.run_id}:${item.candidate_id}`}>
                      <TableCell className="font-medium text-zinc-950">
                        {item.skc_name || "-"}
                      </TableCell>
                      <TableCell>{item.status || "-"}</TableCell>
                      <TableCell>{item.candidate_id ?? "-"}</TableCell>
                      <TableCell className="max-w-2xl whitespace-normal text-zinc-700">
                        {item.error_message || "-"}
                      </TableCell>
                      <TableCell>{item.updated_at || "-"}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      ) : null}
    </div>
  );
}
