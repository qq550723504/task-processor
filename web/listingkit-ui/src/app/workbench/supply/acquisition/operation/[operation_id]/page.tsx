import { AcquisitionPage } from "@/components/workbench/acquisition/acquisition-page";

export default async function Page({ params }: { params: Promise<{ operation_id: string }> }) {
  const { operation_id } = await params;
  return <AcquisitionPage operationId={operation_id} />;
}
