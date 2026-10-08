import { z } from "zod";
import { collectionID } from "../contracts/product-collection";
import { SUPPLY_MAX_BYTES } from "../contracts/supply-chain";
import { supplyIntentRequestSchema } from "./supply-chain";
import type { SupplyIntent } from "./supply-chain";

const storageKey="listingkit.supply.intent";
const identity=z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const envelope=z.object({userId:identity,organizationId:identity,key:collectionID,route:z.enum(["transfer","save-target","approve","create-operation","review-decision","review-apply","resolve-upload"]),command:z.unknown()}).strict();
export function parseSupplyIntent(raw:string|null):SupplyIntent|null{
 if(!raw || new TextEncoder().encode(raw).length>SUPPLY_MAX_BYTES)return null;
 try{
  const value=envelope.safeParse(JSON.parse(raw));if(!value.success)return null;
  const schema=supplyIntentRequestSchema(value.data.route);const command=schema?.safeParse(value.data.command);if(!command?.success)return null;
  if(value.data.route==="approve" && "actionId" in command.data && command.data.actionId && command.data.actionId!==value.data.key)return null;
  return {...value.data,command:command.data};
 }catch{return null}
}
export function loadSupplyIntent():SupplyIntent|null{
 try{return parseSupplyIntent(sessionStorage.getItem(storageKey))}catch{return null}
}
export function saveSupplyIntent(intent:SupplyIntent|null):boolean{
 try{
  if(intent===null){sessionStorage.removeItem(storageKey);return true}
  const raw=JSON.stringify(intent);if(!parseSupplyIntent(raw))return false;sessionStorage.setItem(storageKey,raw);return true;
 }catch{return false}
}
