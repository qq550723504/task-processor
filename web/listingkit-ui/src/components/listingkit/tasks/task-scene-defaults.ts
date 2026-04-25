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

const platformCategorySceneDefaults = {
  amazon: {
    shoes: {
      sceneStyle: "studio",
      backgroundTone: "bright",
      composition: "centered",
      propsLevel: "none",
      audienceHint: "premium",
    },
    jewelry: {
      sceneStyle: "studio",
      backgroundTone: "cool",
      composition: "close_up",
      propsLevel: "none",
      audienceHint: "premium",
    },
    bags: {
      sceneStyle: "studio",
      backgroundTone: "neutral",
      composition: "centered",
      propsLevel: "none",
      audienceHint: "premium",
    },
  },
  shein: {
    shoes: {
      sceneStyle: "lifestyle",
      backgroundTone: "warm",
      composition: "close_up",
      propsLevel: "light",
      audienceHint: "youthful",
    },
    jewelry: {
      sceneStyle: "lifestyle",
      backgroundTone: "warm",
      composition: "close_up",
      propsLevel: "light",
      audienceHint: "youthful",
    },
    bags: {
      sceneStyle: "lifestyle",
      backgroundTone: "warm",
      composition: "multi_angle",
      propsLevel: "light",
      audienceHint: "youthful",
    },
  },
  temu: {
    shoes: {
      sceneStyle: "lifestyle",
      backgroundTone: "bright",
      composition: "multi_angle",
      propsLevel: "moderate",
      audienceHint: "sporty",
    },
    jewelry: {
      sceneStyle: "lifestyle",
      backgroundTone: "bright",
      composition: "close_up",
      propsLevel: "light",
      audienceHint: "youthful",
    },
    bags: {
      sceneStyle: "lifestyle",
      backgroundTone: "bright",
      composition: "multi_angle",
      propsLevel: "moderate",
      audienceHint: "sporty",
    },
  },
  walmart: {
    shoes: {
      sceneStyle: "lifestyle",
      backgroundTone: "neutral",
      composition: "centered",
      propsLevel: "light",
      audienceHint: "homey",
    },
    jewelry: {
      sceneStyle: "studio",
      backgroundTone: "neutral",
      composition: "close_up",
      propsLevel: "none",
      audienceHint: "premium",
    },
    bags: {
      sceneStyle: "lifestyle",
      backgroundTone: "neutral",
      composition: "centered",
      propsLevel: "light",
      audienceHint: "homey",
    },
  },
} as const;

export function getPlatformSceneDefaults(
  platform?: string,
  sceneCategory?: string,
): TaskSceneDraftValues | null {
  if (!platform) {
    return null;
  }
  const normalizedPlatform =
    platform.trim().toLowerCase() as keyof typeof platformSceneDefaults;
  const normalizedCategory =
    sceneCategory?.trim().toLowerCase() as
      | keyof (typeof platformCategorySceneDefaults)[keyof typeof platformCategorySceneDefaults]
      | undefined;

  if (normalizedCategory) {
    const platformCategoryDefaults = platformCategorySceneDefaults[normalizedPlatform];
    if (platformCategoryDefaults?.[normalizedCategory]) {
      return { ...platformCategoryDefaults[normalizedCategory] };
    }
  }
  if (platformSceneDefaults[normalizedPlatform]) {
    return { ...platformSceneDefaults[normalizedPlatform] };
  }
  return null;
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

export function matchesSceneDefaults(
  values: TaskSceneDraftValues,
  defaults: TaskSceneDraftValues | null,
) {
  if (!defaults) {
    return false;
  }
  return (
    (values.sceneStyle ?? "") === (defaults.sceneStyle ?? "") &&
    (values.backgroundTone ?? "") === (defaults.backgroundTone ?? "") &&
    (values.composition ?? "") === (defaults.composition ?? "") &&
    (values.propsLevel ?? "") === (defaults.propsLevel ?? "") &&
    (values.audienceHint ?? "") === (defaults.audienceHint ?? "")
  );
}

