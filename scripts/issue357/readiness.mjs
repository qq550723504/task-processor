import {until} from './io.mjs';

export function waitForProvider(origin,{requestTimeout=10000,overallTimeout=300000,fetchProvider=fetch}={}) {
 return until(async()=>{
  const response=await fetchProvider(`${origin}/debug/ready`,{signal:AbortSignal.timeout(requestTimeout)});
  return response.ok;
 },'PROVIDER_RESTART',overallTimeout);
}
