"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ArrowRight, Database, LoaderCircle, RefreshCw, ShieldAlert } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/shared/empty-state";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { useCanonicalProducts } from "@/lib/query/use-canonical-products";
import type { CanonicalProductListItem } from "@/lib/canonical-products/canonical-products";

function formatDate(value?: string) {
  if (!value) {
    return "未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function CanonicalProductListPage() {
  const searchParams = useSearchParams();
  const page = Number(searchParams.get("page") ?? "1") || 1;
  const products = useCanonicalProducts({ page, page_size: 30 });
  const items = products.data?.items ?? [];

  return (
    <ListingKitPageShell backgroundClassName="bg-[#f7f7f3]">
      <section className="grid gap-4 border-b border-zinc-200 pb-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-teal-700">
            标准商品
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight text-zinc-950">
            标准商品
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
            从最近的 ListingKit 任务结果中聚合标准商品，用于检查 1688、POD 到 SHEIN 前的统一商品事实。
          </p>
        </div>
        <Button variant="secondary" onClick={() => products.refetch()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
      </section>

      <Card className="border-zinc-200 bg-white p-4">
        <div className="flex flex-wrap items-center gap-3 text-sm text-zinc-600">
          <Database className="h-4 w-4 text-teal-700" />
          <span>当前页 {items.length} 个标准商品</span>
          <span className="text-zinc-300">/</span>
          <span>来源：ListingKit task result canonical_product</span>
        </div>
      </Card>

      {products.isLoading ? (
        <Card className="flex min-h-72 items-center justify-center">
          <LoaderCircle className="h-6 w-6 animate-spin text-zinc-500" />
        </Card>
      ) : products.isError ? (
        <EmptyState
          title="标准商品加载失败"
          description="任务列表或任务详情接口暂时不可用。"
          action={
            <Button variant="secondary" onClick={() => products.refetch()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              刷新
            </Button>
          }
        />
      ) : items.length === 0 ? (
        <EmptyState
          title="暂无标准商品"
          description="完成或待审核的 ListingKit 任务产出标准商品后会出现在这里。"
          action={<Link className="text-sm font-medium text-zinc-950 underline" href="/listing-kits">查看任务列表</Link>}
        />
      ) : (
        <div className="grid gap-3">
          {items.map((item) => (
            <CanonicalProductRow key={item.taskId} item={item} />
          ))}
        </div>
      )}
    </ListingKitPageShell>
  );
}

function CanonicalProductRow({ item }: { item: CanonicalProductListItem }) {
  return (
    <Card className="border-zinc-200 bg-white p-4 transition hover:border-zinc-300 hover:shadow-md">
      <div className="grid gap-4 lg:grid-cols-[72px_1fr_auto] lg:items-center">
        <div className="h-[72px] w-[72px] overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50">
          {item.imageUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={item.imageUrl} alt="" className="h-full w-full object-cover" />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xs text-zinc-400">
              No image
            </div>
          )}
        </div>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            {item.needsReview ? (
              <Badge className="gap-1 rounded-full text-[11px]" variant="warning">
                <ShieldAlert className="mr-1 h-3.5 w-3.5" />
                需审核
              </Badge>
            ) : (
              <Badge className="rounded-full text-[11px]" variant="success">
                已校验
              </Badge>
            )}
            {item.platformLabels.map((platform) => (
              <Badge key={platform} className="rounded-full text-[11px] uppercase tracking-[0.14em]" variant="neutral">
                {platform}
              </Badge>
            ))}
          </div>
          <h2 className="mt-2 truncate text-lg font-semibold text-zinc-950">{item.title}</h2>
          <p className="mt-1 text-sm text-zinc-500">
            {item.brand || "未知品牌"} · {item.categoryPath.join(" / ") || "未分类"}
          </p>
          <p className="mt-1 text-xs text-zinc-400">
            {item.imageCount} 张图片 · {item.variantCount} 个变体 · {formatDate(item.completedAt ?? item.createdAt)}
          </p>
        </div>
        <Link
          href={`/listing-kits/canonical-products/${item.taskId}`}
          className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
        >
          详情
          <ArrowRight className="ml-2 h-4 w-4" />
        </Link>
      </div>
    </Card>
  );
}
