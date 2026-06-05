"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { SheinActivityEnrollmentRunRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinEnrollmentRunsTable({
  isLoading,
  items,
}: {
  isLoading: boolean;
  items: SheinActivityEnrollmentRunRecord[];
}) {
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
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={7}>
                加载中...
              </TableCell>
            </TableRow>
          ) : items.length === 0 ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={7}>
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
                <TableCell>{item.error_summary || "-"}</TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
