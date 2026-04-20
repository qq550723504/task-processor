import type { ScenePresetSummary } from "@/lib/types/listingkit";

const DEFAULTS_SOURCE_LABELS: Record<string, string> = {
  explicit: "User override",
  platform_category: "Platform + category default",
  platform: "Platform default",
  fallback: "Fallback default",
};

function normalizeLabel(value?: string): string | undefined {
  if (!value) {
    return undefined;
  }
  return value
    .split(/[_-]+/u)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function presentSceneDefaultsSource(value?: string): string | undefined {
  if (!value) {
    return undefined;
  }
  return DEFAULTS_SOURCE_LABELS[value] ?? normalizeLabel(value);
}

export function presentSceneValue(value?: string): string | undefined {
  return normalizeLabel(value);
}

export function presentScenePresetCompact(summary?: ScenePresetSummary | null): {
  title: string;
  detail?: string;
} | null {
  if (!summary) {
    return null;
  }

  const title =
    presentSceneValue(summary.scene_category) ??
    presentSceneValue(summary.scene_style) ??
    summary.prompt_key ??
    "";
  if (!title) {
    return null;
  }

  const defaultsSource = presentSceneDefaultsSource(summary.defaults_source);
  const sceneStyle = presentSceneValue(summary.scene_style);
  const detail = [defaultsSource, sceneStyle]
    .filter((value) => value && value !== title)
    .join(" · ");

  return {
    title,
    detail: detail || undefined,
  };
}

