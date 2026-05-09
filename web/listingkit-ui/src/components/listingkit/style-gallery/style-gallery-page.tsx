"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { ExternalLink, ImageIcon, RefreshCw, Send } from "lucide-react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  buildStyleGalleryHandoff,
  saveStyleGalleryHandoff,
} from "@/lib/style-gallery/gallery-handoff";
import type {
  StyleGalleryItem,
  StyleGalleryResponse,
} from "@/lib/types/style-gallery";

type GalleryPageProps = {
  initialGallery: StyleGalleryResponse;
};

type DimensionPreset = "all" | "square" | "portrait" | "landscape" | "large";

type ImageDimensions = {
  label: string;
  width: number;
  height: number;
};

const DIMENSION_FILTER_OPTIONS: Array<{ value: DimensionPreset; label: string }> = [
  { value: "all", label: "全部尺寸" },
  { value: "square", label: "方图 1:1" },
  { value: "portrait", label: "竖图" },
  { value: "landscape", label: "横图" },
  { value: "large", label: "大图 >= 1000px" },
];

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

function sourceTone(source: string) {
  if (source.includes("mockup") || source.includes("shein")) {
    return "border-emerald-200 bg-emerald-50 text-emerald-700";
  }
  if (source.includes("legacy")) {
    return "border-amber-200 bg-amber-50 text-amber-700";
  }
  return "border-zinc-200 bg-zinc-50 text-zinc-600";
}

export function formatImageDimensions(width?: number, height?: number) {
  if (!width || !height) {
    return "";
  }
  return `${width} x ${height}px`;
}

