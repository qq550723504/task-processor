import type { ProductWorkspaceNavItem } from "@/components/listingkit/workspace/product-workspace-model";

const STATUS_MARKERS: Record<NonNullable<ProductWorkspaceNavItem["status"]>, string> = {
  ready: "✓",
  processing: "●",
  attention: "⚠",
  failed: "✕",
  idle: "○",
};

export function ProductWorkspaceNavigation({
  canonicalItems,
  platformItems,
  onSelect,
  onRecoverPlatform,
  onSelectHistory,
  historySelected = false,
}: {
  canonicalItems: readonly ProductWorkspaceNavItem[];
  platformItems: readonly ProductWorkspaceNavItem[];
  onSelect: (item: ProductWorkspaceNavItem) => void;
  onRecoverPlatform?: (item: ProductWorkspaceNavItem) => void;
  onSelectHistory: () => void;
  historySelected?: boolean;
}) {
  return (
    <div className="space-y-5 rounded-xl border border-border bg-card p-3">
      <NavigationGroup label="商品资料">
        {canonicalItems.map((item) => (
          <NavigationButton item={item} key={item.key} onSelect={onSelect} />
        ))}
      </NavigationGroup>

      <NavigationGroup label="平台资料">
        {platformItems.length > 0 ? (
          platformItems.map((item) => (
            <div className="flex items-center gap-1" key={item.key}>
              <div className="min-w-0 flex-1">
                <NavigationButton item={item} onSelect={onSelect} showStatus />
              </div>
              {item.recoveryLabel ? (
                <button
                  className="shrink-0 rounded-md border border-border px-2 py-1.5 text-xs font-medium text-muted-foreground transition hover:bg-accent hover:text-foreground"
                  onClick={() => onRecoverPlatform?.(item)}
                  type="button"
                >
                  {item.recoveryLabel}
                </button>
              ) : null}
            </div>
          ))
        ) : (
          <p className="px-2 text-xs leading-5 text-muted-foreground">尚未生成平台资料</p>
        )}
      </NavigationGroup>

      <div className="border-t border-border pt-3">
        <button
          aria-current={historySelected ? "page" : undefined}
          className={navigationButtonClass(historySelected)}
          onClick={onSelectHistory}
          type="button"
        >
          历史
        </button>
      </div>
    </div>
  );
}

function NavigationGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <p className="px-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
        {label}
      </p>
      <div className="space-y-1">{children}</div>
    </div>
  );
}

function NavigationButton({
  item,
  onSelect,
  showStatus = false,
}: {
  item: ProductWorkspaceNavItem;
  onSelect: (item: ProductWorkspaceNavItem) => void;
  showStatus?: boolean;
}) {
  return (
    <button
      aria-current={item.selected ? "page" : undefined}
      aria-label={item.label}
      className={navigationButtonClass(Boolean(item.selected))}
      onClick={() => onSelect(item)}
      type="button"
    >
      <span className="truncate">{item.label}</span>
      {showStatus ? (
        <span aria-hidden="true" className="ml-auto text-xs text-muted-foreground">
          {STATUS_MARKERS[item.status ?? "idle"]}
        </span>
      ) : null}
    </button>
  );
}

function navigationButtonClass(selected: boolean) {
  return [
    "flex w-full items-center rounded-lg px-2 py-2 text-left text-sm transition",
    selected
      ? "bg-accent font-medium text-accent-foreground"
      : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
  ].join(" ");
}
