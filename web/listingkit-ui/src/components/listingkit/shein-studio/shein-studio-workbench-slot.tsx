"use client";

import dynamic from "next/dynamic";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

const SheinStudioWorkbench = dynamic(
  () =>
    import("@/components/listingkit/shein-studio/shein-studio-workbench").then((module) => ({
      default: module.SheinStudioWorkbench,
    })),
  { ssr: false },
);

export function SheinStudioWorkbenchSlot({
  activeStep,
  selection,
  workbenchKey,
}: {
  activeStep: SheinStudioStepKey;
  selection?: SDSProductVariantSelection;
  workbenchKey: string;
}) {
  return (
    <SheinStudioWorkbench
      activeStep={activeStep}
      key={workbenchKey}
      selection={selection}
    />
  );
}
