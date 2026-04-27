"use client";

import { useState } from "react";

import { SDSProductBrowser } from "@/components/listingkit/sds/sds-product-browser";
import { Button } from "@/components/shared/button";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SheinProductPickerModal({
  initialKeyword,
  initialPage,
  initialShipmentArea,
  selection,
}: {
  initialKeyword: string;
  initialPage: number;
  initialShipmentArea: string;
  selection?: SDSProductVariantSelection;
}) {
  const [open, setOpen] = useState(false);

  if (!selection?.variantId) {
    return (
      <section id="sds-product-browser" className="scroll-mt-6">
        <SDSProductBrowser
          initialKeyword={initialKeyword}
          initialPage={initialPage}
          initialShipmentArea={initialShipmentArea}
        />
      </section>
    );
  }

  return (
    <>
      <section
        id="sds-product-browser"
        className="scroll-mt-6 rounded-[1.75rem] border border-emerald-200 bg-emerald-50/70 p-4 shadow-sm"
      >
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700">
              Selected SDS product
            </p>
            <h2 className="mt-1 truncate text-lg font-semibold text-zinc-950">
              {selection.productName}
            </h2>
            <p className="mt-1 text-sm text-zinc-600">
              Variant {selection.variantId} · {selection.variantLabel}
              {selection.printableWidth && selection.printableHeight
                ? ` · ${selection.printableWidth}×${selection.printableHeight}px`
                : ""}
            </p>
          </div>
          <Button onClick={() => setOpen(true)} tone="secondary">
            Change product
          </Button>
        </div>
      </section>

      {open ? (
        <div
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-zinc-950/55 px-4 py-6 backdrop-blur-sm"
          role="dialog"
        >
          <div className="w-full max-w-6xl rounded-[2rem] bg-[#f7f3ee] p-4 shadow-2xl">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-[1.5rem] bg-white px-5 py-4">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
                  Change SDS product
                </p>
                <h2 className="mt-1 text-xl font-semibold text-zinc-950">
                  Pick another product or variant.
                </h2>
              </div>
              <Button onClick={() => setOpen(false)} tone="ghost">
                Close
              </Button>
            </div>
            <SDSProductBrowser
              initialKeyword={initialKeyword}
              initialPage={initialPage}
              initialShipmentArea={initialShipmentArea}
            />
          </div>
        </div>
      ) : null}
    </>
  );
}
