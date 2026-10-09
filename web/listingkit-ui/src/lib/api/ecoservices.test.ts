import {describe,it,expect} from "vitest";
import {ecoCheckoutSchema,ecoFinancialSchema,ecoInputs,ecoResultSchema} from "./ecoservices";

it("accepts the immutable V2 quote and its explicit confirmation policy",()=>{
 const id="7d25708b-b8cb-4e89-ac61-b5da1b4f0b08",policy="ecoservices-v2-channel-net-10-platform-fee-manual-expiry";
 const request={id,listingId:id,listingVersion:"1",title:"服务",category:"STORE_OPENING",description:"需求",fileIds:[],state:"QUOTED",version:"2",quote:{commissionBps:1000,allocationBasis:"CHANNEL_SETTLEMENT_NET_FLOOR_V2",policyVersion:policy,amountMinor:"100",scope:"原交付",acceptanceCriteria:"可使用",deliveryDays:3,version:"1"},acceptedDeliveryVersion:"0",financialHold:false,financialState:"",financialReason:"",createdAt:"2026-10-09T00:00:00Z",updatedAt:"2026-10-09T00:00:00Z",side:"buyer"};
 expect(ecoResultSchema.safeParse({request}).success).toBe(true);
 expect(ecoInputs.confirm.safeParse({quoteVersion:"1",policyAccepted:policy}).success).toBe(true);
 expect(ecoResultSchema.safeParse({request:{...request,quote:{...request.quote,allocationBasis:"UNKNOWN"}}}).success).toBe(false);
});

it("requires exact canonical chargeback facts and never invents zero for missing data",()=>{
 const facts={grossMinor:"101",refundedMinor:"2",platformMinor:"7",providerMinor:"72",sharedMinor:"10",returnedMinor:"3",releasedMinor:"91",channelFeeMinor:"0",channelFeeObserved:false,reconciliationReason:""};
 for(const chargedBackMinor of ["0","20","101","9007199254740993","9223372036854775807"]){const result=ecoFinancialSchema.safeParse({...facts,chargedBackMinor});expect(result.success).toBe(true);if(result.success)expect(result.data).toMatchObject({chargedBackMinor,refundedMinor:"2"})}
 expect(ecoFinancialSchema.safeParse(facts).success).toBe(false);
 for(const chargedBackMinor of [-1,20,null,"-1","020","2.0","9223372036854775808"])
  expect(ecoFinancialSchema.safeParse({...facts,chargedBackMinor}).success).toBe(false);
});

it("reads actual settlement and payer totals without guessing missing voucher facts",()=>{
 const facts={grossMinor:"100",refundedMinor:"20",chargedBackMinor:"0",platformMinor:"6",providerMinor:"54",sharedMinor:"8",returnedMinor:"2",releasedMinor:"72",channelFeeMinor:"0",channelFeeObserved:false,reconciliationReason:""};
 const channelAmounts={payerMinor:"80",settlementMinor:"80",payerRefundedMinor:"20",settlementRefundedMinor:"20"};
 expect(ecoFinancialSchema.safeParse({...facts,channelAmounts}).success).toBe(true);
 expect(ecoFinancialSchema.safeParse({...facts,channelAmounts:{...channelAmounts,settlementMinor:undefined}}).success).toBe(false);
});

it("rejects malformed sibling version strings without throwing or changing the positive int64 contract",()=>{
 for(const deliveryVersion of ["2.0","n/a","","-1","01","0","9223372036854775808"])
  expect(ecoInputs.accept.safeParse({deliveryVersion}).success).toBe(false);
 expect(ecoInputs.accept.parse({deliveryVersion:"9223372036854775807"})).toEqual({deliveryVersion:"9223372036854775807"});
});

describe("service checkout QR contract",()=>{
 it("accepts both original Native URL variants and rejects other destinations",()=>{
  const orderId="7d25708b-b8cb-4e89-ac61-b5da1b4f0b08";
  for(const codeUrl of ["weixin://wxpay/bizpayurl?pr=original","weixin://wxpay/bizpayurl/up?pr=original"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(true);
  for(const codeUrl of ["https://attacker.example/qr","weixin://wxpay/bizpayurl/other?pr=x","weixin://wxpay.attacker.example/bizpayurl?pr=x","weixin://user@wxpay/bizpayurl?pr=x","weixin://wxpay/bizpayurl/up?pr=x#fragment"])
   expect(ecoCheckoutSchema.safeParse({orderId,codeUrl}).success,codeUrl).toBe(false);
 });
});
