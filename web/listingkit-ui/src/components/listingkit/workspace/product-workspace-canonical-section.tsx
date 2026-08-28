import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ProductWorkspaceSectionKey } from "@/components/listingkit/workspace/product-workspace-model";
import type { CanonicalAttribute, CanonicalProduct } from "@/lib/types/listingkit";

type CanonicalSectionKey = Exclude<ProductWorkspaceSectionKey, "overview">;

export function ProductWorkspaceCanonicalSection({
  section,
  product,
}: {
  section: CanonicalSectionKey;
  product?: CanonicalProduct | null;
}) {
  switch (section) {
    case "images":
      return <ImagesSection product={product} />;
    case "basic":
      return <BasicSection product={product} />;
    case "sku":
      return <SKUSection product={product} />;
    case "specs":
      return <SpecificationsSection product={product} />;
    case "attributes":
      return <AttributesSection product={product} />;
    case "description":
      return <DescriptionSection product={product} />;
  }
}

function SectionCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Card className="min-w-0 p-5">
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      <div className="mt-4">{children}</div>
    </Card>
  );
}

function ImagesSection({ product }: { product?: CanonicalProduct | null }) {
  const images = collectCanonicalImages(product);
  return (
    <SectionCard title="图片">
      {images.length === 0 ? (
        <EmptyCopy>暂无商品图片</EmptyCopy>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {images.map(({ image, alt, label, key }) => (
            <div
              className="overflow-hidden rounded-lg border border-border bg-muted/30"
              key={key}
            >
              <div className="aspect-square bg-muted">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  alt={alt}
                  className="h-full w-full object-cover"
                  src={image.url}
                />
              </div>
              <div className="border-t border-border px-3 py-2 text-xs text-muted-foreground">
                {label}
              </div>
            </div>
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function BasicSection({ product }: { product?: CanonicalProduct | null }) {
  return (
    <SectionCard title="基础信息">
      <div className="grid gap-3 sm:grid-cols-2">
        <Fact label="商品标题" value={product?.title || "暂无标题"} />
        <Fact label="品牌" value={product?.brand || "暂无品牌"} />
        <Fact
          label="分类"
          value={product?.category_path?.length ? product.category_path.join(" / ") : "暂无分类"}
        />
        <Fact
          label="商品卖点"
          value={formatStringList(product?.selling_points, "暂无商品卖点")}
        />
        <Fact
          label="SEO 关键词"
          value={formatStringList(product?.seo_keywords, "暂无 SEO 关键词")}
        />
      </div>
    </SectionCard>
  );
}

function SKUSection({ product }: { product?: CanonicalProduct | null }) {
  const variants = product?.variants ?? [];
  return (
    <SectionCard title="SKU">
      {variants.length === 0 ? (
        <EmptyCopy>暂无 SKU</EmptyCopy>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <Table className="min-w-[30rem]">
            <TableHeader className="bg-muted/60">
              <TableRow>
                <TableHead>SKU</TableHead>
                <TableHead>规格</TableHead>
                <TableHead>价格</TableHead>
                <TableHead>库存</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {variants.map((variant, index) => (
                <TableRow key={`${variant.sku ?? "variant"}-${index}`}>
                  <TableCell className="font-mono text-xs">{variant.sku || "-"}</TableCell>
                  <TableCell>{formatVariantAttributes(variant.attributes)}</TableCell>
                  <TableCell>{formatVariantPrice(variant.price)}</TableCell>
                  <TableCell>{variant.stock ?? 0}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </SectionCard>
  );
}

function SpecificationsSection({ product }: { product?: CanonicalProduct | null }) {
  const entries = flattenSpecifications(product?.specifications);
  return (
    <SectionCard title="规格">
      {entries.length === 0 ? (
        <EmptyCopy>暂无规格信息</EmptyCopy>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2">
          {entries.map((entry) => (
            <Fact key={entry.path} label={entry.label} value={entry.value} />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function AttributesSection({ product }: { product?: CanonicalProduct | null }) {
  const attributes = Object.entries(product?.attributes ?? {});
  return (
    <SectionCard title="属性">
      {attributes.length === 0 ? (
        <EmptyCopy>暂无商品属性</EmptyCopy>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2">
          {attributes.map(([name, attribute]) => (
            <AttributeFact attribute={attribute} key={name} name={name} />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function DescriptionSection({ product }: { product?: CanonicalProduct | null }) {
  return (
    <SectionCard title="描述">
      <p className="whitespace-pre-wrap text-sm leading-6 text-muted-foreground">
        {product?.description || "暂无商品描述"}
      </p>
    </SectionCard>
  );
}

function AttributeFact({
  name,
  attribute,
}: {
  name: string;
  attribute: CanonicalAttribute;
}) {
  const value = formatCanonicalAttributeValue(attribute) || "暂无值";
  return <Fact label={name} value={value} />;
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-muted/30 px-3 py-2">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className="mt-1 break-words text-sm text-foreground">{value}</div>
    </div>
  );
}

function EmptyCopy({ children }: { children: React.ReactNode }) {
  return <p className="text-sm text-muted-foreground">{children}</p>;
}

function collectCanonicalImages(product?: CanonicalProduct | null) {
  const productImages = (product?.images ?? []).map((image, index) => ({
    image,
    alt: image.alt || product?.title || `商品图片 ${index + 1}`,
    label: image.role || `图片 ${index + 1}`,
    key: `product-${image.url ?? "image"}-${index}`,
  }));
  const variantImages = (product?.variants ?? []).flatMap((variant, variantIndex) => {
    const variantLabel = variant.sku?.trim() || `变体 ${variantIndex + 1}`;
    return (variant.images ?? []).map((image, imageIndex) => ({
      image,
      alt: image.alt || `${product?.title || "商品图片"} - ${variantLabel}`,
      label: [variantLabel, image.role].filter(Boolean).join(" · "),
      key: `variant-${variantLabel}-${image.url ?? "image"}-${imageIndex}`,
    }));
  });

  return [...productImages, ...variantImages].filter(({ image }) =>
    Boolean(image.url?.trim()),
  );
}

function formatStringList(values: string[] | undefined, emptyValue: string) {
  const formatted = (values ?? [])
    .map((value) => value.trim())
    .filter(Boolean)
    .join(" · ");
  return formatted || emptyValue;
}

function formatVariantAttributes(attributes?: Record<string, CanonicalAttribute>) {
  const entries = Object.entries(attributes ?? {})
    .map(([name, attribute]) => {
      const value = formatCanonicalAttributeValue(attribute);
      return value ? `${name}: ${value}` : "";
    })
    .filter(Boolean);

  return entries.join(" · ") || "-";
}

function formatVariantPrice(price?: Record<string, unknown>) {
  const amount = Number(price?.amount);
  if (!Number.isFinite(amount)) {
    return "-";
  }

  const currency = typeof price?.currency === "string" ? price.currency.trim() : "";
  if (!currency) {
    return amount.toFixed(2);
  }

  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency,
      currencyDisplay: "code",
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount);
  } catch {
    return `${currency} ${amount.toFixed(2)}`;
  }
}

function formatCanonicalAttributeValue(attribute: CanonicalAttribute) {
  if (!attribute.value) {
    return "";
  }
  return `${attribute.value}${attribute.unit ? ` ${attribute.unit}` : ""}`;
}

function flattenSpecifications(
  value: CanonicalProduct["specifications"] | undefined,
): Array<{ path: string; label: string; value: string }> {
  const output: Array<{ path: string; label: string; value: string }> = [];

  function visit(current: unknown, path: string[]) {
    if (current === null || current === undefined || current === "") {
      return;
    }
    if (Array.isArray(current)) {
      const formatted = current.map(formatScalar).filter(Boolean).join(", ");
      if (formatted) {
        output.push({
          path: path.join("."),
          label: specificationLabel(path),
          value: formatted,
        });
      }
      return;
    }
    if (typeof current === "object") {
      for (const [key, nested] of Object.entries(current as Record<string, unknown>)) {
        visit(nested, [...path, key]);
      }
      return;
    }
    output.push({
      path: path.join("."),
      label: specificationLabel(path),
      value: formatScalar(current),
    });
  }

  visit(value, []);
  return output.filter((entry) => entry.value);
}

function specificationLabel(path: string[]) {
  const [scope, nested, leaf] = path;
  if (scope === "dimensions") {
    return `商品尺寸 · ${nested ?? "规格"}`;
  }
  if (scope === "weight") {
    return `商品重量 · ${nested ?? "规格"}`;
  }
  if (scope === "package" && nested === "dimensions") {
    return `包装尺寸 · ${leaf ?? "规格"}`;
  }
  if (scope === "package" && nested === "weight") {
    return `包装重量 · ${leaf ?? "规格"}`;
  }
  if (scope === "package") {
    return `包装 · ${nested ?? "规格"}`;
  }
  if (scope === "technical") {
    return nested ?? "技术参数";
  }
  return path.join(" · ") || "规格";
}

function formatScalar(value: unknown) {
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
}
