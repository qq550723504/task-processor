import { useState } from "react";

import { regenerateSheinDataImage } from "@/lib/api/shein-image-regeneration";
import type { ApplyRevisionRequest } from "@/lib/api/revision";
import type { SubmitTaskRequest } from "@/lib/api/submit";
import type { UpdateSheinFinalDraftRequest } from "@/lib/api/shein-final-draft";
import { submitErrorMessage } from "@/components/listingkit/workspace/workspace-screen-helpers";
import {
  buildApplyManualSheinCategoryRevision,
  buildApplyManualSheinSaleAttributesRevision,
  buildApplySuggestedSheinCategoryRevision,
  buildConfirmCurrentSheinCategoryRevision,
  buildRefreshCurrentSheinCategoryRevision,
  buildRegenerateSheinAttributesRevision,
  buildRegenerateSheinSaleAttributesRevision,
  buildConfirmCurrentSheinSaleAttributesRevision,
  buildConfirmSheinAttributesRevision,
  buildConfirmSheinFallbackAttributesRevision,
} from "@/components/listingkit/workspace/shein-workspace-action-builders";
import type {
  SheinManualCategoryCandidate,
  SheinPreviewPayload,
  SheinResolvedAttribute,
  SheinSaleAttributeTemplateOption,
} from "@/lib/types/listingkit";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";

type MutationLike<TVariables = unknown> = {
  isPending: boolean;
  variables?: TVariables;
};

type ApplyRevisionMutation = MutationLike & {
  mutate: (
    payload: ApplyRevisionRequest,
    options?: {
      onSuccess?: () => void;
      onError?: (error: unknown) => void;
    },
  ) => void;
  mutateAsync: (payload: ApplyRevisionRequest) => Promise<unknown>;
};

type SubmitTaskMutation = MutationLike & {
  error?: unknown;
  mutate: (
    payload: SubmitTaskRequest,
    options?: { onSettled?: () => void },
  ) => void;
};

type UpdateSheinFinalDraftMutation = MutationLike & {
  mutateAsync: (payload: UpdateSheinFinalDraftRequest) => Promise<unknown>;
  mutate: (
    payload: UpdateSheinFinalDraftRequest,
    options?: {
      onSuccess?: () => void;
      onError?: (error: unknown) => void;
    },
  ) => void;
};

