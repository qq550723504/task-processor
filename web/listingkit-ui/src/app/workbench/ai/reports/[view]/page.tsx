import { notFound } from "next/navigation";
import { ReportPage } from "@/components/workbench/reports/report-page";
import { ConsoleState } from "@/components/workbench/console/console-page";
import { isReportCenterAvailable } from "@/lib/server/report-center-availability";
export default async function Page({ params }: { params: Promise<{ view: string }> }) {
  const {view}=await params;
  if(view!=="recent"&&view!=="favorites"&&view!=="all") notFound();
  return isReportCenterAvailable() ? <ReportPage view={view} /> : <ConsoleState kind="unavailable" title="我的报告尚未启用">当前安装尚未启用报告服务。</ConsoleState>;
}
