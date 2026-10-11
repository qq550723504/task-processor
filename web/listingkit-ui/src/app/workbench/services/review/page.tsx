import {EcoservicesReview} from "@/components/workbench/ecoservices/review";
import {ConsoleState} from "@/components/workbench/console/console-page";
import {isEcoservicesAvailable,isEcoservicesNonPaymentOnly} from "@/lib/server/ecoservices-availability";
export default function Page(){return isEcoservicesAvailable()?<EcoservicesReview nonPaymentOnly={isEcoservicesNonPaymentOnly()}/>:<ConsoleState kind="unavailable" title="生态服务尚未开放"/>;}
