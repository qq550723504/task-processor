"use client";

/* eslint-disable @next/next/no-img-element -- SDS image hosts are tenant data outside fixed Next image remote patterns. */
import {
  formatInventorySummary,
  formatProductTimes,
  getCostSourceLabel,
} from "@/components/listingkit/shein-enrollment/shein-product-table-formatters";
import {
  formatSheinPriceSnapshot,
  getSheinSKUPriceSnapshots,
  getSheinSKUSupplyPriceSnapshots,
} from "@/components/listingkit/shein-enrollment/shein-price-snapshot";
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

function getProfitSnapshot(item: SheinSyncedProductRecord) {
  const salePrice = parsePriceSnapshotAmount(item.price_snapshot);
  const costPrice = item.effective_cost_price;
  if (salePrice == null || costPrice == null || costPrice === 0) {
    return null;
  }
  const profit = salePrice - costPrice;
  return {
    profit,
    profitRate: (profit / costPrice) * 100,
    currency: getPriceSnapshotCurrency(item),
  };
}

function parsePriceSnapshotAmount(value?: string) {
  if (!value) {
    return null;
  }
  const match = value.replace(/,/g, "").match(/-?\d+(?:\.\d+)?/);
  if (!match) {
    return null;
  }
  const parsed = Number.parseFloat(match[0]);
  return Number.isFinite(parsed) ? parsed : null;
}

function getPriceSnapshotCurrency(item: SheinSyncedProductRecord) {
  if (item.currency?.trim()) {
    return item.currency.trim();
  }
  const match = item.price_snapshot?.match(/[A-Z]{3}/);
  return match?.[0] || "USD";
}

function formatSignedCurrency(value: number, currency: string) {
  return `${value >= 0 ? "+" : ""}${value.toFixed(2)} ${currency}`;
}

function formatSignedRate(value: number) {
  return `${value >= 0 ? "+" : ""}${value.toFixed(1)}%`;
}

function getShelfStatusView(item: SheinSyncedProductRecord) {
  if (item.is_active === false) {
    return {
      label: "已下架",
      className: "bg-zinc-100 text-zinc-600",
    };
  }

  switch (item.shelf_status) {
    case "ON_SHELF":
      return {
        label: "已上架",
        className: "bg-emerald-50 text-emerald-700",
      };
    case "OFF_SHELF":
      return {
        label: "已下架",
        className: "bg-zinc-100 text-zinc-600",
      };
    default:
      return {
        label: item.shelf_status || "-",
        className: "bg-zinc-100 text-zinc-600",
      };
  }
}

type SKUPriceGridRow = {
  skuCode: string;
  salePrice: string | null;
  supplyPrice: string | null;
};

