"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import {
  officialConnectionCompleteSchema,
  type OfficialConnectionCallback,
} from "@/lib/contracts/store-connection";
import {
  readStoreAuthorization,
  clearStoreAuthorization,
  completeStoreConnection,
  queryStoreConnection,
  StoreConnectionError,
} from "@/lib/api/store-connection";

export function SheinAuthorizationCallback() {
  const context = useWorkbenchContext();
  const captured = useRef(false);
  const busy = useRef(false);
  const callback = useRef<OfficialConnectionCallback | null>(null);
  const [pending, setPending] =
    useState<ReturnType<typeof readStoreAuthorization>>(null);
  const [message, setMessage] = useState("正在检查官方授权返回…");
  const [working, setWorking] = useState(false);
  const [canQuery, setCanQuery] = useState(false);
  const [done, setDone] = useState(false);
  const [hasCallback, setHasCallback] = useState(false);
  useEffect(() => {
    if (captured.current) return;
    captured.current = true;
    const query = new URLSearchParams(window.location.search);
    // Remove one-time provider parameters before rendering any navigation.
    window.history.replaceState(null, "", window.location.pathname);
    const original = readStoreAuthorization();
    const validKeys = [...query.keys()].every(
      (key) =>
        ["appid", "tempToken", "state"].includes(key) &&
        query.getAll(key).length === 1,
    );
    const input = officialConnectionCompleteSchema.safeParse({
      attemptId: original?.attemptId,
      appId: query.get("appid"),
      state: query.get("state"),
      tempToken: query.get("tempToken"),
    });
    const valid = !(
      !original ||
      !validKeys ||
      !input.success ||
      Date.parse(original.expiresAt) <= Date.now()
    );
    if (valid && input.success) callback.current = input.data;
    // Publish the captured browser snapshot after the imperative URL scrub.
    // Only the readiness flag enters rendering; provider values stay in memory.
    queueMicrotask(() => {
      setPending(original);
      setHasCallback(valid);
      setMessage(
        valid
          ? "已收到官方返回，请确认当前账号和企业后完成连接。"
          : "本次授权返回无法确认或已过期。请回到店铺重新发起官方授权。",
      );
    });
  }, []);
  const matches =
    !!pending &&
    context.user?.id === pending.expectedUserId &&
    context.effectiveOrganization?.id === pending.expectedOrganizationId &&
    !context.selectionRequired &&
    !context.isLoading &&
    !context.isSwitching &&
    !context.error &&
    !context.blockingError;
  async function finish(queryOnly = false) {
    if (
      busy.current ||
      !matches ||
      !pending ||
      (!queryOnly && !callback.current)
    )
      return;
    busy.current = true;
    setWorking(true);
    try {
      const result = queryOnly
        ? await queryStoreConnection(
            pending,
            pending.storeId,
            pending.attemptId,
          )
        : await completeStoreConnection(
            pending,
            pending.storeId,
            callback.current!,
          );
      if (result.connectionStatus === "connected") {
        callback.current = null;
        setHasCallback(false);
        clearStoreAuthorization();
        setDone(true);
        setMessage("官方连接已完成。可以回到店铺开通服务。");
      } else {
        setCanQuery(
          result.state === "credential_received" || result.state === "verified",
        );
        setMessage("尚未确认连接有效，请核验原连接或回到店铺查看状态。");
      }
    } catch (error) {
      callback.current = null;
      setHasCallback(false);
      setCanQuery(true);
      setMessage(
        error instanceof StoreConnectionError &&
          error.code === "STORE_AUTHORIZATION_OUTCOME_UNKNOWN"
          ? "临时凭据交换结果未知，不能重复交换。请回到店铺查看状态并重新官方授权。"
          : "连接结果尚未确认。可安全核验原连接；不会再次交换临时凭据。",
      );
    } finally {
      busy.current = false;
      setWorking(false);
    }
  }
  return (
    <main className="mx-auto max-w-xl space-y-5 p-8">
      <h1 className="text-2xl font-semibold">SHEIN 官方授权返回</h1>
      <p role="status">{message}</p>
      {pending && !matches ? (
        <p role="alert">
          请使用发起授权的账号登录，并选择原企业。当前身份或企业不一致，不能绑定此店铺。
        </p>
      ) : null}
      {!done && hasCallback ? (
        <Button disabled={!matches || working} onClick={() => void finish()}>
          完成官方连接
        </Button>
      ) : null}
      {!done && canQuery ? (
        <Button
          variant="outline"
          disabled={!matches || working}
          onClick={() => void finish(true)}
        >
          核验原连接
        </Button>
      ) : null}
      {pending && matches ? (
        <Button asChild variant="outline">
          <Link href={`/workbench/stores/${pending.storeId}`} prefetch={false}>
            返回店铺
          </Link>
        </Button>
      ) : (
        <Link href="/workbench/stores" prefetch={false}>
          返回我的店铺
        </Link>
      )}
    </main>
  );
}
