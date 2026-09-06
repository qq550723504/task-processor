import { SheinDiagnosticPage } from "@/components/workbench/shein-diagnostic/diagnostic-page";

export default async function Page({ params }: { params: Promise<{ record_id: string }> }) {
  const { record_id } = await params;
  return <SheinDiagnosticPage recordId={record_id} />;
}
