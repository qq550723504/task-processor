"use client";

import Link from "next/link";
import { ExternalLink, ImageIcon, RefreshCw } from "lucide-react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type {
  SheinStyleGalleryItem,
  SheinStyleGalleryResponse,
} from "@/lib/types/shein-style-gallery";

type GalleryPageProps = {
  initialGallery: SheinStyleGalleryResponse;
};

function formatDate(value?: string) {
  if (!value) {
    return "Unknown";
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

export function SheinStyleGalleryPage({ initialGallery }: GalleryPageProps) {
  const items = initialGallery.items;

  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-[radial-gradient(circle_at_12%_8%,rgba(14,165,233,0.15),transparent_28%),radial-gradient(circle_at_85%_0%,rgba(245,158,11,0.18),transparent_30%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)] px-6 py-10">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6">
        <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/80 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur lg:grid-cols-[1fr_auto] lg:items-end">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-sky-700">
              SHEIN Style Gallery
            </p>
            <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
              款式图库
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
              只展示 AI 生成的原始款式图，不混入 SDS mockup、SHEIN 草稿图或上架资料图。上架资料仍会在任务工作台中使用官方 mockup。
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button tone="secondary" onClick={() => window.location.reload()}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Refresh
            </Button>
            <Link
              href="/listing-kits/shein"
              className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
            >
              New SHEIN batch
            </Link>
          </div>
        </section>

        <section className="grid gap-3 md:grid-cols-4">
          <MetricCard label="Total" value={initialGallery.total} />
          <MetricCard label="AI saved" value={initialGallery.summary.studioSaved} />
          <MetricCard label="AI uploaded" value={initialGallery.summary.publishedInputs} />
          <MetricCard label="Studio legacy" value={initialGallery.summary.studioLegacy} />
        </section>

        {items.length === 0 ? (
          <Card className="flex min-h-72 flex-col items-center justify-center border-white/70 bg-white/82 p-8 text-center">
            <ImageIcon className="h-8 w-8 text-zinc-400" />
            <h2 className="mt-4 text-lg font-semibold text-zinc-950">暂无款式图</h2>
            <p className="mt-2 max-w-md text-sm text-zinc-500">
              先从 SHEIN Studio 生成款式，或创建 ListingKit 任务后再回来查看。
            </p>
          </Card>
        ) : (
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {items.map((item) => (
              <GalleryCard key={item.id} item={item} />
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

function GalleryCard({ item }: { item: SheinStyleGalleryItem }) {
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
          />
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
          {item.taskId ? (
            <Link
              href={`/listing-kits/${item.taskId}/workspace?platform=shein`}
              className="inline-flex items-center gap-1 font-medium text-zinc-900 hover:underline"
            >
              Workspace
              <ExternalLink className="h-3 w-3" />
            </Link>
          ) : null}
        </div>
      </div>
    </Card>
  );
}
