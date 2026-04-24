import { SDSProductBrowser } from "@/components/listingkit/sds-product-browser";
import { TaskCreateForm } from "@/components/listingkit/task-create-form";

export default async function ListingKitSDSPage({
  searchParams,
}: {
  searchParams: Promise<{
    keyword?: string;
    page?: string;
    shipmentArea?: string;
    categoryId?: string;
    onSaleStatus?: string;
    hotSellStatus?: string;
    sort?: string;
    weightBand?: string;
    cycleBand?: string;
    productId?: string;
    variantId?: string;
    parentProductId?: string;
    prototypeGroupId?: string;
    layerId?: string;
  }>;
}) {
  const {
    keyword,
    page,
    shipmentArea,
    variantId,
    parentProductId,
    prototypeGroupId,
    layerId,
  } =
    await searchParams;

  return (
    <div className="relative isolate flex-1 overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(244,114,182,0.18),_transparent_28%),radial-gradient(circle_at_top_right,_rgba(34,197,94,0.15),_transparent_24%),linear-gradient(180deg,_#fffdf8_0%,_#f6f5ef_46%,_#eeece3_100%)]">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:28px_28px] opacity-40" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-1 flex-col gap-8 px-6 py-10 lg:px-10">
        <section className="grid gap-6 rounded-[2rem] border border-white/70 bg-white/70 px-6 py-6 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur md:grid-cols-[1.3fr_0.7fr] lg:px-8">
          <div className="space-y-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-emerald-700">
              ListingKit SDS Console
            </p>
            <div className="space-y-3">
              <h1 className="max-w-3xl font-serif text-4xl leading-tight tracking-[-0.04em] text-zinc-950 md:text-5xl">
                Browse live SDS inventory, choose a variant, and push directly into
                your sync workflow.
              </h1>
              <p className="max-w-2xl text-sm leading-7 text-zinc-600 md:text-base">
                This screen is tuned for fast operator work: filter by shipment area,
                search by SKU, inspect variants, then hand off the selected template
                to ListingKit without leaving the page.
              </p>
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-3 md:grid-cols-1">
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-zinc-950 px-5 py-4 text-white shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Shipment Area
              </div>
              <div className="mt-3 text-2xl font-semibold">{shipmentArea ?? "US"}</div>
            </div>
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Variant ID
              </div>
              <div className="mt-3 text-2xl font-semibold text-zinc-950">
                {variantId ?? "Not set"}
              </div>
            </div>
            <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
              <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
                Prototype Group
              </div>
              <div className="mt-3 text-2xl font-semibold text-zinc-950">
                {prototypeGroupId ?? "Auto"}
              </div>
            </div>
          </div>
        </section>

        <SDSProductBrowser
          initialKeyword={keyword ?? ""}
          initialPage={Number(page ?? "1") || 1}
          initialShipmentArea={shipmentArea ?? "US"}
        />
        <TaskCreateForm
          initialValues={{
            platforms: ["amazon"],
            sdsEnabled: true,
            sdsVariantId: variantId ?? "",
            sdsParentProductId: parentProductId ?? "",
            sdsPrototypeGroupId: prototypeGroupId ?? "",
            sdsLayerId: layerId ?? "",
            sdsDesignType: "material",
            sdsFitLevel: "1",
            sdsResizeMode: "0",
          }}
          variant="sds"
        />
      </div>
    </div>
  );
}