export function StyleGalleryPage({ initialGallery }: GalleryPageProps) {
  const items = initialGallery.items;
  const [dimensionPreset, setDimensionPreset] = useState<DimensionPreset>("all");
  const [dimensionsById, setDimensionsById] = useState<Record<string, ImageDimensions>>({});
  const [minHeight, setMinHeight] = useState("");
  const [minWidth, setMinWidth] = useState("");
  const minHeightValue = Number(minHeight);
  const minWidthValue = Number(minWidth);
  const hasMinHeight = Number.isFinite(minHeightValue) && minHeightValue > 0;
  const hasMinWidth = Number.isFinite(minWidthValue) && minWidthValue > 0;
  const hasActiveDimensionFilter =
    dimensionPreset !== "all" || hasMinHeight || hasMinWidth;
  const visibleItems = useMemo(
    () =>
      items.filter((item) =>
        matchesDimensionFilter(dimensionsById[item.id], {
          preset: dimensionPreset,
          minHeight: hasMinHeight ? minHeightValue : undefined,
          minWidth: hasMinWidth ? minWidthValue : undefined,
        }),
      ),
    [
      dimensionPreset,
      dimensionsById,
      hasMinHeight,
      hasMinWidth,
      items,
      minHeightValue,
      minWidthValue,
    ],
  );

  function handleDimensions(itemId: string, dimensions: ImageDimensions) {
    setDimensionsById((current) => {
      const existing = current[itemId];
      if (
        existing?.width === dimensions.width &&
        existing.height === dimensions.height
      ) {
        return current;
      }
      return { ...current, [itemId]: dimensions };
    });
  }

  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-[radial-gradient(circle_at_12%_8%,rgba(14,165,233,0.15),transparent_28%),radial-gradient(circle_at_85%_0%,rgba(245,158,11,0.18),transparent_30%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)] px-6 py-10">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6">
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
            <Button tone="secondary" onClick={() => window.location.reload()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              刷新
            </Button>
            <Link
              href="/listing-kits/sds"
              className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
            >
              从 SDS 源生成
            </Link>
          </div>
        </section>

        <section className="grid gap-3 md:grid-cols-4">
          <MetricCard label="总数" value={initialGallery.total} />
          <MetricCard label="已保存 AI 图" value={initialGallery.summary.studioSaved} />
          <MetricCard label="已上传 AI 图" value={initialGallery.summary.publishedInputs} />
          <MetricCard label="历史图库" value={initialGallery.summary.studioLegacy} />
        </section>

        <section className="grid gap-3 rounded-[1.25rem] border border-white/70 bg-white/82 p-4 shadow-sm md:grid-cols-[minmax(12rem,0.8fr)_minmax(10rem,0.45fr)_minmax(10rem,0.45fr)_auto] md:items-end">
          <label className="space-y-1.5 text-sm">
            <span className="block text-xs font-semibold text-zinc-600">
              尺寸筛选
            </span>
            <select
              className="h-10 w-full rounded-lg border border-zinc-200 bg-white px-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 disabled:bg-zinc-100 disabled:text-zinc-400"
              disabled={items.length === 0}
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
            </select>
          </label>
          <label className="space-y-1.5 text-sm">
            <span className="block text-xs font-semibold text-zinc-600">
              最小宽度
            </span>
            <input
              className="h-10 w-full rounded-lg border border-zinc-200 bg-white px-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 disabled:bg-zinc-100 disabled:text-zinc-400"
              disabled={items.length === 0}
              inputMode="numeric"
              min={0}
              placeholder="px"
              type="number"
              value={minWidth}
              onChange={(event) => setMinWidth(event.target.value)}
            />
          </label>
          <label className="space-y-1.5 text-sm">
            <span className="block text-xs font-semibold text-zinc-600">
              最小高度
            </span>
            <input
              className="h-10 w-full rounded-lg border border-zinc-200 bg-white px-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 disabled:bg-zinc-100 disabled:text-zinc-400"
              disabled={items.length === 0}
              inputMode="numeric"
              min={0}
              placeholder="px"
              type="number"
              value={minHeight}
              onChange={(event) => setMinHeight(event.target.value)}
            />
          </label>
          <p className="text-sm text-zinc-500 md:pb-2">
            显示 {visibleItems.length} / {items.length}
          </p>
        </section>

        {items.length === 0 ? (
          <Card className="flex min-h-72 flex-col items-center justify-center border-white/70 bg-white/82 p-8 text-center">
            <ImageIcon className="h-8 w-8 text-zinc-400" />
            <h2 className="mt-4 text-lg font-semibold text-zinc-950">暂无款式图</h2>
            <p className="mt-2 max-w-md text-sm text-zinc-500">
              先从 SDS 源生成款式，或创建 ListingKit 任务后再回来查看。
            </p>
          </Card>
        ) : visibleItems.length === 0 ? (
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
        ) : (
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {visibleItems.map((item) => (
              <GalleryCard
                key={item.id}
                item={item}
                onDimensions={handleDimensions}
              />
            ))}
          </section>
        )}
      </div>
    </div>
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

function matchesDimensionFilter(
  dimensions: ImageDimensions | undefined,
  filter: {
    preset: DimensionPreset;
    minHeight?: number;
    minWidth?: number;
  },
) {
  if (
    filter.preset === "all" &&
    !filter.minHeight &&
    !filter.minWidth
  ) {
    return true;
  }
  if (!dimensions) {
    return false;
  }
  if (filter.minWidth && dimensions.width < filter.minWidth) {
    return false;
  }
  if (filter.minHeight && dimensions.height < filter.minHeight) {
    return false;
  }
  if (filter.preset === "square") {
    return Math.abs(dimensions.width - dimensions.height) <=
      Math.max(dimensions.width, dimensions.height) * 0.05;
  }
  if (filter.preset === "portrait") {
    return dimensions.height > dimensions.width;
  }
  if (filter.preset === "landscape") {
    return dimensions.width > dimensions.height;
  }
  if (filter.preset === "large") {
    return dimensions.width >= 1000 && dimensions.height >= 1000;
  }
  return true;
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
                label: formatImageDimensions(image.naturalWidth, image.naturalHeight),
                width: image.naturalWidth,
                height: image.naturalHeight,
              };
              setDimensions(nextDimensions);
              if (nextDimensions.label) {
                onDimensions(item.id, nextDimensions as ImageDimensions);
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
          <span className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${sourceTone(item.source)}`}>
            {item.sourceLabel}
          </span>
          {item.taskStatus ? (
            <span className="rounded-full bg-zinc-100 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-600">
              {item.taskStatus}
            </span>
          ) : null}
        </div>
        <div>
          <h2 className="line-clamp-1 text-base font-semibold text-zinc-950">{item.title}</h2>
          <p className="mt-1 line-clamp-1 text-xs text-zinc-500">
            {item.productName || item.prompt || item.fileName || item.id}
          </p>
        </div>
        <div className="flex items-center justify-between gap-3 text-xs text-zinc-500">
          <span>{formatDate(item.updatedAt ?? item.createdAt)}</span>
          {dimensions.label ? <span className="shrink-0">{dimensions.label}</span> : null}
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
