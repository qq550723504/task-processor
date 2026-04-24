import Link from "next/link";

import { SDSProductBrowser } from "@/components/listingkit/sds-product-browser";
import { SheinStudioWorkbenchSlot } from "@/components/listingkit/shein-studio-workbench-slot";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

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
    productName?: string;
    variantLabel?: string;
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
      }
    : undefined;
  const workbenchKey = `${selection?.variantId ?? 0}:${selection?.prototypeGroupId ?? 0}:${selection?.layerId ?? ""}`;

  return (
    <div className="relative isolate flex-1 overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(251,146,60,0.18),_transparent_26%),radial-gradient(circle_at_top_right,_rgba(236,72,153,0.14),_transparent_24%),linear-gradient(180deg,_#fffdf9_0%,_#f7f3ee_46%,_#efebe4_100%)]">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.032)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.032)_1px,transparent_1px)] bg-[size:30px_30px] opacity-40" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 px-6 py-10 lg:px-10">
        <section className="grid gap-6 rounded-[2rem] border border-white/70 bg-white/72 px-6 py-6 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur md:grid-cols-[1.2fr_0.8fr] lg:px-8">
          <div className="space-y-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-rose-700">
              SHEIN Studio
            </p>
            <div className="space-y-3">
              <h1 className="max-w-3xl font-serif text-4xl leading-tight tracking-[-0.04em] text-zinc-950 md:text-5xl">
                Generate multiple apparel design styles, preview them on SDS
                templates, then send approved ones into SHEIN review.
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-zinc-600 md:text-base">
                Pick the POD product first. The printable area is read from SDS, the
                theme is controlled by one prompt, and approved styles are converted
                into normal ListingKit SHEIN tasks for final review and publish.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/listing-kits/shein/gallery"
                className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
              >
                View style gallery
              </Link>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 md:grid-cols-1">
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-zinc-950 px-5 py-4 text-white shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Shipment Area
              </div>
              <div className="mt-3 text-2xl font-semibold">
                {params.shipmentArea ?? "US"}
              </div>
            </div>
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Variant
              </div>
              <div className="mt-3 text-2xl font-semibold text-zinc-950">
                {params.variantId ?? "Pending"}
              </div>
            </div>
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Printable Area
              </div>
              <div className="mt-3 text-2xl font-semibold text-zinc-950">
                {selection?.printableWidth && selection?.printableHeight
                  ? `${selection.printableWidth}×${selection.printableHeight}`
                  : "Auto"}
              </div>
            </div>
          </div>
        </section>

        <SDSProductBrowser
          initialKeyword={params.keyword ?? ""}
          initialPage={Number(params.page ?? "1") || 1}
          initialShipmentArea={params.shipmentArea ?? "US"}
        />
        <SheinStudioWorkbenchSlot
          selection={selection}
          workbenchKey={workbenchKey}
        />
      </div>
    </div>
  );
}
