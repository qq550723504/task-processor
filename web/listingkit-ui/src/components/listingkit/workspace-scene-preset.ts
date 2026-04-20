import type { QueueItem, ScenePresetSummary } from "@/lib/types/listingkit";

function hasScenePreset(summary?: ScenePresetSummary | null): summary is ScenePresetSummary {
  if (!summary) {
    return false;
  }

  return Boolean(
    summary.prompt_key ||
      summary.defaults_source ||
      summary.scene_category ||
      summary.scene_style ||
      summary.background_tone ||
      summary.composition ||
      summary.props_level ||
      summary.audience_hint ||
      summary.custom_scene_hint,
  );
}

export function resolveWorkspaceScenePreset(params: {
  reviewPreviewPreset?: ScenePresetSummary | null;
  focusedScenePreset?: ScenePresetSummary | null;
  queueItems?: QueueItem[] | null;
  selectedSlot?: string | null;
  focusedAssetId?: string | null;
}): ScenePresetSummary | undefined {
  if (hasScenePreset(params.reviewPreviewPreset)) {
    return params.reviewPreviewPreset;
  }

  if (hasScenePreset(params.focusedScenePreset)) {
    return params.focusedScenePreset;
  }

  const queueItems = params.queueItems ?? [];
  const selectedSlot = params.selectedSlot?.trim();
  const focusedAssetId = params.focusedAssetId?.trim();

  const candidates = queueItems.filter((item) => {
    if (!hasScenePreset(item.scene_preset)) {
      return false;
    }
    if (!selectedSlot) {
      return true;
    }
    return item.slot === selectedSlot;
  });

  if (candidates.length === 0) {
    return undefined;
  }

  if (focusedAssetId) {
    const focusedMatch = candidates.find(
      (item) => item.selected_asset_id?.trim() === focusedAssetId,
    );
    if (focusedMatch?.scene_preset) {
      return focusedMatch.scene_preset;
    }
  }

  return candidates[0]?.scene_preset;
}

