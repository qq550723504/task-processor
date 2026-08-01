import { collectSheinPreviewImageGroups } from "@/components/listingkit/shein/shein-preview-image";
import type { SheinFlowStep } from "@/components/listingkit/shein/shein-flow-nav";
import {
  projectSheinReadinessActions,
  type SheinReadinessProjection,
} from "@/components/listingkit/shein/shein-workspace-actions";
import {
  hasSheinAttributeReviewSignal,
  hasSheinCategoryReviewSignal,
  hasSheinSaleAttributeReviewSignal,
} from "@/components/listingkit/workspace/workspace-screen-helpers";
import type {
  SDSSyncSummary,
  SheinPreviewPayload,
} from "@/lib/types/listingkit";

import type {
  WorkspacePlatformAdapter,
  WorkspacePlatformProjection,
} from "@/components/listingkit/workspace/workspace-platform-adapter";

export type SheinWorkspaceProjection = {
  images: ReturnType<typeof collectSheinPreviewImageGroups>["productImages"];
  mockupImages: ReturnType<typeof collectSheinPreviewImageGroups>["mockupImages"];
  variantCount?: number;
  readiness: SheinReadinessProjection;
  showCategoryReview: boolean;
  showAttributeReview: boolean;
  showSaleAttributeReview: boolean;
  showReviewDetails: boolean;
  shouldOpenAdvancedDetails: boolean;
  flowSteps: SheinFlowStep[];
};

export function createSheinWorkspaceAdapter({
  taskId,
  isFinalReviewMode,
  shein,
  sdsDesignResult,
}: {
  taskId: string;
  isFinalReviewMode: boolean;
  shein?: SheinPreviewPayload | null;
  sdsDesignResult?: SDSSyncSummary;
}): WorkspacePlatformAdapter<WorkspacePlatformProjection<SheinWorkspaceProjection>> {
  return {
    platform: "shein",
    project: () => {
      const imageGroups = collectSheinPreviewImageGroups(shein, sdsDesignResult);
      const readiness = projectSheinReadinessActions(
        shein?.submit_readiness?.blocking_items,
      );
      const showCategoryReview = hasSheinCategoryReviewSignal(shein?.editor_context);
      const showAttributeReview = hasSheinAttributeReviewSignal(shein?.editor_context);
      const showSaleAttributeReview = hasSheinSaleAttributeReviewSignal(
        shein?.editor_context,
      );
      const showReviewDetails =
        showCategoryReview || showAttributeReview || showSaleAttributeReview;
      const preFinalReviewBlocked =
        readiness.cookieBlocked ||
        readiness.categoryBlocked ||
        readiness.attributeBlocked ||
        readiness.saleAttributeBlocked ||
        readiness.previewBlocked;
      const projection: SheinWorkspaceProjection = {
        images: imageGroups.productImages,
        mockupImages: imageGroups.mockupImages,
        variantCount: shein?.final_review?.skus?.length,
        readiness,
        showCategoryReview,
        showAttributeReview,
        showSaleAttributeReview,
        showReviewDetails,
        shouldOpenAdvancedDetails:
          !isFinalReviewMode &&
          (readiness.categoryBlocked ||
            readiness.attributeBlocked ||
            readiness.saleAttributeBlocked),
        flowSteps: [
          {
            key: "preview",
            label: "检查图片",
            description: imageGroups.productImages.length
              ? `已准备 ${imageGroups.productImages.length} 张 SHEIN 成品图，SDS mockup 会单独作为渲染参考展示。`
              : "检查 SHEIN 成品图是否已经生成；SDS mockup 仅作为渲染参考。",
            href: "#shein-preview-images",
            state:
              readiness.previewBlocked || !imageGroups.productImages.length
                ? "blocked"
                : "done",
            actionLabel: "查看图片",
          },
          {
            key: "category",
            label: "确认类目",
            description: readiness.categoryBlocked
              ? "确认 SHEIN 类目和 category path，不使用静态兜底。"
              : "SHEIN 类目已确认，可查看当前类目摘要。",
            href: "#shein-category-review-card",
            state: readiness.categoryBlocked ? "blocked" : "done",
            actionLabel: readiness.categoryBlocked ? "确认类目" : "查看类目",
          },
          {
            key: "attributes",
            label: "确认普通属性",
            description: "补齐普通属性候选值，人工确认后才缓存。",
            href: "#shein-attribute-review-card",
            state: readiness.attributeBlocked ? "blocked" : "done",
            actionLabel: "确认属性",
          },
          {
            key: "sale-attributes",
            label: "确认销售属性",
            description: "检查颜色、尺寸等销售属性映射。",
            href: "#shein-sale-attribute-review-card",
            state: readiness.saleAttributeBlocked ? "blocked" : "done",
            actionLabel: "确认销售属性",
          },
          {
            key: "submit",
            label: "提交",
            description: "先上传 SHEIN 图片，再保存草稿或发布。",
            href: `/listing-kits/${taskId}/workspace?platform=shein&section_key=final_review`,
            state:
              isFinalReviewMode ||
              shein?.submit_readiness?.status === "ready" ||
              !preFinalReviewBlocked
                ? "active"
                : "blocked",
            actionLabel: "打开最终确认",
          },
        ],
      };

      return {
        kind: "shein",
        platform: "shein",
        projection,
      };
    },
  };
}
