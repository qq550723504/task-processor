import type { ComponentProps } from "react";

import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";
import { SheinCategoryReviewCard } from "@/components/listingkit/shein/shein-category-review-card";
import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";

type SheinCategoryReviewCardProps = ComponentProps<
  typeof SheinCategoryReviewCard
>;
type SheinAttributeReviewCardProps = ComponentProps<
  typeof SheinAttributeReviewCard
>;
type SheinSaleAttributeReviewCardProps = ComponentProps<
  typeof SheinSaleAttributeReviewCard
>;

export function SheinAdvancedReviewDetails({
  open,
  showCategoryReview,
  showAttributeReview,
  showSaleAttributeReview,
  categoryReviewProps,
  attributeReviewProps,
  saleAttributeReviewProps,
}: {
  open: boolean;
  showCategoryReview: boolean;
  showAttributeReview: boolean;
  showSaleAttributeReview: boolean;
  categoryReviewProps: SheinCategoryReviewCardProps;
  attributeReviewProps: SheinAttributeReviewCardProps;
  saleAttributeReviewProps: SheinSaleAttributeReviewCardProps;
}) {
  return (
    <details
      className="group rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm"
      id="shein-advanced-review-details"
      open={open}
    >
      <summary className="flex cursor-pointer list-none flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
            高级详情
          </p>
          <h2 className="mt-1 text-xl font-semibold tracking-tight text-zinc-950">
            类目和属性映射诊断
          </h2>
          <p className="mt-1 text-sm leading-6 text-zinc-600">
            {open
              ? "当前存在 SHEIN 阻断项，已经为你展开需要优先处理的类目和属性诊断。"
              : "这里是内部排查信息，默认收起。需要处理类目、普通属性或销售属性时再展开。"}
          </p>
        </div>
        <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-semibold text-zinc-600">
          {open ? "已自动展开" : "点击展开"}
        </span>
      </summary>
      <div className="mt-5 grid min-w-0 items-start gap-4 xl:grid-cols-2">
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
