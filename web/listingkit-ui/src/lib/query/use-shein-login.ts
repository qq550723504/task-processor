"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  clearSheinCookie,
  clearSheinLastFailure,
  getSheinLastFailure,
  listSheinLoginAccounts,
  loginSheinAccount,
  submitSheinVerifyCode,
} from "@/lib/api/shein-login";

const sheinLoginAccountsKey = ["listingkit", "shein-login", "accounts"] as const;

export function useSheinLoginAccounts() {
  return useQuery({
    queryKey: sheinLoginAccountsKey,
    queryFn: listSheinLoginAccounts,
    refetchInterval: 15000,
  });
}

function useInvalidateSheinLoginAccounts() {
  const client = useQueryClient();
  return async () => {
    await client.invalidateQueries({ queryKey: sheinLoginAccountsKey });
  };
}

export function useLoginSheinAccount() {
  const invalidate = useInvalidateSheinLoginAccounts();
  return useMutation({
    mutationFn: (storeID: number) => loginSheinAccount(storeID),
    onSuccess: invalidate,
  });
}

export function useSubmitSheinVerifyCode() {
  const invalidate = useInvalidateSheinLoginAccounts();
  return useMutation({
    mutationFn: ({ storeID, code }: { storeID: number; code: string }) =>
      submitSheinVerifyCode(storeID, code),
    onSuccess: invalidate,
  });
}

export function useClearSheinCookie() {
  const invalidate = useInvalidateSheinLoginAccounts();
  return useMutation({
    mutationFn: (storeID: number) => clearSheinCookie(storeID),
    onSuccess: invalidate,
  });
}

export function useClearSheinLastFailure() {
  const invalidate = useInvalidateSheinLoginAccounts();
  return useMutation({
    mutationFn: (storeID: number) => clearSheinLastFailure(storeID),
    onSuccess: invalidate,
  });
}

export function useSheinLastFailure(storeID?: number | null) {
  return useQuery({
    queryKey: ["listingkit", "shein-login", "failure", storeID],
    queryFn: () => getSheinLastFailure(storeID as number),
    enabled: Boolean(storeID),
  });
}
