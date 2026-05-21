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
  mutate: (payload: ApplyRevisionRequest) => void;
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

  const handleApplySuggestedSheinCategory = () => {
    const revision = buildApplySuggestedSheinCategoryRevision(sheinPreview);
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
  };

  const handleConfirmCurrentSheinCategory = () => {
    const revision = buildConfirmCurrentSheinCategoryRevision(sheinPreview);
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
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
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
  };

  const handleConfirmSheinFallbackAttributes = () => {
    const revision = buildConfirmSheinFallbackAttributesRevision(sheinPreview);
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
  };

  const handleConfirmCurrentSheinSaleAttributes = () => {
    const revision =
      buildConfirmCurrentSheinSaleAttributesRevision(sheinPreview);
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
  };

  const handleRegenerateSheinSaleAttributes = () => {
    const revision = buildConfirmCurrentSheinCategoryRevision(sheinPreview);
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
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
    if (!revision) {
      return;
    }
    applyRevision.mutate(revision);
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
    handleApplySuggestedSheinCategory,
    handleConfirmCurrentSheinCategory,
    handleApplyManualSheinCategory,
    handleConfirmSheinAttributes,
    handleConfirmSheinFallbackAttributes,
    handleConfirmCurrentSheinSaleAttributes,
    handleRegenerateSheinSaleAttributes,
    handleApplyManualSheinSaleAttributes,
    handleSaveSheinFinalDraft,
    handleSubmitShein,
    handleSaveSheinDraft: () => handleSubmitShein("save_draft"),
    handleRegenerateSheinImage,
  };
}