export function useSheinWorkspaceActions({
  taskId,
  sheinPreview,
  preview,
  taskResult,
  applyRevision,
  submitTask,
  updateSheinFinalDraft,
}: {
  taskId: string;
  sheinPreview?: SheinPreviewPayload;
  preview: { refetch: () => Promise<unknown> };
  taskResult: { refetch: () => Promise<unknown> };
  applyRevision: ApplyRevisionMutation;
  submitTask: SubmitTaskMutation;
  updateSheinFinalDraft: UpdateSheinFinalDraftMutation;
}) {
  type SheinRevisionNoticeScope = "category" | "attribute" | "sale_attribute";
  type SheinRevisionNotice = {
    scope: SheinRevisionNoticeScope;
    tone: "default" | "success";
    message: string;
  };
  type SheinRefreshHistoryEntry = {
    scope: SheinRevisionNoticeScope;
    status: "running" | "completed";
    title: string;
    detail: string;
    affectedAreas: string[];
    occurredAt: string;
  };

  const [selectedSheinImageUrl, setSelectedSheinImageUrl] = useState<string>();
  const [regeneratingSheinImage, setRegeneratingSheinImage] = useState(false);
  const [sheinImageRegenerationError, setSheinImageRegenerationError] =
    useState<string | null>(null);
  const [sheinSubmitAction, setSheinSubmitAction] = useState<
    "publish" | "save_draft" | null
  >(null);
  const [sheinFinalDraftMessage, setSheinFinalDraftMessage] = useState<
    string | null
  >(null);
  const [sheinFinalDraftError, setSheinFinalDraftError] = useState<
    string | null
  >(null);
  const [sheinRevisionNotice, setSheinRevisionNotice] =
    useState<SheinRevisionNotice | null>(null);
  const [sheinRefreshHistory, setSheinRefreshHistory] = useState<
    SheinRefreshHistoryEntry[]
  >([]);

  const pushSheinRefreshHistory = (entry: SheinRefreshHistoryEntry) => {
    setSheinRefreshHistory((current) => [entry, ...current].slice(0, 8));
  };

  const runSheinRevisionAction = ({
    revision,
    scope,
    pendingMessage,
    successMessage,
    pendingTitle,
    successTitle,
    affectedAreas,
  }: {
    revision: ApplyRevisionRequest | null;
    scope: SheinRevisionNoticeScope;
    pendingMessage: string;
    successMessage: string;
    pendingTitle: string;
    successTitle: string;
    affectedAreas: string[];
  }) => {
    if (!revision) {
      return;
    }
    pushSheinRefreshHistory({
      scope,
      status: "running",
      title: pendingTitle,
      detail: pendingMessage,
      affectedAreas,
      occurredAt: new Date().toISOString(),
    });
    setSheinRevisionNotice({
      scope,
      tone: "default",
      message: pendingMessage,
    });
    applyRevision.mutate(revision, {
      onSuccess: () => {
        pushSheinRefreshHistory({
          scope,
          status: "completed",
          title: successTitle,
          detail: successMessage,
          affectedAreas,
          occurredAt: new Date().toISOString(),
        });
        setSheinRevisionNotice({
          scope,
          tone: "success",
          message: successMessage,
        });
      },
      onError: () => setSheinRevisionNotice(null),
    });
  };

  const handleApplySuggestedSheinCategory = () => {
    runSheinRevisionAction({
      revision: buildApplySuggestedSheinCategoryRevision(sheinPreview),
      scope: "category",
      pendingTitle: "正在应用建议类目",
      pendingMessage: "正在应用建议类目并刷新后续模板…",
      successTitle: "建议类目已应用",
      successMessage: "已应用建议类目，系统会按最新类目继续刷新属性和销售属性。",
      affectedAreas: ["类目", "普通属性", "销售属性"],
    });
  };

  const handleConfirmCurrentSheinCategory = () => {
    runSheinRevisionAction({
      revision: buildConfirmCurrentSheinCategoryRevision(sheinPreview),
      scope: "category",
      pendingTitle: "正在确认当前类目",
      pendingMessage: "正在确认当前类目并刷新后续模板…",
      successTitle: "当前类目已确认",
      successMessage: "已确认当前类目，系统会按这个类目继续刷新属性和销售属性。",
      affectedAreas: ["类目", "普通属性", "销售属性"],
    });
  };

  const handleRefreshSheinCategory = () => {
    runSheinRevisionAction({
      revision: buildRefreshCurrentSheinCategoryRevision(sheinPreview),
      scope: "category",
      pendingTitle: "正在刷新类目模板",
      pendingMessage: "正在重新拉取当前类目模板…",
      successTitle: "类目模板已触发刷新",
      successMessage: "已触发类目模板刷新，稍后会用最新类目继续刷新属性映射。",
      affectedAreas: ["类目", "普通属性", "销售属性"],
    });
  };

  const handleApplyManualSheinCategory = async (
    candidate: SheinManualCategoryCandidate,
  ) => {
    await applyRevision.mutateAsync(
      buildApplyManualSheinCategoryRevision(candidate),
    );
  };

  const handleConfirmSheinAttributes = (attributes: SheinResolvedAttribute[]) => {
    const revision = buildConfirmSheinAttributesRevision({
      attributes,
      sheinPreview,
    });
    runSheinRevisionAction({
      revision,
      scope: "attribute",
      pendingTitle: "正在保存普通属性",
      pendingMessage: "正在保存普通属性…",
      successTitle: "普通属性已保存",
      successMessage: "已保存普通属性，最新结果会同步到当前 SHEIN 资料包。",
      affectedAreas: ["普通属性"],
    });
  };

  const handleConfirmSheinFallbackAttributes = () => {
    runSheinRevisionAction({
      revision: buildConfirmSheinFallbackAttributesRevision(sheinPreview),
      scope: "attribute",
      pendingTitle: "正在按 SDS 属性确认普通属性",
      pendingMessage: "正在按当前 SDS 属性确认普通属性…",
      successTitle: "普通属性已按 SDS 属性确认",
      successMessage: "已按当前 SDS 属性确认，后续建议继续用最新模板再复核一次。",
      affectedAreas: ["普通属性"],
    });
  };

  const handleRegenerateSheinAttributes = () => {
    runSheinRevisionAction({
      revision: buildRegenerateSheinAttributesRevision(sheinPreview),
      scope: "attribute",
      pendingTitle: "正在重新生成普通属性",
      pendingMessage: "正在重新生成普通属性…",
      successTitle: "普通属性已触发刷新",
      successMessage: "已触发普通属性刷新，系统会按当前类目模板重新生成属性候选。",
      affectedAreas: ["普通属性"],
    });
  };

  const handleConfirmCurrentSheinSaleAttributes = () => {
    const revision =
      buildConfirmCurrentSheinSaleAttributesRevision(sheinPreview);
    runSheinRevisionAction({
      revision,
      scope: "sale_attribute",
      pendingTitle: "正在确认当前销售属性",
      pendingMessage: "正在确认当前销售属性…",
      successTitle: "销售属性已确认",
      successMessage: "已确认当前销售属性，最新规格映射会同步到当前 SHEIN 资料包。",
      affectedAreas: ["销售属性"],
    });
  };

  const handleRegenerateSheinSaleAttributes = () => {
    runSheinRevisionAction({
      revision: buildRegenerateSheinSaleAttributesRevision(sheinPreview),
      scope: "sale_attribute",
      pendingTitle: "正在重新生成销售属性",
      pendingMessage: "正在重新生成销售属性…",
      successTitle: "销售属性已触发刷新",
      successMessage: "已触发销售属性刷新，系统会重新拉取模板并刷新颜色、尺寸等映射。",
      affectedAreas: ["销售属性"],
    });
  };

  const handleApplyManualSheinSaleAttributes = ({
    primaryOption,
    secondaryOption,
    skcSelections,
    skuSelections,
  }: {
    primaryOption?: SheinSaleAttributeTemplateOption | null;
    secondaryOption?: SheinSaleAttributeTemplateOption | null;
    skcSelections: Record<string, { valueId?: number; textValue?: string }>;
    skuSelections: Record<string, { valueId?: number; textValue?: string }>;
  }) => {
    const revision = buildApplyManualSheinSaleAttributesRevision({
      sheinPreview,
      primaryOption,
      secondaryOption,
      skcSelections,
      skuSelections,
    });
    runSheinRevisionAction({
      revision,
      scope: "sale_attribute",
      pendingTitle: "正在保存销售属性手工修正",
      pendingMessage: "正在保存销售属性手工修正…",
      successTitle: "销售属性手工修正已保存",
      successMessage: "已保存销售属性手工修正，最新规格映射会同步到当前 SHEIN 资料包。",
      affectedAreas: ["销售属性"],
    });
  };

  const handleSaveSheinFinalDraft = (
    payload: UpdateSheinFinalDraftRequest,
    successMessage = "Final SHEIN draft saved.",
  ) => {
    setSheinFinalDraftMessage(null);
    setSheinFinalDraftError(null);
    updateSheinFinalDraft.mutate(payload, {
      onSuccess: () => setSheinFinalDraftMessage(successMessage),
      onError: (error) => setSheinFinalDraftError(submitErrorMessage(error)),
    });
  };

  const handleSubmitShein = async (
    actionType: "publish" | "save_draft" = "publish",
    finalDraftPayload?: UpdateSheinFinalDraftRequest,
  ) => {
    if (finalDraftPayload) {
      setSheinFinalDraftMessage(null);
      setSheinFinalDraftError(null);
      try {
        await updateSheinFinalDraft.mutateAsync(finalDraftPayload);
      } catch (error) {
        setSheinFinalDraftError(submitErrorMessage(error));
        return;
      }
    }
    setSheinSubmitAction(actionType);
    submitTask.mutate(
      {
        platform: "shein",
        action: actionType,
        confirmed_final: true,
      },
      {
        onSettled: () => setSheinSubmitAction(null),
      },
    );
  };

  const handleRegenerateSheinImage = async (
    image: SheinPreviewImage,
    prompt: string,
  ) => {
    setRegeneratingSheinImage(true);
    setSheinImageRegenerationError(null);
    try {
      const response = await regenerateSheinDataImage(taskId, {
        image_url: image.url,
        label: image.label,
        prompt,
      });
      setSelectedSheinImageUrl(response.image?.image_url ?? undefined);
      await Promise.all([preview.refetch(), taskResult.refetch()]);
    } catch (error) {
      setSheinImageRegenerationError(submitErrorMessage(error));
      throw error;
    } finally {
      setRegeneratingSheinImage(false);
    }
  };

  return {
    selectedSheinImageUrl,
    setSelectedSheinImageUrl,
    regeneratingSheinImage,
    sheinImageRegenerationError,
    sheinSubmitAction,
    sheinFinalDraftMessage,
    sheinFinalDraftError,
    sheinCategoryRevisionNotice:
      sheinRevisionNotice?.scope === "category" ? sheinRevisionNotice : null,
    sheinAttributeRevisionNotice:
      sheinRevisionNotice?.scope === "attribute" ? sheinRevisionNotice : null,
    sheinSaleAttributeRevisionNotice:
      sheinRevisionNotice?.scope === "sale_attribute" ? sheinRevisionNotice : null,
    sheinRefreshHistory,
    handleApplySuggestedSheinCategory,
    handleConfirmCurrentSheinCategory,
    handleRefreshSheinCategory,
    handleApplyManualSheinCategory,
    handleConfirmSheinAttributes,
    handleConfirmSheinFallbackAttributes,
    handleRegenerateSheinAttributes,
    handleConfirmCurrentSheinSaleAttributes,
    handleRegenerateSheinSaleAttributes,
    handleApplyManualSheinSaleAttributes,
    handleSaveSheinFinalDraft,
    handleSubmitShein,
    handleSaveSheinDraft: () => handleSubmitShein("save_draft"),
    handleRegenerateSheinImage,
  };
}
