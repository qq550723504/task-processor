import { SdsNewBatchShell } from "@/components/listingkit/sds/sds-new-batch-shell";

export default async function SdsNewPage({
  searchParams,
}: {
  searchParams?: Promise<{ entry?: string }>;
} = {}) {
  const resolvedSearchParams = (await searchParams) ?? {};
  const isQuickSingleEntry = resolvedSearchParams.entry === "single";

  return <SdsNewBatchShell isQuickSingleEntry={isQuickSingleEntry} />;
}
