import { ProjectPage } from "@/components/workbench/project-center/project-page";
export default async function Page({params}:{params:Promise<{projectId:string}>}){const {projectId}=await params;return <ProjectPage projectId={projectId}/>;}
