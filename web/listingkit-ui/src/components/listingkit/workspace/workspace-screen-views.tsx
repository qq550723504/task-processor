import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";

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

function WorkspaceStageSection({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="space-y-1">
        <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
          {eyebrow}
        </p>
        <h3 className="text-xl font-semibold tracking-tight text-zinc-950">
          {title}
        </h3>
        <p className="text-sm leading-6 text-zinc-600">{description}</p>
      </div>
      <div className="mt-4 space-y-4">{children}</div>
    </section>
  );
}

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
  const hasRepairGuidance = Boolean(
    previewSuggestionProps.suggestion ||
      (reviewSectionTabsProps.sections?.length ?? 0) > 1 ||
      selectedPlatform === "shein",
  );
  const hasPreviewConfirmation = Boolean(
    selectedPlatform === "shein" ||
      previewCanvasProps.preview ||
      previewCanvasProps.response ||
      (slotNavigationProps.slots?.length ?? 0) > 1,
  );
  const hasSubmitPreparation = selectedPlatform === "shein";

  return (
    <div className="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_21rem] 2xl:grid-cols-[minmax(0,1fr)_24rem]">
      <main className="min-w-0 space-y-4">
        {hasRepairGuidance ? (
          <WorkspaceStageSection
            eyebrow="审核修复"
            title="先处理当前阻断和审核建议"
            description="优先确认类目、普通属性、销售属性等会影响 SHEIN 提交的数据，再进入预览和最终草稿确认。"
          >
            {selectedPlatform === "shein" ? (
              <div id="shein-submit-readiness" className="scroll-mt-6">
                <SheinSubmitReadinessPanel {...sheinReadinessProps} compact />
              </div>
            ) : null}
            <WorkspacePreviewSuggestionCard {...previewSuggestionProps} />
            <ReviewSectionTabs {...reviewSectionTabsProps} />
            {!previewSuggestionProps.suggestion &&
            (reviewSectionTabsProps.sections?.length ?? 0) <= 1 &&
            selectedPlatform !== "shein" ? (
              <div className="rounded-2xl border border-dashed border-zinc-200 bg-zinc-50/70 px-4 py-3 text-sm leading-6 text-zinc-600">
                当前没有额外的审核阻断，可以直接继续检查预览和提交准备。
              </div>
            ) : null}
          </WorkspaceStageSection>
        ) : null}

        {hasPreviewConfirmation ? (
          <WorkspaceStageSection
            eyebrow="预览确认"
            title="确认图片、画布和款式预览"
            description="在这里检查主图、细节图和预览画布，确保当前素材和变体展示符合预期。"
          >
            {selectedPlatform === "shein" ? (
              <div id="shein-preview-images" className="scroll-mt-6">
                <SheinDataImageGallery {...sheinImageGalleryProps} />
              </div>
            ) : null}
            <div id="shein-preview-canvas" className="scroll-mt-6">
              <PreviewCanvas {...previewCanvasProps} />
            </div>
            <SlotNavigationList {...slotNavigationProps} />
          </WorkspaceStageSection>
        ) : null}

        {hasSubmitPreparation ? (
          <WorkspaceStageSection
            eyebrow="提交准备"
            title="最后检查提交草稿和来源资料"
            description="价格、SKU、最终图片和来源商品信息都收在这里；平时默认收起，只在接近提交时集中确认。"
          >
            <details
              id="shein-general-final-review-details"
              className="rounded-[1.5rem] border border-zinc-200 bg-white p-4 shadow-sm"
            >
              <summary className="flex cursor-pointer list-none flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                    最终确认草稿
                  </p>
                  <h4 className="mt-1 text-lg font-semibold text-zinc-950">
                    提交前价格、SKU 和最终图片
                  </h4>
                  <p className="mt-1 text-sm leading-6 text-zinc-600">
                    审核页里默认收起；只有处理价格、库存或最终提交细节时再展开。
                  </p>
                </div>
                <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-[11px] font-medium text-zinc-600">
                  点击展开
                </span>
              </summary>
              <div id="shein-final-review" className="mt-4 scroll-mt-6">
                <SheinFinalReviewPanel {...sheinFinalReviewProps} />
              </div>
            </details>
            <div id="shein-source-product" className="scroll-mt-6">
              <SheinSourceProductPanel
                {...sheinSourceProductProps}
                defaultCollapsed
              />
            </div>
          </WorkspaceStageSection>
        ) : null}
      </main>

      <aside className="min-w-0 space-y-4 md:sticky md:top-6 md:self-start">
        <ReviewToolbar {...reviewToolbarProps} />
        <details className="rounded-[1.25rem] border border-zinc-200 bg-white p-4 shadow-sm">
          <summary className="cursor-pointer list-none">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  更多诊断
                </p>
                <p className="mt-1 text-sm leading-6 text-zinc-600">
                  查看提交时间线、场景预设和恢复建议。
                </p>
              </div>
              <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-[11px] font-medium text-zinc-600">
                点击展开
              </span>
            </div>
          </summary>
          <div className="mt-4 space-y-4">
            {selectedPlatform === "shein" ? (
              <SheinSubmissionTimeline {...sheinTimelineProps} />
            ) : null}
            <ScenePresetPanel {...scenePresetPanelProps} />
            <RecoveryActionList {...recoveryActionListProps} />
          </div>
        </details>
      </aside>
    </div>
  );
}
