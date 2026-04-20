export type TaskSceneDraftValues = {
  sceneCategory?: string;
  sceneStyle?: string;
  backgroundTone?: string;
  composition?: string;
  propsLevel?: string;
  audienceHint?: string;
  customSceneHint?: string;
};

const platformSceneDefaults = {
  amazon: {
    sceneStyle: "studio",
    backgroundTone: "bright",
    composition: "centered",
    propsLevel: "none",
    audienceHint: "premium",
  },
  shein: {
    sceneStyle: "lifestyle",
    backgroundTone: "warm",
    composition: "close_up",
    propsLevel: "light",
    audienceHint: "youthful",
  },
  temu: {
    sceneStyle: "lifestyle",
    backgroundTone: "bright",
    composition: "multi_angle",
    propsLevel: "moderate",
    audienceHint: "sporty",
  },
  walmart: {
    sceneStyle: "lifestyle",
    backgroundTone: "neutral",
    composition: "centered",
    propsLevel: "light",
    audienceHint: "homey",
  },
} as const;

export function getPlatformSceneDefaults(platform?: string): TaskSceneDraftValues | null {
  if (!platform) {
    return null;
  }
  const normalized = platform.trim().toLowerCase() as keyof typeof platformSceneDefaults;
  return platformSceneDefaults[normalized]
    ? { ...platformSceneDefaults[normalized] }
    : null;
}

export function hasAnySceneCustomization(values: TaskSceneDraftValues) {
  return Boolean(
    values.sceneCategory?.trim() ||
      values.sceneStyle?.trim() ||
      values.backgroundTone?.trim() ||
      values.composition?.trim() ||
      values.propsLevel?.trim() ||
      values.audienceHint?.trim() ||
      values.customSceneHint?.trim(),
  );
}

