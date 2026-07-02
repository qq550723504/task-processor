"use client";

export function ListingKitPagination({
  onPageChange,
  page,
  pageSize,
  total,
}: {
  onPageChange: (page: number) => void;
  page: number;
  pageSize: number;
  total: number;
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(Math.max(page, 1), totalPages);

  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-600 sm:flex-row sm:items-center sm:justify-between">
      <div>
        第 {currentPage} / {totalPages} 页 · 共 {total} 条
      </div>
      <div className="flex gap-2">
        <button
          className="h-9 rounded-lg border border-zinc-200 px-3 text-sm text-zinc-700 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={currentPage <= 1}
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          type="button"
        >
          上一页
        </button>
        <button
          className="h-9 rounded-lg border border-zinc-200 px-3 text-sm text-zinc-700 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={currentPage >= totalPages}
          onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
          type="button"
        >
          下一页
        </button>
      </div>
    </div>
  );
}
