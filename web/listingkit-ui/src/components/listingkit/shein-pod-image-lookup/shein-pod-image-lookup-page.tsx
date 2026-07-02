"use client";

import Link from "next/link";
import { ExternalLink, ImageIcon, Search } from "lucide-react";
import { FormEvent, useState } from "react";

import { lookupSheinPODImages } from "@/lib/api/shein-pod-image-lookup";
import type {
  SheinPODImageLookupRecord,
  SheinPODImageLookupResponse,
} from "@/lib/types/listingkit/shein-pod-image-lookup";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const DEFAULT_LIMIT = 20;

function formatDateTime(value?: string) {
  if (!value) {
    return "-";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString("zh-CN", {
    hour12: false,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function ValueCell({ label, value }: { label: string; value?: string | number }) {
  return (
    <div className="min-w-0 space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="break-all font-mono text-sm text-foreground">{value || "-"}</p>
    </div>
  );
}

function SourceImage({
  alt,
  label,
  url,
}: {
  alt: string;
  label: string;
  url?: string;
}) {
  if (!url) {
    return (
      <div className="flex aspect-square min-h-[112px] items-center justify-center rounded-md border border-dashed border-border bg-muted/30 text-sm text-muted-foreground">
        无图片
      </div>
    );
  }

  return (
    <a
      aria-label={label}
      className="group block min-w-0"
      href={url}
      rel="noreferrer"
      target="_blank"
    >
      <div className="aspect-square overflow-hidden rounded-md border border-border bg-muted">
        <img
          alt={alt}
          className="h-full w-full object-cover transition-transform group-hover:scale-[1.03]"
          src={url}
        />
      </div>
      <span className="mt-2 inline-flex items-center gap-1 text-sm font-medium text-primary">
        {label}
        <ExternalLink className="size-3.5" />
      </span>
    </a>
  );
}

function GalleryLinks({ urls }: { urls?: string[] }) {
  const validURLs = urls?.filter((url) => url.trim()) ?? [];
  if (validURLs.length === 0) {
    return <span className="text-sm text-muted-foreground">无 SDS 画廊图</span>;
  }

  return (
    <div className="flex flex-wrap gap-2">
      {validURLs.map((url, index) => (
        <a
          className="inline-flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-sm text-foreground hover:bg-muted"
          href={url}
          key={`${url}-${index}`}
          rel="noreferrer"
          target="_blank"
        >
          画廊 {index + 1}
          <ExternalLink className="size-3.5" />
        </a>
      ))}
    </div>
  );
}

function ResultRow({ item }: { item: SheinPODImageLookupRecord }) {
  const taskHref = `/listing-kits/${item.task_id}/workspace?platform=shein`;

  return (
    <article className="grid gap-4 rounded-md border border-border bg-background p-4 lg:grid-cols-[260px_1fr]">
      <div className="grid grid-cols-2 gap-3">
        <SourceImage alt="AI 原图" label="AI 原图" url={item.ai_original_image_url} />
        <SourceImage alt="SDS 主图" label="SDS 主图" url={item.sds_main_image_url} />
      </div>

      <div className="min-w-0 space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          {item.status ? <Badge variant="neutral">{item.status}</Badge> : null}
          <Badge variant="outline">店铺 #{item.store_id || "-"}</Badge>
          <Link
            className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
            href={taskHref}
          >
            任务
            <ExternalLink className="size-3.5" />
          </Link>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          <ValueCell label="卖家 SKU" value={item.seller_sku} />
          <ValueCell label="供应商编码" value={item.supplier_code} />
          <ValueCell label="SHEIN 产品编号" value={item.shein_spu_name} />
          <ValueCell label="SHEIN 版本号" value={item.shein_version} />
          <ValueCell label="AI 原图 Key" value={item.ai_original_image_key} />
          <ValueCell label="更新时间" value={formatDateTime(item.updated_at)} />
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          <div className="min-w-0 space-y-1">
            <p className="text-xs text-muted-foreground">商品名</p>
            <p className="break-words text-sm text-foreground">
              {item.product_name || "-"}
            </p>
          </div>
          <div className="min-w-0 space-y-1">
            <p className="text-xs text-muted-foreground">生成提示词</p>
            <p className="break-words text-sm text-foreground">{item.prompt || "-"}</p>
          </div>
        </div>

        <div className="space-y-2">
          <p className="text-xs text-muted-foreground">SDS 画廊图</p>
          <GalleryLinks urls={item.sds_gallery_image_urls} />
        </div>
      </div>
    </article>
  );
}

export function SheinPODImageLookupPage() {
  const [storeId, setStoreId] = useState("869");
  const [lookupQuery, setLookupQuery] = useState("");
  const [response, setResponse] = useState<SheinPODImageLookupResponse | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const parsedStoreID = Number.parseInt(storeId.trim(), 10);
    const trimmedQuery = lookupQuery.trim();
    if (!Number.isFinite(parsedStoreID) || parsedStoreID <= 0) {
      setError("请输入有效的店铺 ID");
      return;
    }
    if (!trimmedQuery) {
      setError("请输入卖家 SKU、SHEIN 产品编号、版本号或任务 ID");
      return;
    }

    setLoading(true);
    setError("");
    try {
      setResponse(
        await lookupSheinPODImages(parsedStoreID, {
          query: trimmedQuery,
          limit: DEFAULT_LIMIT,
        }),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "查询失败");
      setResponse(null);
    } finally {
      setLoading(false);
    }
  }

  const items = response?.items ?? [];

  return (
    <div className="flex min-w-0 flex-col gap-5 py-4">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <ImageIcon className="size-5 text-muted-foreground" />
          <h1 className="text-2xl font-semibold text-foreground">POD 原图查询</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          按店铺和 SHEIN 卖家 SKU、产品编号、版本号或任务 ID 查询 POD 任务里的 AI 原图。
        </p>
      </div>

      <form
        className="grid gap-3 rounded-md border border-border bg-background p-4 md:grid-cols-[160px_1fr_auto]"
        onSubmit={handleSubmit}
      >
        <div className="space-y-2">
          <Label htmlFor="shein-pod-store-id">店铺 ID</Label>
          <Input
            id="shein-pod-store-id"
            inputMode="numeric"
            onChange={(event) => setStoreId(event.target.value)}
            value={storeId}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="shein-pod-query">查询关键词</Label>
          <Input
            id="shein-pod-query"
            onChange={(event) => setLookupQuery(event.target.value)}
            placeholder="XB0606012001V49720-T000A11F9-R4012C1-14624330"
            value={lookupQuery}
          />
        </div>
        <div className="flex items-end">
          <Button className="w-full md:w-auto" disabled={loading} type="submit">
            <Search data-icon="inline-start" />
            <span>{loading ? "查询中" : "查询"}</span>
          </Button>
        </div>
      </form>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {response ? (
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            共找到 <span className="font-medium text-foreground">{response.total}</span>{" "}
            条匹配记录
          </p>
        </div>
      ) : null}

      {response && items.length === 0 ? (
        <div className="rounded-md border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
          没有找到匹配的 POD 原图记录
        </div>
      ) : null}

      {items.length > 0 ? (
        <div className="flex flex-col gap-3">
          {items.map((item) => (
            <ResultRow item={item} key={item.task_id} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
