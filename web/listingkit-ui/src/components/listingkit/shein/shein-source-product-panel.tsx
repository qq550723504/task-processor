import { Card } from "@/components/shared/card";
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

export function SheinSourceProductPanel({
  shein,
}: {
  shein?: SheinPreviewPayload | null;
}) {
  const source = shein?.source_product;
  if (!source) {
    return null;
  }

  const material =
    source.attributes?.material ?? source.attributes?.material_description;

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SDS 来源商品
          </p>
          <p className="mt-1 break-words text-sm leading-6 text-zinc-700">
            {source.title}
          </p>
        </div>

        <dl className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
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
      </div>
    </Card>
  );
}
