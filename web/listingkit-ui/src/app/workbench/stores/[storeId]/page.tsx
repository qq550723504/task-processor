import { StoreDetailPage } from "@/components/workbench/stores/store-detail-page";

export default async function WorkbenchStorePage({ params }: { params: Promise<{ storeId: string }> }) {
  const { storeId } = await params;
  return <StoreDetailPage storeId={storeId} />;
}
