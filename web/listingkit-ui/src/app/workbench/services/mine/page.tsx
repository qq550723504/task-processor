import {EcoservicesMine} from "@/components/workbench/ecoservices/mine";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {isEcoservicesAvailable,isEcoservicesNonPaymentOnly} from "@/lib/server/ecoservices-availability";
export default function Page(){return isEcoservicesAvailable()?<EcoservicesMine nonPaymentOnly={isEcoservicesNonPaymentOnly()}/>:<ConsoleState kind="unavailable" title="生态服务尚未开放"/>;}
