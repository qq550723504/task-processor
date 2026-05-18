import { useState } from "react";
import Link from "next/link";
import { ExternalLink, ImageIcon, RefreshCw, Send } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  DIMENSION_FILTER_OPTIONS,
  formatDate,
  formatImageDimensions,
  sourceTone,
  type DimensionPreset,
  type ImageDimensions,
} from "@/components/listingkit/style-gallery/style-gallery-page-model";
import {
  buildStyleGalleryHandoff,
  saveStyleGalleryHandoff,
} from "@/lib/style-gallery/gallery-handoff";
import type {
  StyleGalleryItem,
  StyleGalleryResponse,
} from "@/lib/types/style-gallery";

export function StyleGalleryHero() {
  return (
    <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/80 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur lg:grid-cols-[1fr_auto] lg:items-end">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-sky-700">
          ListingKit 款式图库
        </p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
          款式图库
        </h1>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
          只展示 AI 生成的原始款式图，不混入 SDS mockup、平台草稿图或上架资料图。上架资料仍会在任务工作台中使用官方 mockup。
        </p>
      </div>
      <div className="flex flex-wrap gap-3">
        <Button variant="secondary" onClick={() => window.location.reload()}>
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
        <Link
          href="/listing-kits/sds"
          className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
        >
          从 POD 生成
        </Link>
      </div>
    </section>
  );
}

export function StyleGalleryMetrics({
  gallery,
}: {
  gallery: StyleGalleryResponse;
}) {
  return (
    <section className="grid gap-3 md:grid-cols-4">
      <MetricCard label="总数" value={gallery.total} />
      <MetricCard label="已保存 AI 图" value={gallery.summary.studioSaved} />
      <MetricCard
        label="已上传 AI 图"
        value={gallery.summary.publishedInputs}
      />
      <MetricCard label="历史图库" value={gallery.summary.studioLegacy} />
    </section>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <Card className="border-white/70 bg-white/82 p-5">
      <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
        {label}
      </p>
      <div className="mt-3 text-3xl font-semibold tracking-[-0.04em] text-zinc-950">
        {value}
      </div>
    </Card>
  );
}

