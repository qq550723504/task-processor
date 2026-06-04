"use client";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { SheinSyncedProductRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinSyncedProductsTable({
  isLoading,
  items,
}: {
  isLoading: boolean;
  items: SheinSyncedProductRecord[];
}) {
  return (
    <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>商品</TableHead>
            <TableHead>SKC</TableHead>
            <TableHead>上架状态</TableHead>
            <TableHead>发布时间</TableHead>
            <TableHead>同步时间</TableHead>
            <TableHead>生效成本</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={6}>
                加载中...
              </TableCell>
            </TableRow>
          ) : items.length === 0 ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={6}>
                暂无同步商品
              </TableCell>
            </TableRow>
          ) : (
            items.map((item) => (
              <TableRow key={item.id}>
                <TableCell>
                  <div className="font-medium text-zinc-950">
                    {item.product_name_multi || item.spu_name || "-"}
                  </div>
                  <div className="text-xs text-zinc-500">{item.supplier_code || "-"}</div>
                </TableCell>
                <TableCell>{item.skc_name || item.skc_code || "-"}</TableCell>
                <TableCell>{item.shelf_status || "-"}</TableCell>
                <TableCell>{item.publish_time || "-"}</TableCell>
                <TableCell>{item.last_sync_at || "-"}</TableCell>
                <TableCell>{item.effective_cost_price ?? "-"}</TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
