type InventorySummary = {
  total: string | null;
  available: string | null;
  raw: string | null;
};

type ProductTimesInput = {
  created_at?: string | null;
  publish_time?: string | null;
  first_shelf_time?: string | null;
  last_sync_at?: string | null;
};

type ProductTimeEntry = {
  label: string;
  value: string;
};

export function formatInventorySummary(value?: string | null): InventorySummary {
  const text = value?.trim();
  if (!text) {
    return {
      total: null,
      available: null,
      raw: null,
    };
  }

  try {
    const parsed = JSON.parse(text) as {
      total_inventory?: unknown;
      saleable_inventory?: unknown;
      total?: unknown;
      available?: unknown;
    };

    if (typeof parsed === "object" && parsed !== null) {
      const total = parsed.total ?? parsed.total_inventory;
      const available = parsed.available ?? parsed.saleable_inventory;
      if (total !== undefined || available !== undefined) {
        return {
          total: total === undefined ? null : String(total),
          available: available === undefined ? null : String(available),
          raw: null,
        };
      }
    }
  } catch {
    // Fall back to raw text below when the value is not JSON.
  }

  return {
    total: null,
    available: null,
    raw: text,
  };
}

export function getCostSourceLabel(source?: string | null) {
  switch (source) {
    case "manual":
      return "人工";
    case "auto":
      return "自动";
    case "none":
    default:
      return "缺失";
  }
}

export function formatProductTimes(input: ProductTimesInput): ProductTimeEntry[] {
  return [
    { label: "创建", value: input.created_at?.trim() ?? "" },
    { label: "发布", value: input.publish_time?.trim() ?? "" },
    { label: "首次上架", value: input.first_shelf_time?.trim() ?? "" },
    { label: "最近同步", value: input.last_sync_at?.trim() ?? "" },
  ];
}
