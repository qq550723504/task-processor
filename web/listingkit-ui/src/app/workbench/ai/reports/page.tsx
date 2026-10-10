import { ReportPage } from "@/components/workbench/reports/report-page";
import { ConsoleState } from "@/components/workbench/console/console-page";
import { isReportCenterAvailable } from "@/lib/server/report-center-availability";
export default function Page() {
  return isReportCenterAvailable() ? <ReportPage /> : <ConsoleState kind="unavailable" title="我的报告尚未启用">当前安装尚未启用报告服务。</ConsoleState>;
}
