"use client";

import dynamic from "next/dynamic";

import type { SDSProductVariantSelection } from "@/lib/types/sds";

const SheinStudioWorkbench = dynamic(
  () =>
    import("@/components/listingkit/shein-studio/shein-studio-workbench").then((module) => ({
      default: module.SheinStudioWorkbench,
    })),
  { ssr: false },
);

export function SheinStudioWorkbenchSlot({
  selection,
  workbenchKey,
}: {
  selection?: SDSProductVariantSelection;
  workbenchKey: string;
}) {
  return <SheinStudioWorkbench key={workbenchKey} selection={selection} />;
}
