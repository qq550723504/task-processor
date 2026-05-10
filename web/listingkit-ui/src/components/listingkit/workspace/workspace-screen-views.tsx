import Link from "next/link";
import type { ComponentProps } from "react";

import { PreviewCanvas } from "@/components/listingkit/shared/preview-canvas";
import { RecoveryActionList } from "@/components/listingkit/review/recovery-action-list";
import { ReviewSectionTabs } from "@/components/listingkit/review/review-section-tabs";
import { ReviewToolbar } from "@/components/listingkit/review/review-toolbar";
import { ScenePresetPanel } from "@/components/listingkit/review/scene-preset-panel";
import { SlotNavigationList } from "@/components/listingkit/review/slot-navigation-list";
import { SheinDataImageGallery } from "@/components/listingkit/shein/shein-data-image-gallery";
import { SheinFinalReviewPanel } from "@/components/listingkit/shein/shein-final-review-panel";
import { SheinSourceProductPanel } from "@/components/listingkit/shein/shein-source-product-panel";
import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";
import { SheinSubmissionTimeline } from "@/components/listingkit/shein/shein-submission-timeline";
import { WorkspacePreviewSuggestionCard } from "@/components/listingkit/workspace/workspace-preview-suggestion";

type SheinImageGalleryProps = ComponentProps<typeof SheinDataImageGallery>;
type SheinFinalReviewPanelProps = ComponentProps<typeof SheinFinalReviewPanel>;
type SheinSubmitReadinessPanelProps = ComponentProps<
  typeof SheinSubmitReadinessPanel
>;
type SheinSubmissionTimelineProps = ComponentProps<
  typeof SheinSubmissionTimeline
>;

export function SheinFinalReviewWorkspaceView({
  taskId,
  imageGalleryProps,
  finalReviewProps,
  readinessProps,
  timelineProps,
}: {
  taskId: string;
  imageGalleryProps: SheinImageGalleryProps;
  finalReviewProps: SheinFinalReviewPanelProps;
  readinessProps: SheinSubmitReadinessPanelProps;
  timelineProps: SheinSubmissionTimelineProps;
}) {
  return (
    <section className="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_24rem] 2xl:grid-cols-[minmax(0,1fr)_26rem]">
      <main className="min-w-0 space-y-4">
        <div className="rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
                最终确认模式
              </p>
              <h2 className="mt-1 text-2xl font-semibold tracking-tight text-zinc-950">
                确认图片、价格和 SKU 后再提交
              </h2>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
                这里是客户提交前的主视图。只保留最终会影响 SHEIN
                提交的数据；如果有阻断项，点击“去处理”回到对应修复卡片。
              </p>
            </div>
            <Link
              className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
              href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}
            >
              打开完整审核
            </Link>
          </div>
        </div>

        <div id="shein-preview-images" className="scroll-mt-6">
          <SheinDataImageGallery {...imageGalleryProps} />
        </div>

        <div id="shein-final-review" className="scroll-mt-6">
          <SheinFinalReviewPanel {...finalReviewProps} />
        </div>
      </main>

      <aside className="min-w-0 space-y-4 md:sticky md:top-6 md:self-start">
        <div id="shein-submit-readiness" className="scroll-mt-6">
          <SheinSubmitReadinessPanel {...readinessProps} compact />
        </div>
        <SheinSubmissionTimeline {...timelineProps} />
      </aside>
    </section>
  );
}

export function WorkspaceReviewView({
  selectedPlatform,
  previewSuggestionProps,
  reviewSectionTabsProps,
  sheinSourceProductProps,
  sheinImageGalleryProps,
  sheinFinalReviewProps,
  previewCanvasProps,
  slotNavigationProps,
  reviewToolbarProps,
  sheinReadinessProps,
  sheinTimelineProps,
  scenePresetPanelProps,
  recoveryActionListProps,
}: {
  selectedPlatform?: string;
  previewSuggestionProps: ComponentProps<typeof WorkspacePreviewSuggestionCard>;
  reviewSectionTabsProps: ComponentProps<typeof ReviewSectionTabs>;
  sheinSourceProductProps: ComponentProps<typeof SheinSourceProductPanel>;
  sheinImageGalleryProps: SheinImageGalleryProps;
  sheinFinalReviewProps: SheinFinalReviewPanelProps;
  previewCanvasProps: ComponentProps<typeof PreviewCanvas>;
  slotNavigationProps: ComponentProps<typeof SlotNavigationList>;
  reviewToolbarProps: ComponentProps<typeof ReviewToolbar>;
  sheinReadinessProps: SheinSubmitReadinessPanelProps;
  sheinTimelineProps: SheinSubmissionTimelineProps;
  scenePresetPanelProps: ComponentProps<typeof ScenePresetPanel>;
  recoveryActionListProps: ComponentProps<typeof RecoveryActionList>;
}) {
  return (
    <div className="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_21rem] 2xl:grid-cols-[minmax(0,1fr)_24rem]">
      <main className="min-w-0 space-y-4">
        <WorkspacePreviewSuggestionCard {...previewSuggestionProps} />
        <ReviewSectionTabs {...reviewSectionTabsProps} />
        {selectedPlatform === "shein" ? (
          <div id="shein-source-product" className="scroll-mt-6">
            <SheinSourceProductPanel {...sheinSourceProductProps} />
          </div>
        ) : null}
        {selectedPlatform === "shein" ? (
          <div id="shein-preview-images" className="scroll-mt-6">
            <SheinDataImageGallery {...sheinImageGalleryProps} />
          </div>
        ) : null}
        {selectedPlatform === "shein" ? (
          <div id="shein-final-review" className="scroll-mt-6">
            <SheinFinalReviewPanel {...sheinFinalReviewProps} />
          </div>
        ) : null}
        <div id="shein-preview-canvas" className="scroll-mt-6">
          <PreviewCanvas {...previewCanvasProps} />
        </div>
        <SlotNavigationList {...slotNavigationProps} />
      </main>

      <aside className="min-w-0 space-y-4 md:sticky md:top-6 md:self-start">
        <ReviewToolbar {...reviewToolbarProps} />
        {selectedPlatform === "shein" ? (
          <div id="shein-submit-readiness" className="scroll-mt-6">
            <SheinSubmitReadinessPanel {...sheinReadinessProps} compact />
          </div>
        ) : null}
        {selectedPlatform === "shein" ? (
          <SheinSubmissionTimeline {...sheinTimelineProps} />
        ) : null}
        <ScenePresetPanel {...scenePresetPanelProps} />
        <RecoveryActionList {...recoveryActionListProps} />
      </aside>
    </div>
  );
}
