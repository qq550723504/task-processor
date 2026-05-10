export type DimensionPreset =
  | "all"
  | "square"
  | "portrait"
  | "landscape"
  | "large";

export type ImageDimensions = {
  label: string;
  width: number;
  height: number;
};

export const DIMENSION_FILTER_OPTIONS: Array<{
  value: DimensionPreset;
  label: string;
}> = [
  { value: "all", label: "全部尺寸" },
  { value: "square", label: "方图 1:1" },
  { value: "portrait", label: "竖图" },
  { value: "landscape", label: "横图" },
  { value: "large", label: "大图 >= 1000px" },
];

export function formatDate(value?: string) {
  if (!value) {
    return "未知";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function sourceTone(source: string) {
  if (source.includes("mockup") || source.includes("shein")) {
    return "border-emerald-200 bg-emerald-50 text-emerald-700";
  }
  if (source.includes("legacy")) {
    return "border-amber-200 bg-amber-50 text-amber-700";
  }
  return "border-zinc-200 bg-zinc-50 text-zinc-600";
}

export function formatImageDimensions(width?: number, height?: number) {
  if (!width || !height) {
    return "";
  }
  return `${width} x ${height}px`;
}

export function matchesDimensionFilter(
  dimensions: ImageDimensions | undefined,
  filter: {
    preset: DimensionPreset;
    minHeight?: number;
    minWidth?: number;
  },
) {
  if (filter.preset === "all" && !filter.minHeight && !filter.minWidth) {
    return true;
  }
  if (!dimensions) {
    return false;
  }
  if (filter.minWidth && dimensions.width < filter.minWidth) {
    return false;
  }
  if (filter.minHeight && dimensions.height < filter.minHeight) {
    return false;
  }
  if (filter.preset === "square") {
    return (
      Math.abs(dimensions.width - dimensions.height) <=
      Math.max(dimensions.width, dimensions.height) * 0.05
    );
  }
  if (filter.preset === "portrait") {
    return dimensions.height > dimensions.width;
  }
  if (filter.preset === "landscape") {
    return dimensions.width > dimensions.height;
  }
  if (filter.preset === "large") {
    return dimensions.width >= 1000 && dimensions.height >= 1000;
  }
  return true;
}