export function StyleGalleryDimensionFilters({
  dimensionPreset,
  itemCount,
  minHeight,
  minWidth,
  setDimensionPreset,
  setMinHeight,
  setMinWidth,
  visibleCount,
}: {
  dimensionPreset: DimensionPreset;
  itemCount: number;
  minHeight: string;
  minWidth: string;
  setDimensionPreset: (value: DimensionPreset) => void;
  setMinHeight: (value: string) => void;
  setMinWidth: (value: string) => void;
  visibleCount: number;
}) {
  return (
    <section className="grid gap-3 rounded-[1.25rem] border border-white/70 bg-white/82 p-4 shadow-sm md:grid-cols-[minmax(12rem,0.8fr)_minmax(10rem,0.45fr)_minmax(10rem,0.45fr)_auto] md:items-end">
      <Label className="space-y-1.5 text-sm">
        <span className="block text-xs font-semibold text-zinc-600">
          尺寸筛选
        </span>
        <Select
          className="rounded-lg"
          disabled={itemCount === 0}
          value={dimensionPreset}
          onChange={(event) =>
            setDimensionPreset(event.target.value as DimensionPreset)
          }
        >
          {DIMENSION_FILTER_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
      </Label>
      <Label className="space-y-1.5 text-sm">
        <span className="block text-xs font-semibold text-zinc-600">
          最小宽度
        </span>
        <Input
          className="rounded-lg"
          disabled={itemCount === 0}
          inputMode="numeric"
          min={0}
          placeholder="px"
          type="number"
          value={minWidth}
          onChange={(event) => setMinWidth(event.target.value)}
        />
      </Label>
      <Label className="space-y-1.5 text-sm">
        <span className="block text-xs font-semibold text-zinc-600">
          最小高度
        </span>
        <Input
          className="rounded-lg"
          disabled={itemCount === 0}
          inputMode="numeric"
          min={0}
          placeholder="px"
          type="number"
          value={minHeight}
          onChange={(event) => setMinHeight(event.target.value)}
        />
      </Label>
      <p className="text-sm text-zinc-500 md:pb-2">
        显示 {visibleCount} / {itemCount}
      </p>
    </section>
  );
}

export function StyleGalleryEmptyState() {
  return (
    <Card className="flex min-h-72 flex-col items-center justify-center border-white/70 bg-white/82 p-8 text-center">
      <ImageIcon className="h-8 w-8 text-zinc-400" />
      <h2 className="mt-4 text-lg font-semibold text-zinc-950">暂无款式图</h2>
      <p className="mt-2 max-w-md text-sm text-zinc-500">
        先从 POD 生成款式，或创建 ListingKit 任务后再回来查看。
      </p>
    </Card>
  );
}

export function StyleGalleryLoadError({
  message,
}: {
  message: string;
}) {
  return (
    <Card className="flex min-h-72 flex-col items-center justify-center border-rose-200 bg-rose-50/80 p-8 text-center">
      <ImageIcon className="h-8 w-8 text-rose-400" />
      <h2 className="mt-4 text-lg font-semibold text-zinc-950">图库加载失败</h2>
      <p className="mt-2 max-w-md text-sm text-zinc-600">{message}</p>
      <p className="mt-2 max-w-md text-xs text-zinc-500">
        这通常不是“没有生成图片”，而是图库取数失败。可以先刷新一次再看。
      </p>
    </Card>
  );
}

export function StyleGalleryNoResults({
  hasActiveDimensionFilter,
}: {
  hasActiveDimensionFilter: boolean;
}) {
  return (
    <Card className="flex min-h-64 flex-col items-center justify-center border-white/70 bg-white/82 p-8 text-center">
      <ImageIcon className="h-8 w-8 text-zinc-400" />
      <h2 className="mt-4 text-lg font-semibold text-zinc-950">
        没有符合尺寸的款式图
      </h2>
      <p className="mt-2 max-w-md text-sm text-zinc-500">
        {hasActiveDimensionFilter
          ? "调整尺寸筛选条件，或等待图片尺寸读取完成。"
          : "当前没有可展示的款式图。"}
      </p>
    </Card>
  );
}

export function StyleGalleryGrid({
  items,
  onDimensions,
}: {
  items: StyleGalleryItem[];
  onDimensions: (itemId: string, dimensions: ImageDimensions) => void;
}) {
  return (
    <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {items.map((item) => (
        <GalleryCard key={item.id} item={item} onDimensions={onDimensions} />
      ))}
    </section>
  );
}

function GalleryCard({
  item,
  onDimensions,
}: {
  item: StyleGalleryItem;
  onDimensions: (itemId: string, dimensions: ImageDimensions) => void;
}) {
  const [dimensions, setDimensions] = useState<{
    label: string;
    width?: number;
    height?: number;
  }>({ label: "" });

  function handleUseForListingTask() {
    saveStyleGalleryHandoff(
      buildStyleGalleryHandoff(item, {
        width: dimensions.width,
        height: dimensions.height,
      }),
    );
  }

  return (
    <Card className="group overflow-hidden border-white/70 bg-white/90 shadow-[0_16px_44px_rgba(39,39,42,0.07)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_60px_rgba(39,39,42,0.11)]">
      <a href={item.imageUrl} target="_blank" rel="noreferrer" className="block">
        <div className="relative aspect-square overflow-hidden bg-zinc-100">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={item.imageUrl}
            alt={item.title}
            className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.03]"
            loading="lazy"
            onLoad={(event) => {
              const image = event.currentTarget;
              const nextDimensions = {
                label: formatImageDimensions(
                  image.naturalWidth,
                  image.naturalHeight,
                ),
                width: image.naturalWidth,
                height: image.naturalHeight,
              };
              setDimensions(nextDimensions);
              if (nextDimensions.label) {
                onDimensions(item.id, nextDimensions);
              }
            }}
          />
          {dimensions.label ? (
            <span className="absolute bottom-2 left-2 rounded-full bg-zinc-950/75 px-2.5 py-1 text-[11px] font-medium text-white backdrop-blur">
              {dimensions.label}
            </span>
          ) : null}
        </div>
      </a>
      <div className="space-y-3 p-4">
        <div className="flex flex-wrap gap-2">
          <Badge
            className={`rounded-full px-2.5 py-1 text-[10px] uppercase tracking-[0.16em] ${sourceTone(item.source)}`}
            variant="outline"
          >
            {item.sourceLabel}
          </Badge>
          {item.taskStatus ? (
            <Badge
              className="rounded-full px-2.5 py-1 text-[10px] uppercase tracking-[0.16em]"
              variant="neutral"
            >
              {item.taskStatus}
            </Badge>
          ) : null}
        </div>
        <div>
          <h2 className="line-clamp-1 text-base font-semibold text-zinc-950">
            {item.title}
          </h2>
          <p className="mt-1 line-clamp-1 text-xs text-zinc-500">
            {item.productName || item.prompt || item.fileName || item.id}
          </p>
        </div>
        <div className="flex items-center justify-between gap-3 text-xs text-zinc-500">
          <span>{formatDate(item.updatedAt ?? item.createdAt)}</span>
          {dimensions.label ? (
            <span className="shrink-0">{dimensions.label}</span>
          ) : null}
          {item.taskId ? (
            <Link
              href={`/listing-kits/${item.taskId}/workspace?platform=shein`}
              className="inline-flex items-center gap-1 font-medium text-zinc-900 hover:underline"
            >
              工作区
              <ExternalLink className="h-3 w-3" />
            </Link>
          ) : null}
        </div>
        <Link
          href="/listing-kits/sds"
          prefetch={false}
          onClick={handleUseForListingTask}
          className="inline-flex h-9 w-full items-center justify-center rounded-xl bg-zinc-950 px-3 text-sm font-medium text-white transition hover:bg-zinc-800"
        >
          <Send className="mr-2 h-4 w-4" />
          用于生成任务
        </Link>
      </div>
    </Card>
  );
}
