import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
} from "@/lib/types/shein-studio";

export function resolveGenerationStartValidation({
  activeSelection,
  prompt,
  sheinStoreId,
}: {
  activeSelection?: SDSProductVariantSelection;
  prompt: string;
  sheinStoreId: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!sheinStoreId.trim()) {
    return { error: "请先选择批次店铺。" };
  }
  if (!prompt.trim()) {
    return { error: "请先填写主题提示词。", focusPrompt: true };
  }
  return null;
}

export function resolveRegenerationStartValidation({
  activeSelection,
  prompt,
}: {
  activeSelection?: SDSProductVariantSelection;
  prompt: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!prompt.trim()) {
    return { error: "请先填写主题提示词。", focusPrompt: true };
  }
  return null;
}

export function buildGenerationPromptHistoryGroups({
  activeGroupId,
  groupedImageMode,
  groups,
  prompt,
}: {
  activeGroupId: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  groups: SheinStudioGroupedWorkspace[];
  prompt: string;
}) {
  const trimmedPrompt = prompt.trim();
  if (!trimmedPrompt || !activeGroupId) {
    return groups;
  }
  const historyEntry: SDSGroupedPromptHistoryEntry = {
    prompt: trimmedPrompt,
    groupedImageMode,
    createdAt: new Date().toISOString(),
  };
  let changed = false;
  const nextGroups = groups.map((group) => {
    if (group.id !== activeGroupId) {
      return group;
    }
    const newest = group.promptHistory[0];
    const promptHistory =
      newest?.prompt === historyEntry.prompt &&
      newest?.groupedImageMode === historyEntry.groupedImageMode
        ? group.promptHistory
        : [historyEntry, ...group.promptHistory].slice(0, 5);
    if (
      promptHistory === group.promptHistory &&
      group.currentPrompt === trimmedPrompt
    ) {
      return group;
    }
    changed = true;
    return {
      ...group,
      currentPrompt: trimmedPrompt,
      promptHistory,
      updatedAt: historyEntry.createdAt,
    };
  });
  return changed ? nextGroups : groups;
}

export function withGenerationTargetMetadata(
  images: SheinStudioGeneratedDesign[],
  target: { key: string; label?: string },
) {
  return images.map((image) => ({
    ...image,
    targetGroupKey: target.key,
    targetGroupLabel: target.label,
  }));
}

export function mergeGeneratedDesignCollections(
  currentDesigns: SheinStudioGeneratedDesign[],
  incomingDesigns: SheinStudioGeneratedDesign[],
) {
  const nextByID = new Map(currentDesigns.map((design) => [design.id, design]));
  for (const design of incomingDesigns) {
    nextByID.set(design.id, design);
  }
  return Array.from(nextByID.values());
}

export function mergeGeneratedSelectedIds(
  currentIDs: string[],
  designs: SheinStudioGeneratedDesign[],
) {
  const next = new Set(currentIDs);
  for (const design of designs) {
    next.add(design.id);
  }
  return Array.from(next);
}

export function replaceRegeneratedDesign({
  designId,
  designs,
  replacement,
}: {
  designId: string;
  designs: SheinStudioGeneratedDesign[];
  replacement: SheinStudioGeneratedDesign;
}) {
  return designs.map((design) =>
    design.id === designId
      ? {
          ...replacement,
          id: designId,
          targetGroupKey: design.targetGroupKey,
          targetGroupLabel: design.targetGroupLabel,
        }
      : design,
  );
}
