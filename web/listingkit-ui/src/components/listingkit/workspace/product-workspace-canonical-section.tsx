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
  const images = (product?.images ?? []).filter((image) => Boolean(image.url?.trim()));
  return (
    <SectionCard title="图片">
      {images.length === 0 ? (
        <EmptyCopy>暂无商品图片</EmptyCopy>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {images.map((image, index) => (
            <div
              className="overflow-hidden rounded-lg border border-border bg-muted/30"
              key={`${image.url}-${index}`}
            >
              <div className="aspect-square bg-muted">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  alt={image.alt || product?.title || `商品图片 ${index + 1}`}
                  className="h-full w-full object-cover"
                  src={image.url}
                />
              </div>
              <div className="border-t border-border px-3 py-2 text-xs text-muted-foreground">
                {image.role || `图片 ${index + 1}`}
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
                <TableHead>标题</TableHead>
                <TableHead>库存</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {variants.map((variant, index) => (
                <TableRow key={`${variant.sku ?? "variant"}-${index}`}>
                  <TableCell className="font-mono text-xs">{variant.sku || "-"}</TableCell>
                  <TableCell>{variant.title || "-"}</TableCell>
                  <TableCell>{variant.stock ?? "-"}</TableCell>
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
  const value = attribute.value
    ? `${attribute.value}${attribute.unit ? ` ${attribute.unit}` : ""}`
    : "暂无值";
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
        output.push({ path: path.join("."), label: path.at(-1) ?? "规格", value: formatted });
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
      label: path.at(-1) ?? "规格",
      value: formatScalar(current),
    });
  }

  visit(value, []);
  return output.filter((entry) => entry.value);
}

function formatScalar(value: unknown) {
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return "";
}
