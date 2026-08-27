export type ProductWorkspaceSectionKey =
  | "overview"
  | "images"
  | "basic"
  | "sku"
  | "specs"
  | "attributes"
  | "description";

type ProductWorkspaceItemStatus =
  | "ready"
  | "processing"
  | "attention"
  | "failed"
  | "idle";

export type ProductWorkspaceNavItem = {
  key: string;
  label: string;
  platform?: string;
  selected?: boolean;
  status?: ProductWorkspaceItemStatus;
};

export type ProductWorkspacePlatformInput = {
  platform: string;
  label: string;
  status?: ProductWorkspaceItemStatus;
};

export type ProductWorkspaceAttentionSeverity = "blocking" | "warning" | "passed";

export type ProductWorkspaceAttentionItem = {
  severity: ProductWorkspaceAttentionSeverity;
  label: string;
  count: number;
};

const CANONICAL_NAVIGATION: ReadonlyArray<{
  key: ProductWorkspaceSectionKey;
  label: string;
}> = [
  { key: "overview", label: "概览" },
  { key: "images", label: "图片" },
  { key: "basic", label: "基础信息" },
  { key: "sku", label: "SKU" },
  { key: "specs", label: "规格" },
  { key: "attributes", label: "属性" },
  { key: "description", label: "描述" },
];

export function buildProductWorkspaceCanonicalNavigation(
  selectedSection: ProductWorkspaceSectionKey,
): ProductWorkspaceNavItem[] {
  return CANONICAL_NAVIGATION.map((item) => ({
    ...item,
    selected: item.key === selectedSection,
  }));
}

export function buildProductWorkspacePlatformNavigation(
  platforms: readonly ProductWorkspacePlatformInput[],
  selectedPlatform?: string,
): ProductWorkspaceNavItem[] {
  return platforms.map((platform) => ({
    key: `platform:${platform.platform}`,
    label: platform.label,
    platform: platform.platform,
    selected: platform.platform === selectedPlatform,
    status: platform.status ?? "idle",
  }));
}

export function buildProductWorkspaceAttentionSummary({
  blockingCount,
  warningCount,
  passedCount,
}: {
  blockingCount: number;
  warningCount: number;
  passedCount: number;
}): ProductWorkspaceAttentionItem[] {
  return [
    { severity: "blocking", label: "必须处理", count: blockingCount },
    { severity: "warning", label: "建议确认", count: warningCount },
    { severity: "passed", label: "已通过", count: passedCount },
  ];
}
