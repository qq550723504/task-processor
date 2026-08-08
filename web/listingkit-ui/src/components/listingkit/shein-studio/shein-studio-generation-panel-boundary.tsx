import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import type { SheinStudioGenerationPanelProps } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import {
  buildSheinStudioGenerationPanelProps,
  type SheinStudioGenerationPanelProjectionInput,
} from "@/components/listingkit/shein-studio/shein-studio-generation-controller";

type SheinStudioGenerationPanelBoundaryProps = {
  input: SheinStudioGenerationPanelProjectionInput;
  promptInputRef: SheinStudioGenerationPanelProps["promptInputRef"];
};

export function SheinStudioGenerationPanelBoundary({
  input,
  promptInputRef,
}: SheinStudioGenerationPanelBoundaryProps) {
  const panelProps = buildSheinStudioGenerationPanelProps(input);

  return (
    <SheinStudioGenerationPanel
      {...panelProps}
      promptInputRef={promptInputRef}
    />
  );
}
