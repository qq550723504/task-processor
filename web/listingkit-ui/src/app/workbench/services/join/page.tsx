import {EcoservicesJoin} from "@/components/workbench/ecoservices/join";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {isEcoservicesAvailable} from "@/lib/server/ecoservices-availability";
export default function Page(){return isEcoservicesAvailable()?<EcoservicesJoin/>:<ConsoleState kind="unavailable" title="生态服务尚未开放"/>;}
