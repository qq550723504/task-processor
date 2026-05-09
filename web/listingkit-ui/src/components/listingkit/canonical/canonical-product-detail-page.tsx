"use client";

import Link from "next/link";
import { ArrowLeft, CheckCircle2, LoaderCircle, ShieldAlert } from "lucide-react";

import { Card } from "@/components/shared/card";
import { EmptyState } from "@/components/shared/empty-state";
import { useCanonicalProductDetail } from "@/lib/query/use-canonical-products";
import type { CanonicalAttribute, CanonicalProduct } from "@/lib/types/listingkit";

export function CanonicalProductDetailPage({ taskId }: { taskId: string }) {
  const detail = useCanonicalProductDetail(taskId);

  if (detail.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[#f7f7f3]">
        <LoaderCircle className="h-6 w-6 animate-spin text-zinc-500" />
      </div>
    );
  }

  if (detail.isError || !detail.data) {
    return (
      <div className="min-h-screen bg-[#f7f7f3] px-6 py-10">
        <EmptyState
          title="未找到 canonical product"
          description="这个任务结果中没有 canonical_product，或接口暂时不可用。"
          action={<Link href="/listing-kits/canonical-products" className="text-sm font-medium text-zinc-950 underline">返回列表</Link>}
        />
      </div>
    );
  }

  const product = detail.data.product;

  return (
    <div className="min-h-screen bg-[#f7f7f3] px-6 py-10">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-5">
        <Link href="/listing-kits/canonical-products" className="inline-flex w-fit items-center text-sm font-medium text-zinc-600 hover:text-zinc-950">
          <ArrowLeft className="mr-2 h-4 w-4" />
          返回 canonical product 列表
        </Link>

        <section className="grid gap-5 border-b border-zinc-200 pb-6 lg:grid-cols-[1fr_320px]">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-teal-700">
              Canonical Detail
            </p>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight text-zinc-950">
              {product.title || detail.data.taskId}
            </h1>
            <p className="mt-2 text-sm text-zinc-600">
              {product.brand || "未知品牌"} · {product.category_path?.join(" / ") || "未分类"}
            </p>
          </div>
          <Card className="p-4">
            <div className="grid grid-cols-3 gap-3 text-center">
              <Metric label="图片" value={detail.data.summary.imageCount} />
              <Metric label="变体" value={detail.data.summary.variantCount} />
              <Metric label="需审核字段" value={detail.data.reviewFieldCount} />
            </div>
          </Card>
        </section>

        <div className="grid gap-5 lg:grid-cols-[360px_1fr]">
          <Card className="overflow-hidden">
            <div className="aspect-square bg-zinc-50">
              {detail.data.summary.imageUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={detail.data.summary.imageUrl} alt="" className="h-full w-full object-cover" />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-sm text-zinc-400">
                  No image
                </div>
              )}
            </div>
            <div className="p-4 text-sm text-zinc-600">
              <div className="break-all font-mono text-xs text-zinc-400">{detail.data.taskId}</div>
              <div className="mt-3 flex flex-wrap gap-2">
                {detail.data.summary.needsReview ? (
                  <span className="inline-flex items-center rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700">
                    <ShieldAlert className="mr-1 h-3.5 w-3.5" />
                    需要人工审核
                  </span>
                ) : (
                  <span className="inline-flex items-center rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700">
                    <CheckCircle2 className="mr-1 h-3.5 w-3.5" />
                    字段可信
                  </span>
                )}
              </div>
            </div>
          </Card>

          <div className="grid gap-5">
            <CanonicalDescription product={product} />
            <CanonicalAttributes product={product} />
            <CanonicalVariants product={product} />
            <CanonicalFieldTraces traces={detail.data.fieldTraces} />
          </div>
        </div>
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <div className="text-2xl font-semibold text-zinc-950">{value}</div>
      <div className="mt-1 text-xs text-zinc-500">{label}</div>
    </div>
  );
}

function CanonicalDescription({ product }: { product: CanonicalProduct }) {
  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold text-zinc-950">基础信息</h2>
      <p className="mt-3 whitespace-pre-wrap text-sm leading-6 text-zinc-600">
        {product.description || "暂无描述"}
      </p>
    </Card>
  );
}

function CanonicalAttributes({ product }: { product: CanonicalProduct }) {
  const attributes = Object.entries(product.attributes ?? {});
  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold text-zinc-950">属性</h2>
      <div className="mt-4 grid gap-2 md:grid-cols-2">
        {attributes.length === 0 ? (
          <p className="text-sm text-zinc-500">暂无属性</p>
        ) : attributes.map(([key, attr]) => (
          <AttributeRow key={key} name={key} attr={attr} />
        ))}
      </div>
    </Card>
  );
}

function AttributeRow({ name, attr }: { name: string; attr: CanonicalAttribute }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
      <div className="text-xs font-medium uppercase tracking-[0.14em] text-zinc-400">{name}</div>
      <div className="mt-1 text-sm text-zinc-800">{attr.value || "空值"}{attr.unit ? ` ${attr.unit}` : ""}</div>
    </div>
  );
}

function CanonicalVariants({ product }: { product: CanonicalProduct }) {
  const variants = product.variants ?? [];
  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold text-zinc-950">变体</h2>
      <div className="mt-4 overflow-hidden rounded-lg border border-zinc-200">
        <table className="w-full text-left text-sm">
          <thead className="bg-zinc-50 text-xs uppercase tracking-[0.14em] text-zinc-500">
            <tr>
              <th className="px-3 py-2">SKU</th>
              <th className="px-3 py-2">标题</th>
              <th className="px-3 py-2">库存</th>
            </tr>
          </thead>
          <tbody>
            {variants.length === 0 ? (
              <tr><td colSpan={3} className="px-3 py-4 text-zinc-500">暂无变体</td></tr>
            ) : variants.map((variant, index) => (
              <tr key={`${variant.sku ?? "variant"}-${index}`} className="border-t border-zinc-200">
                <td className="px-3 py-2 font-mono text-xs text-zinc-700">{variant.sku || "-"}</td>
                <td className="px-3 py-2 text-zinc-700">{variant.title || "-"}</td>
                <td className="px-3 py-2 text-zinc-700">{variant.stock ?? "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function CanonicalFieldTraces({
  traces,
}: {
  traces: Array<{ field: string; trace: { needs_review?: boolean; confidence?: number; review_reason?: string } }>;
}) {
  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold text-zinc-950">字段证据</h2>
      <div className="mt-4 grid gap-2">
        {traces.length === 0 ? (
          <p className="text-sm text-zinc-500">暂无字段追踪</p>
        ) : traces.map(({ field, trace }) => (
          <div key={field} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-white px-3 py-2">
            <div>
              <div className="text-sm font-medium text-zinc-900">{field}</div>
              {trace.review_reason ? <div className="mt-1 text-xs text-zinc-500">{trace.review_reason}</div> : null}
            </div>
            <div className="flex items-center gap-2 text-xs text-zinc-500">
              <span>{Math.round((trace.confidence ?? 0) * 100)}%</span>
              <span className={trace.needs_review ? "text-amber-700" : "text-emerald-700"}>
                {trace.needs_review ? "需审核" : "可信"}
              </span>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}
