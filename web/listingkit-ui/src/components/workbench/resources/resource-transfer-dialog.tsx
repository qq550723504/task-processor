"use client";
import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { getCommercialResources } from "@/lib/api/commercial-billing";
import {
  getMemberDataPrices,
  quoteMemberData,
  transferMemberResource,
  MemberResourceError,
  resourceInteger,
  type MemberResourceScope,
  type MemberResourceEntry,
  type MemberTransferInput,
  type MemberDataQuote,
} from "@/lib/api/member-resources";
import { ResourceDialog } from "./resource-dialog";
import { transferPendingSchema, useResourcePending } from "./resource-pending";
import { roles, name, position, failure } from "./member-resource-view";
import styles from "./resources.module.css";
type TransferCommand = {
  memberId: string;
  input: MemberTransferInput;
  key: string;
};
export function TransferDialog({
  member,
  action,
  scope,
  onClose,
  onChanged,
}: {
  member: MemberResourceEntry;
  action: "allocate" | "reclaim";
  scope: MemberResourceScope;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [type, setType] = useState<MemberTransferInput["resourceType"]>(
    "store_renewal_period",
  );
  const [quantity, setQuantity] = useState("");
  const [amount, setAmount] = useState("");
  const [offerId, setOfferId] = useState("");
  const [quote, setQuote] = useState<MemberDataQuote | null>(null);
  const [message, setMessage] = useState("");
  const pending = useResourcePending(
    scope,
    ["transfer", member.memberId],
    transferPendingSchema,
  );
  const [currentOperation, setOperation] = useState<{
    command: TransferCommand;
    unknown: boolean;
  } | null>(null);
  const operation =
    currentOperation ??
    (pending.command ? { command: pending.command, unknown: true } : null);
  const [quoting, setQuoting] = useState(false);
  const busy = useRef(false);
  const balances = useQuery({
    queryKey: [
      "workbench",
      "commercial-resources",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      "allocation",
    ],
    queryFn: ({ signal }) =>
      getCommercialResources(
        scope.expectedUserId,
        scope.expectedOrganizationId,
        signal,
      ),
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  const prices = useQuery({
    queryKey: [
      "workbench",
      scope.expectedUserId,
      scope.expectedOrganizationId,
      "member-data-prices",
    ],
    queryFn: ({ signal }) => getMemberDataPrices(scope, signal),
    enabled: type === "data_row" && action === "allocate",
    retry: false,
    gcTime: 0,
    staleTime: 0,
  });
  const holding = position(member, type);
  const balance =
    !balances.isPending && !balances.isFetching && !balances.isError
      ? balances.data?.resources.find((b) => b.resource_type === type)
      : undefined;
  const limit =
    action === "reclaim"
      ? holding.free
      : balance?.state === "recorded"
        ? balance.available
        : balance?.state === "not_recorded"
          ? "0"
          : null;
  const dataAmount = type === "data_row" && action === "allocate";
  const count = dataAmount ? (quote?.quantity ?? "") : quantity;
  const valid =
    pending.ready &&
    resourceInteger.safeParse(count).success &&
    count !== "0" &&
    limit !== null &&
    BigInt(count) <= BigInt(limit) &&
    (!dataAmount || !!quote);
  async function preview() {
    if (quoting || operation) return;
    setQuoting(true);
    setMessage("");
    setQuote(null);
    try {
      if (!/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(amount))
        throw new Error("invalid amount");
      const [whole, fraction = ""] = amount.split(".");
      const minor = (
        BigInt(whole) * BigInt(100) +
        BigInt(fraction.padEnd(2, "0"))
      ).toString();
      setQuote(
        await quoteMemberData(
          scope,
          offerId || prices.data?.offers[0]?.offerId || "",
          minor,
        ),
      );
    } catch (error) {
      setMessage(failure(error));
    } finally {
      setQuoting(false);
    }
  }
  async function write(command: TransferCommand) {
    if (busy.current) return;
    const recovering = pending.command !== null;
    try {
      pending.persist(command);
    } catch {
      setMessage("无法保存原操作，请先恢复浏览器存储；本次没有发送新命令。");
      return;
    }
    busy.current = true;
    setOperation({ command, unknown: false });
    setMessage("");
    try {
      const result = await transferMemberResource(
        scope,
        command.memberId,
        command.input,
        command.key,
      );
      pending.clear(command);
      setOperation(null);
      await onChanged();
      if (action === "reclaim" && result.debtRepaid !== "0") {
        setMessage(
          `已回收，偿还企业欠额 ${result.debtRepaid}，转入未分配余额 ${result.netCredit}。`,
        );
      } else onClose();
    } catch (error) {
      if (
        recovering ||
        (error instanceof MemberResourceError && error.outcome === "unknown")
      ) {
        setOperation({ command, unknown: true });
        setMessage(
          "操作结果未确认；请核验原操作，期间不能创建新的分配或回收。",
        );
      } else {
        try {
          pending.clear(command);
          setOperation(null);
        } catch {
          setOperation({ command, unknown: true });
        }
        setMessage(failure(error));
      }
    } finally {
      busy.current = false;
    }
  }
  function confirm() {
    if (!valid || operation) return;
    if (dataAmount && quote && Date.parse(quote.expiresAt) <= Date.now()) {
      setQuote(null);
      setMessage("报价已失效，请重新换算。");
      return;
    }
    void write({
      memberId: member.memberId,
      input: {
        resourceType: type,
        action,
        quantity: count,
        expectedVersion: holding.version,
        ...(dataAmount && quote ? { quoteId: quote.quoteId } : {}),
      },
      key: crypto.randomUUID(),
    });
  }
  const title = action === "allocate" ? "分配资源" : "回收资源";
  return (
    <ResourceDialog title={title} onClose={onClose} locked={operation !== null}>
      <p>
        {name(member)} ·{" "}
        {member.roles.map((role) => roles[role] ?? role).join("、")}
      </p>
      <label className={styles.dialogField}>
        资源类型
        <select
          value={type}
          disabled={operation !== null}
          onChange={(event) => {
            setType(event.target.value as MemberTransferInput["resourceType"]);
            setQuantity("");
            setQuote(null);
            setMessage("");
          }}
        >
          <option value="store_renewal_period">续费期数</option>
          <option value="data_row">数据额度</option>
        </select>
      </label>
      <section className={styles.allocationSection}>
        <h3>{type === "data_row" ? "数据额度" : "续费期数"}</h3>
        <p>
          成员可用 {holding.free} {type === "data_row" ? "条" : "期"} ·{" "}
          {action === "allocate"
            ? `管理员可用 ${limit ?? "正在读取"}`
            : `本次可回收 ${limit}`}{" "}
          · 预留 {holding.reserved}
        </p>
        {dataAmount ? (
          <>
            <label className={styles.dialogField}>
              数据价格
              <select
                value={offerId || prices.data?.offers[0]?.offerId || ""}
                disabled={operation !== null || !prices.data?.offers.length}
                onChange={(event) => {
                  setOfferId(event.target.value);
                  setQuote(null);
                }}
              >
                {prices.data?.offers.map((offer) => (
                  <option key={offer.offerId} value={offer.offerId}>
                    {offer.offerId} · {offer.unitPriceMinor} 分/条
                  </option>
                ))}
              </select>
            </label>
            {prices.isError ||
            (!prices.isPending && !prices.data?.offers.length) ? (
              <p role="alert">当前未配置可用的数据价格，不能分配数据。</p>
            ) : null}
            <label className={styles.dialogField}>
              分配金额（元）
              <input
                value={amount}
                inputMode="decimal"
                maxLength={21}
                disabled={operation !== null}
                onChange={(event) => {
                  setAmount(event.target.value);
                  setQuote(null);
                }}
              />
            </label>
            <Button
              variant="outline"
              disabled={
                operation !== null || quoting || !prices.data?.offers.length
              }
              onClick={() => void preview()}
            >
              {quoting ? "换算中…" : "换算数据条数"}
            </Button>
            {quote ? (
              <p className={styles.conversion}>
                预计换算：{quote.quantity} 条 · 余数 {quote.remainderMinor} 分 ·
                价格版本 {quote.pricingVersion}
                <small>
                  报价有效至 {quote.expiresAt}
                  ；这里只从企业已有数据条数分配，不扣钱包。
                </small>
              </p>
            ) : null}
          </>
        ) : (
          <label className={styles.dialogField}>
            {type === "data_row" ? "本次条数" : "本次期数"}
            <input
              value={quantity}
              inputMode="numeric"
              maxLength={19}
              disabled={operation !== null}
              onChange={(event) => setQuantity(event.target.value)}
            />
            {type === "store_renewal_period" ? <small>30 天 / 期</small> : null}
          </label>
        )}
      </section>
      <p className={styles.readOnly}>
        {action === "reclaim"
          ? "仅可回收未预留、未消费的资源。企业有欠额时优先偿还，净额回到未分配余额。"
          : "换算数量不能超过管理员可用的未分配余额；成员月度 AI 点数上限在下方单独设置。"}
      </p>
      {message ? <p role="alert">{message}</p> : null}
      {pending.error ? (
        <p role="alert">
          原操作记录不可读取，已暂停新操作。请恢复浏览器存储后重试。
        </p>
      ) : null}
      <div className={styles.dialogFooter}>
        <Button
          variant="outline"
          disabled={operation !== null}
          onClick={onClose}
        >
          取消
        </Button>
        {operation?.unknown ? (
          <Button onClick={() => void write(operation.command)}>
            核验原操作
          </Button>
        ) : (
          <Button
            disabled={!valid || operation !== null || quoting}
            onClick={confirm}
          >
            {operation
              ? "提交中…"
              : action === "allocate"
                ? "确认分配"
                : "确认回收"}
          </Button>
        )}
      </div>
    </ResourceDialog>
  );
}
