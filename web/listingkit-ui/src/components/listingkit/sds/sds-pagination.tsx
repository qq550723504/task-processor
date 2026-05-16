"use client";

import { Button } from "@/components/ui/button";

export function SDSPagination({
  page,
  pageCount,
  onPageChange,
}: {
  page: number;
  pageCount: number;
  onPageChange: (page: number) => void;
}) {
  if (pageCount <= 1) {
    return null;
  }

  return (
    <div className="flex items-center justify-between gap-3 rounded-[1.25rem] border border-zinc-200/80 bg-zinc-50/90 px-4 py-3">
      <div className="text-sm text-zinc-600">
        Page {page} / {pageCount}
      </div>
      <div className="flex items-center gap-2">
        <Button
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          variant="secondary"
          type="button"
        >
          Previous
        </Button>
        <Button
          disabled={page >= pageCount}
          onClick={() => onPageChange(page + 1)}
          variant="secondary"
          type="button"
        >
          Next
        </Button>
      </div>
    </div>
  );
}
