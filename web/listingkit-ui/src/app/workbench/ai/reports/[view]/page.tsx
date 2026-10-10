import { notFound } from "next/navigation";
import { ReportPage } from "@/components/workbench/reports/report-page";
export default async function Page({ params }: { params: Promise<{ view: string }> }) {const {view}=await params;if(view!=="recent"&&view!=="favorites"&&view!=="all")notFound();return <ReportPage view={view}/>;}
