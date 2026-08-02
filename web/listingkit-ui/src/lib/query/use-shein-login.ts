"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  cancelSheinLogin,
  clearSheinCookie,
  clearSheinLastFailure,
  getSheinLastFailure,
  listSheinLoginAccounts,
  loginSheinAccount,
  submitSheinVerifyCode,
} from "@/lib/api/shein-login";

const sheinLoginAccountsKey = (tenantID?: string) =>
  ["listingkit", "shein-login", "accounts", tenantID || "current"] as const;

export function useSheinLoginAccounts(tenantID?: string) {
  return useQuery({
    queryKey: sheinLoginAccountsKey(tenantID),
    queryFn: () => listSheinLoginAccounts(tenantID),
    refetchInterval: 15000,
  });
}

function useInvalidateSheinLoginAccounts(tenantID?: string) {
  const client = useQueryClient();
  return async () => {
    await client.invalidateQueries({ queryKey: sheinLoginAccountsKey(tenantID) });
  };
}

export function useLoginSheinAccount(tenantID?: string) {
  const invalidate = useInvalidateSheinLoginAccounts(tenantID);
  return useMutation({
    mutationFn: (storeID: number) => loginSheinAccount(storeID, tenantID),
    onSuccess: invalidate,
  });
}

export function useSubmitSheinVerifyCode(tenantID?: string) {
  const invalidate = useInvalidateSheinLoginAccounts(tenantID);
  return useMutation({
    mutationFn: ({ storeID, code, attemptID }: { storeID: number; code: string; attemptID?: string }) =>
      submitSheinVerifyCode(storeID, code, tenantID, attemptID),
    onSuccess: invalidate,
  });
}

export function useClearSheinCookie(tenantID?: string) {
  const invalidate = useInvalidateSheinLoginAccounts(tenantID);
  return useMutation({
    mutationFn: (storeID: number) => clearSheinCookie(storeID, tenantID),
    onSuccess: invalidate,
  });
}

export function useCancelSheinLogin(tenantID?: string) {
  const invalidate = useInvalidateSheinLoginAccounts(tenantID);
  return useMutation({
    mutationFn: (storeID: number) => cancelSheinLogin(storeID, tenantID),
    onSuccess: invalidate,
  });
}

export function useClearSheinLastFailure(tenantID?: string) {
  const invalidate = useInvalidateSheinLoginAccounts(tenantID);
  return useMutation({
    mutationFn: (storeID: number) => clearSheinLastFailure(storeID, tenantID),
    onSuccess: invalidate,
  });
}

export function useSheinLastFailure(storeID?: number | null, tenantID?: string) {
  return useQuery({
    queryKey: ["listingkit", "shein-login", "failure", storeID, tenantID || "current"],
    queryFn: () => getSheinLastFailure(storeID as number, tenantID),
    enabled: Boolean(storeID),
  });
}
