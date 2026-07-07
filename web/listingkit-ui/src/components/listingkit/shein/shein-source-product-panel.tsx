import Link from "next/link";
import { ExternalLink } from "lucide-react";

import { Card } from "@/components/ui/card";
import { formatSDSPrice } from "@/lib/sds/format";
import type { SheinPreviewPayload } from "@/lib/types/listingkit";

function joinPath(path?: string[] | null) {
  return path?.filter(Boolean).join(" > ") ?? "";
}

function Field({
  label,
  value,
}: {
  label: string;
  value?: string | number | null;
}) {
  if (value === undefined || value === null || value === "") {
    return null;
  }

  return (
    <div className="rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2">
      <dt className="text-[11px] font-medium uppercase tracking-[0.18em] text-zinc-500">
        {label}
      </dt>
      <dd className="mt-1 break-words text-sm leading-5 text-zinc-900">
        {value}
      </dd>
    </div>
  );
}

function presentSourceLabel(label: string) {
  switch (label) {
    case "SDS SKU":
      return "SDS SKU";
    case "Category":
      return "类目";
    case "Material":
      return "材质";
    case "Variant SKU":
      return "变体 SKU";
    case "Size":
      return "尺寸";
    case "Color":
      return "颜色";
    case "Weight":
      return "重量";
    case "Production cycle":
      return "生产周期";
    case "SDS price":
      return "SDS 成本价";
    case "Print area":
      return "印刷区域";
    case "Image request":
      return "出图要求";
    default:
      return label;
  }
}

export function buildSDSSourceProductHref(
  source?: SheinPreviewPayload["source_product"] | null,
) {
  if (!source) {
    return "";
  }
  const parentProductID = normalizePositiveID(source.parent_product_id);
  if (parentProductID) {
    return `https://www.sdsdiy.com/portal/detail/${parentProductID}`;
  }
  return "";
}

export function buildSDSInternalFallbackHref(
  source?: SheinPreviewPayload["source_product"] | null,
) {
  if (!source) {
    return "";
  }
  const params = new URLSearchParams();
  const variantID = normalizePositiveID(source.variant_id);
  if (variantID) {
    params.set("variantId", variantID);
  }
  const keyword = source.sku || source.variant_sku;
  if (keyword) {
    params.set("keyword", keyword);
  }
  return params.toString() ? `/listing-kits/sds?${params.toString()}` : "";
}

function normalizePositiveID(value?: string | number | null) {
  const id = Number(value);
  if (!Number.isFinite(id) || id <= 0) {
    return "";
  }
  return String(Math.trunc(id));
}

export function SheinSourceProductPanel({
  shein,
  defaultCollapsed = false,
}: {
  shein?: SheinPreviewPayload | null;
  defaultCollapsed?: boolean;
}) {
  const source = shein?.source_product;
  if (!source) {
    return null;
  }

  const material =
    source.attributes?.material ?? source.attributes?.material_description;
  const sourceProductHref =
    buildSDSSourceProductHref(source) || buildSDSInternalFallbackHref(source);

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <details open={!defaultCollapsed}>
        <summary className="cursor-pointer list-none">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                SDS 来源商品
              </p>
              <p className="mt-1 break-words text-sm leading-6 text-zinc-700">
                {source.title}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {sourceProductHref ? (
                <Link
                  className="inline-flex items-center gap-1 rounded-full border border-zinc-200 bg-white px-3 py-1 text-[11px] font-medium text-zinc-700 transition hover:border-zinc-300 hover:bg-zinc-50"
                  href={sourceProductHref}
                  rel="noreferrer"
                  target="_blank"
                >
                  打开 SDS 商品
                  <ExternalLink className="size-3" />
                </Link>
              ) : null}
              <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-[11px] font-medium text-zinc-600">
                {defaultCollapsed ? "点击展开" : "来源详情"}
              </span>
            </div>
          </div>
        </summary>

        <dl className="mt-4 grid gap-2 sm:grid-cols-2 2xl:grid-cols-3">
          <Field label={presentSourceLabel("SDS SKU")} value={source.sku} />
          <Field label={presentSourceLabel("Category")} value={joinPath(source.category_path)} />
          <Field label={presentSourceLabel("Material")} value={material} />
          <Field label={presentSourceLabel("Variant SKU")} value={source.variant_sku} />
          <Field label={presentSourceLabel("Size")} value={source.variant_size} />
          <Field label={presentSourceLabel("Color")} value={source.variant_color} />
          <Field label={presentSourceLabel("Weight")} value={source.variant_weight ? `${source.variant_weight}g` : undefined} />
          <Field label={presentSourceLabel("Production cycle")} value={source.production_cycle ? `${source.production_cycle}h` : undefined} />
          <Field label={presentSourceLabel("SDS price")} value={source.variant_price ? formatSDSPrice(source.variant_price) : undefined} />
          <Field label={presentSourceLabel("Print area")} value={source.attributes?.design_area} />
          <Field label={presentSourceLabel("Image request")} value={source.attributes?.picture_request} />
        </dl>
      </details>
    </Card>
  );
}
