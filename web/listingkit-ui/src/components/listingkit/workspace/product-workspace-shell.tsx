import type { ReactNode } from "react";

export function ProductWorkspaceShell({
  navigation,
  work,
  aiReview,
  actions,
}: {
  navigation: ReactNode;
  work: ReactNode;
  aiReview: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="min-w-0 space-y-4">
      <div
        className="grid min-w-0 gap-4 xl:grid-cols-[220px_minmax(0,1fr)_320px] xl:items-start"
        data-product-workspace-grid
      >
        <nav
          aria-label="商品工作台导航"
          className="min-w-0 xl:sticky xl:top-4 xl:max-h-[calc(100vh-2rem)] xl:overflow-y-auto"
        >
          {navigation}
        </nav>
        <section aria-label="商品工作区" className="min-w-0">
          {work}
        </section>
        <aside
          aria-label="AI 审核"
          className="min-w-0 xl:sticky xl:top-4 xl:max-h-[calc(100vh-2rem)] xl:overflow-y-auto"
        >
          {aiReview}
        </aside>
      </div>

      {actions ? (
        <section aria-label="商品操作" className="min-w-0">
          {actions}
        </section>
      ) : null}
    </div>
  );
}
