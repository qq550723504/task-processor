"use client";

import {
  formatInventorySummary,
  formatProductTimes,
  getCostSourceLabel,
} from "@/components/listingkit/shein-enrollment/shein-product-table-formatters";
import { formatSheinPriceSnapshot } from "@/components/listingkit/shein-enrollment/shein-price-snapshot";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { SheinSyncedProductRecord } from "@/lib/types/listingkit/shein-enrollment";

function formatCostPrice(value?: number | null) {
  if (value == null) {
    return "-";
  }

  return value.toFixed(2);
}

function getShelfStatusLabel(status?: string | null) {
  switch (status) {
    case "ON_SHELF":
      return "已上架";
    case "OFF_SHELF":
      return "已下架";
    default:
      return status || "-";
  }
}

export function SheinSyncedProductsTable({
  isLoading,
  items,
}: {
  isLoading: boolean;
  items: SheinSyncedProductRecord[];
}) {
  return (
    <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white">
      <Table className="min-w-[1180px]">
        <TableHeader>
          <TableRow>
            <TableHead className="w-[340px]">商品信息</TableHead>
            <TableHead className="w-[220px]">主规格</TableHead>
            <TableHead className="w-[120px]">近 7 天销量</TableHead>
            <TableHead className="w-[180px]">价格</TableHead>
            <TableHead className="w-[190px]">库存</TableHead>
            <TableHead className="w-[140px]">上架状态</TableHead>
            <TableHead className="w-[220px]">时间</TableHead>
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
                暂无同步商品
              </TableCell>
            </TableRow>
          ) : (
            items.map((item) => {
              const inventory = formatInventorySummary(item.inventory_snapshot);
              const times = formatProductTimes(item);

              return (
                <TableRow key={item.id}>
                  <TableCell>
                    <div className="flex items-start gap-3">
                      <div className="h-16 w-16 shrink-0 overflow-hidden rounded-xl border border-zinc-200 bg-zinc-100">
                        {item.main_image_url ? (
                          <img
                            alt={item.product_name_multi || item.spu_name || "SHEIN 商品"}
                            className="h-full w-full object-cover"
                            src={item.main_image_url}
                          />
                        ) : (
                          <div className="flex h-full w-full items-center justify-center text-xs text-zinc-400">
                            无图
                          </div>
                        )}
                      </div>
                      <div className="min-w-0 space-y-1">
                        <div className="line-clamp-2 font-medium text-zinc-950">
                          {item.product_name_multi || item.spu_name || "-"}
                        </div>
                        <div className="text-xs text-zinc-500">SPU: {item.spu_code || "-"}</div>
                        <div className="text-xs text-zinc-500">
                          货号: {item.supplier_code || "-"}
                        </div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="space-y-1">
                      <div className="font-medium text-zinc-900">
                        {item.sale_name || item.skc_name || "-"}
                      </div>
                      <div className="text-xs text-zinc-500">
                        SKC: {item.skc_name || item.skc_code || "-"}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="text-zinc-500">-</TableCell>
                  <TableCell>
                    <div className="space-y-1 text-sm text-zinc-900">
                      <div>{formatSheinPriceSnapshot(item.price_snapshot)}</div>
                      <div className="text-xs text-zinc-600">
                        成本 {formatCostPrice(item.effective_cost_price)}
                      </div>
                      <div className="text-xs text-zinc-500">
                        来源 {getCostSourceLabel(item.cost_price_source)}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="space-y-1 text-xs text-zinc-600">
                      <div>总库存 {inventory.total || "-"}</div>
                      <div>可用库存 {inventory.available || "-"}</div>
                      {inventory.raw ? <div className="break-all">{inventory.raw}</div> : null}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="inline-flex rounded-full bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700">
                      {getShelfStatusLabel(item.shelf_status)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="space-y-1 text-xs text-zinc-600">
                      {times.map((entry) => (
                        <div key={entry.label}>
                          {entry.label} {entry.value || "-"}
                        </div>
                      ))}
                    </div>
                  </TableCell>
                </TableRow>
              );
            })
          )}
        </TableBody>
      </Table>
    </div>
  );
}
