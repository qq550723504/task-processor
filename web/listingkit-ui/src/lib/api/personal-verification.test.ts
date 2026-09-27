import { describe, expect, it } from "vitest";
import { parsePersonalVerification } from "./personal-verification";

export const personalInitial = { userId:"user",state:"NOT_STARTED",maskedPhone:"138****0001",canStart:true,canRefresh:false,phoneReady:true,quota:{totalLimit:5,totalUsed:0,totalRemaining:5,dailyLimit:3,dailyUsed:0,dailyRemaining:3,serverTime:"2026-09-27T01:00:00Z",resetAt:"2026-09-27T16:00:00Z",nextAllowedAt:"2026-09-27T01:00:00Z"} };
describe("personal verification contract",()=>{
 it("keeps daily and lifetime allowances separate",()=>{expect(parsePersonalVerification(personalInitial,"user").quota.totalRemaining).toBe(5);});
 it("rejects foreign account facts and inconsistent quota",()=>{expect(()=>parsePersonalVerification(personalInitial,"other")).toThrow();expect(()=>parsePersonalVerification({...personalInitial,quota:{...personalInitial.quota,totalUsed:5,totalRemaining:5}},"user")).toThrow();});
 it("rejects raw identity material and unsafe links",()=>{expect(()=>parsePersonalVerification({...personalInitial,idNumber:"110101199001010010"},"user")).toThrow();expect(()=>parsePersonalVerification({...personalInitial,verificationUrl:"javascript:alert(1)"},"user")).toThrow();});
});
