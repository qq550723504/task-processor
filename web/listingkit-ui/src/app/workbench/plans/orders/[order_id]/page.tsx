import { CommercialPage } from "@/components/workbench/commercial/commercial-page";

export default async function CommercialOrderDetailPage({ params }: { params: Promise<{ order_id: string }> }) {
  const { order_id } = await params;
  return <CommercialPage page="order-detail" orderId={order_id} />;
}
