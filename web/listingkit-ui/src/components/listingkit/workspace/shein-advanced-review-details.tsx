import type { ComponentProps } from "react";

import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";
import { SheinCategoryReviewCard } from "@/components/listingkit/shein/shein-category-review-card";
import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";
import { Badge } from "@/components/ui/badge";

type SheinCategoryReviewCardProps = ComponentProps<
  typeof SheinCategoryReviewCard
>;
type SheinAttributeReviewCardProps = ComponentProps<
  typeof SheinAttributeReviewCard
>;
type SheinSaleAttributeReviewCardProps = ComponentProps<
  typeof SheinSaleAttributeReviewCard
>;
type SheinRefreshHistoryEntry = {
  scope: "category" | "attribute" | "sale_attribute";
  status: "running" | "completed";
  title: string;
  detail: string;
  affectedAreas: string[];
  occurredAt: string;
};

function formatRefreshTime(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function scopeLabel(scope: SheinRefreshHistoryEntry["scope"]) {
  switch (scope) {
    case "category":
      return "类目";
    case "attribute":
      return "普通属性";
    case "sale_attribute":
      return "销售属性";
    default:
      return scope;
  }
}

export function SheinAdvancedReviewDetails({
  open,
  showCategoryReview,
  showAttributeReview,
  showSaleAttributeReview,
  refreshHistory = [],
  categoryReviewProps,
  attributeReviewProps,
  saleAttributeReviewProps,
}: {
  open: boolean;
  showCategoryReview: boolean;
  showAttributeReview: boolean;
  showSaleAttributeReview: boolean;
  refreshHistory?: SheinRefreshHistoryEntry[];
  categoryReviewProps: SheinCategoryReviewCardProps;
  attributeReviewProps: SheinAttributeReviewCardProps;
  saleAttributeReviewProps: SheinSaleAttributeReviewCardProps;
}) {
  return (
    <details
      className="group rounded-[1.75rem] border border-border bg-card p-5 shadow-sm"
      id="shein-advanced-review-details"
      open={open}
    >
      <summary className="flex cursor-pointer list-none flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
            高级详情
          </p>
          <h2 className="mt-1 text-xl font-semibold tracking-tight text-foreground">
            类目和属性映射诊断
          </h2>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">
            {open
              ? "当前存在 SHEIN 阻断项，已经为你展开需要优先处理的类目和属性诊断。"
              : "这里是内部排查信息，默认收起。需要处理类目、普通属性或销售属性时再展开。"}
          </p>
        </div>
        <Badge className="rounded-full px-3 py-1 text-xs" variant="neutral">
          {open ? "已自动展开" : "点击展开"}
        </Badge>
      </summary>
      {refreshHistory.length ? (
        <details className="mt-5 rounded-2xl border border-border bg-muted/80 p-4">
          <summary className="cursor-pointer list-none text-sm font-semibold text-foreground">
            查看本次刷新轨迹（{refreshHistory.length}）
          </summary>
          <div className="mt-3 space-y-3">
            {refreshHistory.map((entry, index) => (
              <article
                className="rounded-2xl border border-border/80 bg-background px-4 py-3"
                key={`${entry.scope}-${entry.status}-${entry.occurredAt}-${index}`}
              >
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div className="text-sm font-medium text-foreground">
                    {entry.title}
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge
                      className="rounded-full px-2 py-0.5 text-[11px]"
                      variant={entry.status === "completed" ? "success" : "neutral"}
                    >
                      {entry.status === "completed" ? "已触发" : "进行中"}
                    </Badge>
                    {entry.occurredAt ? (
                      <div className="text-xs text-muted-foreground">
                        {formatRefreshTime(entry.occurredAt)}
                      </div>
                    ) : null}
                  </div>
                </div>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {entry.detail}
                </p>
                <p className="mt-2 text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
                  当前操作 {scopeLabel(entry.scope)}
                  {entry.affectedAreas.length
                    ? ` · 将重算 ${entry.affectedAreas.join(" / ")}`
                    : ""}
                </p>
              </article>
            ))}
          </div>
        </details>
      ) : null}
      <div className="mt-5 grid min-w-0 items-start gap-4 2xl:grid-cols-2">
        {showCategoryReview ? (
          <div id="shein-category-review-card" className="min-w-0">
            <SheinCategoryReviewCard {...categoryReviewProps} />
          </div>
        ) : null}
        {showAttributeReview ? (
          <div id="shein-attribute-review-card" className="min-w-0">
            <SheinAttributeReviewCard {...attributeReviewProps} />
          </div>
        ) : null}
        {showSaleAttributeReview ? (
          <div id="shein-sale-attribute-review-card" className="min-w-0">
            <SheinSaleAttributeReviewCard {...saleAttributeReviewProps} />
          </div>
        ) : null}
      </div>
    </details>
  );
}
