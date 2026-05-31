"use client";

import Link from "next/link";
import { ArrowLeft, CheckCircle2, Images, LoaderCircle, ShieldAlert } from "lucide-react";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyState } from "@/components/shared/empty-state";
import { useCanonicalProductDetail } from "@/lib/query/use-canonical-products";
import type { CanonicalAttribute, CanonicalProduct } from "@/lib/types/listingkit";

export function CanonicalProductDetailPage({ taskId }: { taskId: string }) {
  const detail = useCanonicalProductDetail(taskId);

  if (detail.isLoading) {
    return (
      <ListingKitPageShell
        backgroundClassName="bg-[#f7f7f3]"
        contentClassName="items-center justify-center"
      >
        <LoaderCircle className="h-6 w-6 animate-spin text-zinc-500" />
      </ListingKitPageShell>
    );
  }

  if (detail.isError || !detail.data) {
    return (
      <ListingKitPageShell backgroundClassName="bg-[#f7f7f3]">
        <EmptyState
          title="未找到标准商品"
          description="这个任务结果中没有 canonical_product，或接口暂时不可用。"
          action={<Link href="/listing-kits/canonical-products" className="text-sm font-medium text-zinc-950 underline">返回列表</Link>}
        />
      </ListingKitPageShell>
    );
  }

  const product = detail.data.product;

  return (
    <ListingKitPageShell backgroundClassName="bg-[#f7f7f3]" contentClassName="gap-5">
      <Link href="/listing-kits/canonical-products" className="inline-flex w-fit items-center text-sm font-medium text-zinc-600 hover:text-zinc-950">
        <ArrowLeft className="mr-2 h-4 w-4" />
        返回标准商品列表
      </Link>

      <section className="grid gap-5 border-b border-zinc-200 pb-6 xl:grid-cols-[1fr_320px]">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-teal-700">
            标准商品详情
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight text-zinc-950">
            {product.title || detail.data.taskId}
          </h1>
          <p className="mt-2 text-sm text-zinc-600">
            {product.brand || "未知品牌"} · {product.category_path?.join(" / ") || "未分类"}
          </p>
        </div>
        <Card className="p-4">
          <div className="grid gap-3 text-center sm:grid-cols-3">
            <Metric label="图片" value={detail.data.summary.imageCount} />
            <Metric label="变体" value={detail.data.summary.variantCount} />
            <Metric label="需审核字段" value={detail.data.reviewFieldCount} />
          </div>
        </Card>
      </section>

      <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
        <Card className="overflow-hidden">
          <CanonicalImages product={product} />
          <div className="p-4 text-sm text-zinc-600">
            <div className="break-all font-mono text-xs text-zinc-400">{detail.data.taskId}</div>
            <div className="mt-3 flex flex-wrap gap-2">
              {detail.data.summary.needsReview ? (
                <Badge className="gap-1 rounded-full" variant="warning">
                  <ShieldAlert className="mr-1 h-3.5 w-3.5" />
                  需要人工审核
                </Badge>
              ) : (
                <Badge className="gap-1 rounded-full" variant="success">
                  <CheckCircle2 className="mr-1 h-3.5 w-3.5" />
                  字段可信
                </Badge>
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
    </ListingKitPageShell>
  );
}

function CanonicalImages({ product }: { product: CanonicalProduct }) {
  const images = uniqueCanonicalImages(product);
  const primaryImage = images[0];

  return (
    <div>
      <div className="aspect-square bg-zinc-50">
        {primaryImage ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={primaryImage.url}
            alt={primaryImage.alt || product.title || ""}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm text-zinc-400">
            No image
          </div>
        )}
      </div>

      <Separator />
      <div className="p-4">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center text-sm font-semibold text-zinc-950">
            <Images className="mr-2 h-4 w-4 text-teal-700" />
            图片
          </div>
          <span className="text-xs text-zinc-500">{images.length} 张</span>
        </div>
        {images.length > 0 ? (
          <div className="mt-3 grid gap-2 sm:grid-cols-3">
            {images.map((image, index) => (
              <a
                key={`${image.url}-${index}`}
                href={image.url}
                target="_blank"
                rel="noreferrer"
                className="group overflow-hidden rounded-md border border-zinc-200 bg-zinc-50"
                title={image.url}
              >
                <div className="aspect-square">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={image.url}
                    alt={image.alt || product.title || ""}
                    className="h-full w-full object-cover transition group-hover:scale-105"
                  />
                </div>
                <div className="truncate border-t border-zinc-200 px-2 py-1 text-[11px] text-zinc-500">
                  {image.role || `图片 ${index + 1}`}
                </div>
              </a>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function uniqueCanonicalImages(product: CanonicalProduct) {
  const seen = new Set<string>();
  return (product.images ?? [])
    .map((image) => ({
      url: image.url?.trim() ?? "",
      alt: image.alt,
      role: image.role,
    }))
    .filter((image) => {
      if (!image.url || seen.has(image.url)) {
        return false;
      }
      seen.add(image.url);
      return true;
    });
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
      <div className="mt-4 overflow-x-auto rounded-lg border border-zinc-200">
        <Table className="min-w-[32rem]">
          <TableHeader className="bg-zinc-50">
            <TableRow className="text-xs uppercase tracking-[0.14em] hover:bg-transparent">
              <TableHead className="px-3 py-2">SKU</TableHead>
              <TableHead className="px-3 py-2">标题</TableHead>
              <TableHead className="px-3 py-2">库存</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {variants.length === 0 ? (
              <TableRow><TableCell colSpan={3} className="px-3 py-4 text-zinc-500">暂无变体</TableCell></TableRow>
            ) : variants.map((variant, index) => (
              <TableRow key={`${variant.sku ?? "variant"}-${index}`}>
                <TableCell className="px-3 py-2 font-mono text-xs text-zinc-700">{variant.sku || "-"}</TableCell>
                <TableCell className="px-3 py-2 text-zinc-700">{variant.title || "-"}</TableCell>
                <TableCell className="px-3 py-2 text-zinc-700">{variant.stock ?? "-"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
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
          <div key={field} className="flex flex-col gap-3 rounded-lg border border-zinc-200 bg-white px-3 py-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
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
