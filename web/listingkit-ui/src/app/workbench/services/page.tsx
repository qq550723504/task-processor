import {redirect} from "next/navigation";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {isEcoservicesAvailable} from "@/lib/server/ecoservices-availability";
export default function Page(){if(isEcoservicesAvailable())redirect("/workbench/services/market");return <ConsoleState kind="unavailable" title="生态服务尚未开放"/>;}
