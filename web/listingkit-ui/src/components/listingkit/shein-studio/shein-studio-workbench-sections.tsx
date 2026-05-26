import { Button } from "@/components/ui/button";
import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";
import { buildGroupedGenerationTargets } from "@/lib/shein-studio/grouped-image-mode";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
} from "@/lib/types/shein-studio";

export function SheinStudioWorkbenchAlerts({
  draftWarning,
  generationWarning,
  generationWarningAction,
  galleryRatioCheck,
}: {
  draftWarning: string;
  generationWarning: string;
  generationWarningAction:
    | {
        intent: "focus_generate" | "warm_baseline";
        label: string;
        onClick: () => void;
      }
    | null;
  galleryRatioCheck: SDSRatioMatch | null;
}) {
  return (
    <>
      {generationWarning ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span>{generationWarning}</span>
            {generationWarningAction ? (
              <Button
                className="shrink-0"
                onClick={generationWarningAction.onClick}
                size="sm"
                type="button"
                variant="secondary"
              >
                {generationWarningAction.label}
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}

      {draftWarning ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          {draftWarning}
        </div>
      ) : null}

      {galleryRatioCheck && galleryRatioCheck.status !== "pass" ? (
        <div
          className={
            galleryRatioCheck.status === "blocking"
              ? "rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-900"
              : "rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900"
          }
        >
          {galleryRatioCheck.message}
        </div>
      ) : null}
    </>
  );
}

export function SheinStudioReviewStep({
  createdTaskCount,
  createActionDisabledReason,
  designs,
  groupedImageMode,
  groupedSelections,
  imageStrategy,
  isCreatingTasks,
  onBackToGenerate,
  onCreateReviewTasks,
  onNoteChange,
  onRegenerate,
  onToggle,
  productImageCount,
  regeneratingId,
  renderSizeImagesWithSds,
  selectedIds,
  selection,
}: {
  createdTaskCount: number;
  createActionDisabledReason?: string;
  designs: SheinStudioGeneratedDesign[];
  groupedImageMode: SheinStudioGroupedImageMode;
  groupedSelections: GroupedSDSSelectionEligibility[];
  imageStrategy: SheinStudioImageStrategy;
  isCreatingTasks: boolean;
  onBackToGenerate: () => void;
  onCreateReviewTasks: () => void;
  onNoteChange: (designId: string, note: string) => void;
  onRegenerate: (designId: string) => void;
  onToggle: (designId: string) => void;
  productImageCount: string;
  regeneratingId?: string;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selection?: SDSProductVariantSelection;
}) {
  const selectionByTargetGroupKey = new Map(
    buildGroupedGenerationTargets({
      activeSelection: selection,
      groupedSelections: groupedSelections.map((item) => item.selection),
      groupedImageMode,
    }).map((target) => [target.key, target.selection] as const),
  );

  return (
    <div id="shein-style-review" className="scroll-mt-6">
      <SheinStudioProgressStrip
        createdTaskCount={createdTaskCount}
        generatedStyleCount={designs.length}
        selectedStyleCount={selectedIds.length}
      />
      <SheinDesignPreviewGrid
        createActionDisabledReason={createActionDisabledReason}
        designs={designs}
        imageStrategy={imageStrategy}
        isCreatingTasks={isCreatingTasks}
        onBackToGenerate={onBackToGenerate}
        onCreateReviewTasks={onCreateReviewTasks}
        onNoteChange={onNoteChange}
        onRegenerate={onRegenerate}
        onToggle={onToggle}
        productImageCount={productImageCount}
        regeneratingId={regeneratingId}
        renderSizeImagesWithSds={renderSizeImagesWithSds}
        selectedIds={selectedIds}
        selection={selection}
        selectionByTargetGroupKey={selectionByTargetGroupKey}
      />
    </div>
  );
}
