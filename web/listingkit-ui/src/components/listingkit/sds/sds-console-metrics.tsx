"use client";

import { useLiveSearchParams } from "@/lib/utils/live-search-params";

type SDSConsoleMetricsProps = {
  initialShipmentArea: string;
  initialVariantId: string;
  initialPrototypeGroupId: string;
};

export function SDSConsoleMetrics({
  initialShipmentArea,
  initialVariantId,
  initialPrototypeGroupId,
}: SDSConsoleMetricsProps) {
  const searchParams = useLiveSearchParams();
  const shipmentArea = searchParams.get("shipmentArea") ?? initialShipmentArea;
  const variantId = searchParams.get("variantId") ?? initialVariantId;
  const prototypeGroupId =
    searchParams.get("prototypeGroupId") ?? initialPrototypeGroupId;

  return (
    <div className="grid gap-3 sm:grid-cols-3 md:grid-cols-1">
      <div className="rounded-[1.5rem] border border-zinc-200/80 bg-zinc-950 px-5 py-4 text-white shadow-sm">
        <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
          Shipment Area
        </div>
        <div className="mt-3 text-2xl font-semibold">{shipmentArea || "US"}</div>
      </div>
      <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
        <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
          Variant ID
        </div>
        <div className="mt-3 text-2xl font-semibold text-zinc-950">
          {variantId || "Not set"}
        </div>
      </div>
      <div className="rounded-[1.5rem] border border-zinc-200/80 bg-white px-5 py-4 shadow-sm">
        <div className="text-[11px] uppercase tracking-[0.28em] text-zinc-400">
          Prototype Group
        </div>
        <div className="mt-3 text-2xl font-semibold text-zinc-950">
          {prototypeGroupId || "Auto"}
        </div>
      </div>
    </div>
  );
}
