"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import {
  getAccountFacts,
  getAccountPreferences,
  saveAccountPreferences,
  type RegionInput,
  type AccountPreferences,
} from "@/lib/api/account-facts";
import styles from "./account.module.css";

const date = (value: string) =>
  new Date(value).toLocaleString("zh-CN", {
    timeZone: "Asia/Singapore",
    hour12: false,
  });
export function IdentityDates({ subject }: { subject: string }) {
  const query = useQuery({
    queryKey: ["account-facts", subject],
    queryFn: ({ signal }) =>
      getAccountFacts({ expectedUserId: subject, signal }),
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  if (query.isPending || query.isFetching)
    return <span className={styles.identityMeta}>正在读取注册与登录时间…</span>;
  if (query.isError)
    return (
      <span className={styles.identityMeta} role="status">
        注册与登录时间暂不可用
      </span>
    );
  return (
    <span className={styles.identityMeta}>
      注册时间：
      <time dateTime={query.data.registeredAt}>
        {date(query.data.registeredAt)}
      </time>{" "}
      · 最近登录：
      {query.data.lastLogin ? (
        <time dateTime={query.data.lastLogin}>
          {date(query.data.lastLogin)}
        </time>
      ) : (
        "暂无登录记录"
      )}
    </span>
  );
}
function usePreferences(subject: string) {
  return useQuery({
    queryKey: ["account-preferences", subject],
    queryFn: ({ signal }) =>
      getAccountPreferences({ expectedUserId: subject, signal }),
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
}
export function RegionValue({
  subject,
  field,
}: {
  subject: string;
  field: keyof RegionInput;
}) {
  const query = usePreferences(subject);
  return (
    <>
      {query.isPending || query.isFetching
        ? "正在读取"
        : query.isError
          ? "暂不可用"
          : query.data[field] || "未设置"}
    </>
  );
}
export function RegionSettings({ subject }: { subject: string }) {
  const query = usePreferences(subject);
  if (query.isPending || query.isFetching)
    return <p className={styles.note}>正在读取地区资料…</p>;
  if (query.isError)
    return (
      <p className={styles.errorText} role="status">
        地区资料暂不可用，请刷新后重试。
      </p>
    );
  return (
    <RegionForm key={`${subject}:${query.data.updatedAt}`} data={query.data} />
  );
}
function RegionForm({ data }: { data: AccountPreferences }) {
  const [form, setForm] = useState<RegionInput>({
    country: data.country,
    province: data.province,
    city: data.city,
  });
  const controller = useRef<AbortController | null>(null);
  useEffect(() => {
    const c = new AbortController();
    controller.current = c;
    return () => c.abort();
  }, []);
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (input: RegionInput) =>
      saveAccountPreferences({
        expectedUserId: data.userId,
        input,
        signal: controller.current?.signal,
      }),
    onSuccess: (saved) => {
      if (!controller.current?.signal.aborted)
        queryClient.setQueryData(["account-preferences", data.userId], saved);
    },
  });
  return (
    <div className={styles.profileForm}>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          mutation.mutate(form);
        }}
      >
        {(
          [
            ["country", "国家 / 地区"],
            ["province", "省 / 州"],
            ["city", "城市"],
          ] as const
        ).map(([field, label]) => (
          <label key={field}>
            {label}
            <input
              aria-label={label}
              value={form[field]}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  [field]: event.target.value,
                }))
              }
              disabled={mutation.isPending}
            />
          </label>
        ))}
        <div className={styles.formActions}>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? "正在保存…" : "保存地区资料"}
          </Button>
          <span className={styles.note}>
            {mutation.isSuccess
              ? "地区资料已保存"
              : mutation.isError
                ? "保存结果未确认，请刷新资料核对后重试。"
                : "地区资料由账户服务保存，用于本地化服务。"}
          </span>
        </div>
      </form>
    </div>
  );
}
