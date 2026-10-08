"use client";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
	getStoreConnection,
	getStoreApplications,
  beginStoreConnection,
  queryStoreConnection,
  disconnectStoreConnection,
  rememberStoreAuthorization,
  StoreConnectionError,
} from "@/lib/api/store-connection";
import type { MemberResourceScope } from "@/lib/api/member-resources";
import type { WorkbenchStore } from "@/lib/api/workbench-stores";
import { StoreServiceActions } from "./store-service-actions";
const labels = {
  connected: "已连接",
  disconnected: "未连接",
  expired: "授权已失效",
  unavailable: "连接服务未配置或暂不可用",
};
const applicationLabels={self_operated:"自运营",semi_managed:"半托管",fully_managed:"全托管"} as const;
export function StoreOfficialConnection({
  store,
  scope,
  canWrite,
  administrator,
  onChanged,
}: {
  store: WorkbenchStore;
  scope: MemberResourceScope;
  canWrite: boolean;
  administrator: boolean;
  onChanged: () => Promise<unknown>;
}) {
  const view = useQuery({
    queryKey: [
      "workbench",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      "official-connection",
      store.id,
      store.version,
    ],
    queryFn: ({ signal }) => getStoreConnection(scope, store.id, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
	const [message, setMessage] = useState("");
	const applications=useQuery({
		queryKey:["workbench",scope.expectedUserId,scope.expectedOrganizationId,"official-applications",store.id],
		queryFn:({signal})=>getStoreApplications(scope,store.id,signal),gcTime:0,staleTime:0,retry:false,
	});
	const [selectedApp,setSelectedApp]=useState("");
	const choices=!applications.isFetching && !applications.isError ? applications.data??[]:[];
	const chosen=choices.find(choice=>choice.appId===selectedApp)?.appId ?? (choices.length===1 ? choices[0]!.appId:"");
  const [working, setWorking] = useState(false);
  const busy = useRef(false);
  const active = useRef<{ alive: boolean } | null>(null);
  useEffect(() => {
    const current = { alive: true };
    active.current = current;
    return () => {
      current.alive = false;
    };
  }, [
    scope.expectedUserId,
    scope.expectedOrganizationId,
    store.id,
    canWrite,
    administrator,
  ]);
  const data = !view.isFetching && !view.isError ? view.data : undefined;
  async function run(action: "begin" | "query" | "disconnect") {
    const current = active.current;
    if (busy.current || !current?.alive) return;
    busy.current = true;
    setWorking(true);
    setMessage("");
    try {
			if (action === "begin") {
				if(!chosen)return;
        const key = crypto.randomUUID();
        const result = await beginStoreConnection(
          scope,
          store.id,
          store.version,
					key,
					chosen,
        );
        if (!current.alive) return;
        rememberStoreAuthorization({
          ...scope,
          storeId: store.id,
          attemptId: result.attemptId,
          expiresAt: result.expiresAt,
        });
        window.location.assign(result.authorizationUrl);
        return;
      }
      if (action === "query" && data?.attemptId) {
        await queryStoreConnection(scope, store.id, data.attemptId);
      } else if (action === "disconnect") {
        await disconnectStoreConnection(
          scope,
          store.id,
          store.version,
          crypto.randomUUID(),
        );
        if (!current.alive) return;
        setMessage("本地连接已断开。SHEIN 端授权请在‘我的授权’中自行撤销。");
      }
      if (!current.alive) return;
      await Promise.all([view.refetch(), onChanged()]);
    } catch (error) {
      if (!current.alive) return;
      setMessage(
        error instanceof StoreConnectionError &&
          error.code === "STORE_OFFICIAL_SETUP_UNAVAILABLE"
          ? "尚未配置获批的 SHEIN 开发者应用，暂不能真实连接。"
          : error instanceof StoreConnectionError &&
              error.code === "STORE_AUTHORIZATION_OUTCOME_UNKNOWN"
            ? "授权交换结果未知，不能再次交换原临时凭据。请刷新后重新发起官方授权。"
            : error instanceof StoreConnectionError && error.unknown
              ? "连接操作结果未确认，请先刷新连接状态。不会自动重复发起授权。"
              : "连接操作未完成，请刷新状态并确认当前权限。",
      );
      await view.refetch();
    } finally {
      busy.current = false;
      if (current.alive) setWorking(false);
    }
  }
  return (
    <section className="mt-4 space-y-3" aria-label="SHEIN 官方连接">
      <h2 className="font-semibold">SHEIN 官方连接</h2>
      <p role={view.isError ? "alert" : "status"}>
        {view.isPending || view.isFetching
          ? "正在读取官方连接…"
          : data
            ? labels[data.connectionStatus]
            : "本次未取得官方连接状态。"}
      </p>
      {data?.observedAt ? (
        <p className="text-sm text-muted-foreground">
          最近官方检查：{data.observedAt}
        </p>
      ) : null}
      <p className="text-sm text-muted-foreground">
        前往 SHEIN
        官方授权页，由店铺主账号确认。连接只证明当前授权有效，发布权限另行判断。
      </p>
			{canWrite &&
			store.recordStatus !== "deleting" ? (
				<><label className="grid gap-1 text-sm">官方应用类型
					<select aria-label="官方应用类型" value={chosen} disabled={working || applications.isFetching} onChange={event=>setSelectedApp(event.target.value)} className="rounded-lg border bg-background p-2">
						<option value="">选择已配置的官方应用</option>
						{choices.map(choice=><option key={choice.appId} value={choice.appId}>{applicationLabels[choice.type]} · {choice.appId}</option>)}
					</select>
				</label>
				<p className="text-sm text-muted-foreground">{applications.isFetching ? "正在读取应用配置…": applications.isError ? "未取得应用配置，请刷新后再连接。": Object.entries(applicationLabels).filter(([type])=>!choices.some(choice=>choice.type===type)).map(([,label])=>`${label}未配置`).join("；")}</p>
				<div className="flex flex-wrap gap-2">
					<Button disabled={working || !data || !chosen} onClick={() => void run("begin")}>
            {data?.attemptId ? "重新官方授权" : "前往官方授权"}
          </Button>
          {data?.state === "credential_received" ||
          data?.state === "verified" ? (
            <Button
              variant="outline"
              disabled={working}
              onClick={() => void run("query")}
            >
              核验原连接
            </Button>
          ) : null}
          {data?.attemptId && data.state !== "failed" ? (
            <Button
              variant="outline"
              disabled={working}
              onClick={() => void run("disconnect")}
            >
              断开本地连接
            </Button>
          ) : null}
          <Button
            variant="outline"
            disabled={working}
            onClick={() => void view.refetch()}
          >
            刷新连接状态
          </Button>
				</div></>
      ) : null}
      {message ? <p role="alert">{message}</p> : null}
      <StoreServiceActions
        store={store}
        scope={scope}
        canWrite={canWrite}
        administrator={administrator}
        connected={data?.connectionStatus === "connected"}
        onChanged={onChanged}
      />
    </section>
  );
}
