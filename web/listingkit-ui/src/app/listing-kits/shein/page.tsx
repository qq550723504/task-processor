import { SheinStudioPageShell } from "@/components/listingkit/shein-studio/shein-studio-page-shell";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

type SheinStudioStep = "select" | "generate" | "review" | "tasks";

function parseOptionalNumber(value?: string) {
  const parsed = Number(value ?? 0);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function parseOptionalStringArray(value?: string) {
  if (!value) {
    return undefined;
  }

  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed)) {
      return undefined;
    }

    const items = parsed.filter((item): item is string => typeof item === "string");
    return items.length > 0 ? items : undefined;
  } catch {
    return undefined;
  }
}

function parseOptionalNumberArray(value?: string) {
  if (!value) {
    return undefined;
  }
  const items = value
    .split(",")
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0);
  return items.length > 0 ? Array.from(new Set(items)) : undefined;
}

function parseStudioStep(value?: string): SheinStudioStep {
  if (
    value === "select" ||
    value === "generate" ||
    value === "review" ||
    value === "tasks"
  ) {
    return value;
  }
  return "select";
}

export default async function ListingKitSheinStudioPage({
  searchParams,
}: {
  searchParams: Promise<{
    keyword?: string;
    page?: string;
    shipmentArea?: string;
    productId?: string;
    variantId?: string;
    parentProductId?: string;
    prototypeGroupId?: string;
    layerId?: string;
    printWidth?: string;
    printHeight?: string;
    templateImageUrl?: string;
    maskImageUrl?: string;
    blankDesignUrl?: string;
    mockupImageUrl?: string;
    mockupImageUrls?: string;
    variantIds?: string;
    productName?: string;
    variantLabel?: string;
    step?: string;
  }>;
}) {
  const params = await searchParams;

  const selection: SDSProductVariantSelection | undefined = params.variantId
    ? {
        productId: parseOptionalNumber(params.productId) ?? 0,
        parentProductId:
          parseOptionalNumber(params.parentProductId) ??
          parseOptionalNumber(params.productId) ??
          0,
        variantId: parseOptionalNumber(params.variantId) ?? 0,
        prototypeGroupId: parseOptionalNumber(params.prototypeGroupId) ?? 0,
        layerId: params.layerId ?? "",
        productName: params.productName ?? "Selected SDS product",
        variantLabel: params.variantLabel ?? "Current variant",
        printableWidth: parseOptionalNumber(params.printWidth),
        printableHeight: parseOptionalNumber(params.printHeight),
        templateImageUrl: params.templateImageUrl,
        maskImageUrl: params.maskImageUrl,
        blankDesignUrl: params.blankDesignUrl,
        mockupImageUrl: params.mockupImageUrl,
        mockupImageUrls: parseOptionalStringArray(params.mockupImageUrls),
        selectedVariantIds: parseOptionalNumberArray(params.variantIds),
      }
    : undefined;

  return (
    <SheinStudioPageShell
      activeStep={
        params.step ? parseStudioStep(params.step) : selection ? "generate" : "select"
      }
      initialKeyword={params.keyword ?? ""}
      initialPage={Number(params.page ?? "1") || 1}
      initialShipmentArea={params.shipmentArea ?? "US"}
      selection={selection}
    />
  );
}
