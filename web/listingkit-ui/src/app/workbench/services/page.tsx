import {redirect} from "next/navigation";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {isEcoservicesAvailable} from "@/lib/server/ecoservices-availability";
import {firstConnectedConsoleEntry} from "@/lib/workbench/console-primary-entry";
export default function Page(){const destination=firstConnectedConsoleEntry("/workbench/services",{ecoservicesAvailable:isEcoservicesAvailable()});if(destination)redirect(destination);return <ConsoleState kind="unavailable" title="生态服务尚未开放"/>;}
