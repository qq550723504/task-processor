import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
} from "@/lib/types/shein-studio";

export function SheinStudioWorkbenchAlerts({
  draftWarning,
  generationWarning,
  galleryRatioCheck,
}: {
  draftWarning: string;
  generationWarning: string;
  galleryRatioCheck: SDSRatioMatch | null;
}) {
  return (
    <>
      {generationWarning ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          {generationWarning}
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
      />
    </div>
  );
}
