export const SHEIN_PRODUCTS_TABS = ["products", "costs"] as const;

export type SheinProductsTab = (typeof SHEIN_PRODUCTS_TABS)[number];

export function parseSheinProductsTab(value: string | undefined): SheinProductsTab {
  if (value === "products" || value === "costs") {
    return value;
  }
  return "products";
}

export function isSheinProductsTab(value: string | undefined): value is SheinProductsTab {
  return value === "products" || value === "costs";
}

export function sheinProductsTabLabel(tab: SheinProductsTab) {
  switch (tab) {
    case "products":
      return "同步商品";
    case "costs":
      return "成本价维护";
  }
}