function getSKUPriceGridRows(
  item: SheinSyncedProductRecord,
): SKUPriceGridRow[] {
  const rowsBySKU = new Map<string, SKUPriceGridRow>();
  const skuPrices = getSheinSKUPriceSnapshots(item.price_snapshot);
  const skuSupplyPrices = getSheinSKUSupplyPriceSnapshots(
    item.supply_price_snapshot,
  );

  for (const skuPrice of skuPrices) {
    rowsBySKU.set(skuPrice.skuCode, {
      skuCode: skuPrice.skuCode,
      salePrice: skuPrice.price,
      supplyPrice: null,
    });
  }

  for (const skuSupplyPrice of skuSupplyPrices) {
    const row = rowsBySKU.get(skuSupplyPrice.skuCode);
    if (row) {
      row.supplyPrice = skuSupplyPrice.price;
      continue;
    }
    rowsBySKU.set(skuSupplyPrice.skuCode, {
      skuCode: skuSupplyPrice.skuCode,
      salePrice: null,
      supplyPrice: skuSupplyPrice.price,
    });
  }

  const rows = Array.from(rowsBySKU.values());
  if (rows.length > 0) {
    return rows;
  }

  return [
    {
      skuCode: "-",
      salePrice: formatSheinPriceSnapshot(item.price_snapshot),
      supplyPrice: null,
    },
  ];
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
      <Table className="min-w-[1540px]">
        <TableHeader>
          <TableRow>
            <TableHead className="w-[320px]">商品信息</TableHead>
            <TableHead className="w-[180px]">主规格</TableHead>
            <TableHead className="w-[170px]">SKU</TableHead>
            <TableHead className="w-[120px] text-right">同步售价</TableHead>
            <TableHead className="w-[140px] text-right">SHEIN 供货价</TableHead>
            <TableHead className="w-[120px]">近 7 天销量</TableHead>
            <TableHead className="w-[160px]">成本 / 利润</TableHead>
            <TableHead className="w-[150px]">库存</TableHead>
            <TableHead className="w-[110px]">上架状态</TableHead>
            <TableHead className="w-[190px]">时间</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={10}>
                加载中...
              </TableCell>
            </TableRow>
          ) : items.length === 0 ? (
            <TableRow>
              <TableCell className="py-8 text-zinc-500" colSpan={10}>
                暂无同步商品
              </TableCell>
            </TableRow>
          ) : (
            items.flatMap((item, itemIndex) => {
              const inventory = formatInventorySummary(item.inventory_snapshot);
              const times = formatProductTimes(item);
              const profit = getProfitSnapshot(item);
              const shelfStatus = getShelfStatusView(item);
              const skuRows = getSKUPriceGridRows(item);
              const hasSupplyPriceSnapshot = skuRows.some(
                (row) => row.supplyPrice != null,
              );
              const itemKey =
                item.id ?? item.skc_name ?? `synced-product-${itemIndex}`;

              return skuRows.map((skuRow, skuIndex) => (
                <TableRow key={`${itemKey}-${skuRow.skuCode}`}>
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div className="flex items-start gap-3">
                        <div className="h-16 w-16 shrink-0 overflow-hidden rounded-xl border border-zinc-200 bg-zinc-100">
                          {item.main_image_url ? (
                            <img
                              alt={
                                item.product_name_multi ||
                                item.spu_name ||
                                "SHEIN 商品"
                              }
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
                          <div className="text-xs text-zinc-500">
                            SPU: {item.spu_code || "-"}
                          </div>
                          <div className="text-xs text-zinc-500">
                            货号: {item.supplier_code || "-"}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                  ) : null}
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div className="space-y-1">
                        <div className="font-medium text-zinc-900">
                          {item.sale_name || item.skc_name || "-"}
                        </div>
                        <div className="text-xs text-zinc-500">
                          SKC: {item.skc_name || item.skc_code || "-"}
                        </div>
                      </div>
                    </TableCell>
                  ) : null}
                  <TableCell className="font-mono text-xs text-zinc-700">
                    {skuRow.skuCode}
                  </TableCell>
                  <TableCell className="text-right font-medium tabular-nums text-zinc-900">
                    {skuRow.salePrice || "—"}
                  </TableCell>
                  <TableCell className="text-right font-medium tabular-nums text-zinc-900">
                    {skuRow.supplyPrice ||
                      (hasSupplyPriceSnapshot ? "—" : "待同步")}
                  </TableCell>
                  {skuIndex === 0 ? (
                    <TableCell
                      className="align-top text-zinc-500"
                      rowSpan={skuRows.length}
                    >
                      -
                    </TableCell>
                  ) : null}
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div className="space-y-1 text-xs text-zinc-600">
                        <div className="text-xs text-zinc-600">
                          成本 {formatCostPrice(item.effective_cost_price)}
                        </div>
                        {profit ? (
                          <>
                            <div
                              className={
                                profit.profitRate >= 0
                                  ? "text-xs font-semibold text-emerald-600"
                                  : "text-xs font-semibold text-red-600"
                              }
                            >
                              利润率 {formatSignedRate(profit.profitRate)}
                            </div>
                            <div
                              className={
                                profit.profit >= 0
                                  ? "text-xs text-emerald-600"
                                  : "text-xs text-red-600"
                              }
                            >
                              利润{" "}
                              {formatSignedCurrency(
                                profit.profit,
                                profit.currency,
                              )}
                            </div>
                          </>
                        ) : null}
                        <div className="text-xs text-zinc-500">
                          来源 {getCostSourceLabel(item.cost_price_source)}
                        </div>
                      </div>
                    </TableCell>
                  ) : null}
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div className="space-y-1 text-xs text-zinc-600">
                        <div>总库存 {inventory.total || "-"}</div>
                        <div>可用库存 {inventory.available || "-"}</div>
                        {inventory.raw ? (
                          <div className="break-all">{inventory.raw}</div>
                        ) : null}
                      </div>
                    </TableCell>
                  ) : null}
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div
                        className={`inline-flex rounded-full px-2 py-1 text-xs font-medium ${shelfStatus.className}`}
                      >
                        {shelfStatus.label}
                      </div>
                    </TableCell>
                  ) : null}
                  {skuIndex === 0 ? (
                    <TableCell className="align-top" rowSpan={skuRows.length}>
                      <div className="space-y-1 text-xs text-zinc-600">
                        {times.map((entry) => (
                          <div key={entry.label}>
                            {entry.label} {entry.value || "-"}
                          </div>
                        ))}
                      </div>
                    </TableCell>
                  ) : null}
                </TableRow>
              ));
            })
          )}
        </TableBody>
      </Table>
    </div>
  );
}
