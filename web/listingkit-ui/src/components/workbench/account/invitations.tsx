"use client";
import {
  useEffect,
  useRef,
  useState,
  useId,
  useMemo,
  useSyncExternalStore,
} from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { MemberScope, MemberRole } from "@/lib/api/members";
import {
  createInvitation,
  readAdminInvitation,
  getInvitations,
  invitationAction,
  invitationInput,
  InvitationResult,
  recipientInvitation,
} from "@/lib/api/invitations";
import styles from "./members.module.css";
import design from "./invitations.module.css";
import { PermissionScope } from "./member-facts";
import Image from "next/image";
import Link from "next/link";
const subscribePending = (notify: () => void) => {
  window.addEventListener("membership-pending", notify);
  window.addEventListener("storage", notify);
  return () => {
    window.removeEventListener("membership-pending", notify);
    window.removeEventListener("storage", notify);
  };
};
const noPending = () => null;
const pendingChanged = () =>
  window.dispatchEvent(new Event("membership-pending"));
const roles: Record<MemberRole, string> = {
  listingkit_viewer: "只读成员",
  listingkit_operator: "操作成员",
  listingkit_admin: "企业管理员",
};
const states: Record<string, string> = {
  pending: "邀请中",
  accepting: "加入结果待核实",
  accepted: "已接受",
  declined: "已拒绝",
  cancelled: "已取消",
  expired: "已过期",
};
const deliveries: Record<string, string> = {
  not_attempted: "尚未发送",
  sending: "发送结果待核实",
  mail_server_accepted: "邮件服务器已接收",
  delivery_unknown: "邮件发送结果待核实",
};
export function InvitationsPanel({
  scope,
  assignableRoles,
  onChanged,
  onAuthorityFailure,
}: {
  scope: MemberScope;
  assignableRoles: MemberRole[];
  onChanged: () => void;
  onAuthorityFailure: (error: unknown) => void;
}) {
  const query = useQuery({
    queryKey: [
      "invitations",
      scope.expectedUserId,
      scope.expectedOrganizationId,
    ],
    queryFn: async ({ signal }) => {
      try {
        return await getInvitations({ ...scope, signal });
      } catch (failure) {
        if (!signal.aborted) onAuthorityFailure(failure);
        throw failure;
      }
    },
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const [creating, setCreating] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const active = useRef(false),
    controller = useRef<AbortController | null>(null);
  const storageKey = `invitation.create:${JSON.stringify([scope.expectedUserId, scope.expectedOrganizationId])}`;
  const raw = useSyncExternalStore(
    subscribePending,
    () => {
      try {
        return sessionStorage.getItem(storageKey);
      } catch {
        return "unreadable";
      }
    },
    noPending,
  );
  const retained = useMemo(() => {
    try {
      if (!raw) return { pending: null, error: false };
      const value = JSON.parse(raw),
        input = invitationInput.parse(value.input);
      if (
        typeof value.key !== "string" ||
        !/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/.test(
          value.key,
        )
      )
        throw new Error("invalid key");
      return { pending: { key: value.key as string, ...input }, error: false };
    } catch {
      return { pending: null, error: true };
    }
  }, [raw]);
  const unconfirmed = retained.pending;
  useEffect(() => {
    const c = new AbortController();
    controller.current = c;
    return () => c.abort();
  }, [storageKey]);
  async function run(
    action: () => Promise<InvitationResult>,
    isCreate = false,
  ) {
    const c = controller.current;
    if (!c || c.signal.aborted || active.current) return;
    active.current = true;
    setBusy(true);
    setError("");
    try {
      await action();
      if (!c.signal.aborted) {
        if (isCreate) {
          sessionStorage.removeItem(storageKey);
          pendingChanged();
          setCreating(false);
        }
        await query.refetch();
        onChanged();
      }
    } catch (failure) {
      if (!c.signal.aborted) {
        onAuthorityFailure(failure);
        setError("本次结果未确认，请刷新原邀请；创建结果未知时可核实原标识。");
      }
    } finally {
      active.current = false;
      if (!c.signal.aborted) setBusy(false);
    }
  }
  const requestScope = () => ({ ...scope, signal: controller.current?.signal });
  return (
    <section className={styles.panel} aria-label="正式成员邀请">
      <div className={styles.toolbar}>
        <h2>成员邀请</h2>
        <div>
          <Button
            variant="outline"
            disabled={busy || query.isFetching}
            onClick={() => void query.refetch()}
          >
            刷新邀请
          </Button>
          <Button
            disabled={
              busy ||
              retained.error ||
              !query.data?.canNotify ||
              !!unconfirmed ||
              !assignableRoles.length
            }
            onClick={() => setCreating(true)}
          >
            邀请成员
          </Button>
        </div>
      </div>
      <p>
        通过邮件邀请；对方使用已验证的受邀邮箱登录并接受后，才会成为企业成员。邮件服务器接收不代表已送达收件箱。
      </p>
      {error && <p role="alert">{error}</p>}
      {retained.error && (
        <p role="alert">
          无法读取原邀请标识，请先核对邀请列表；暂不能创建新邀请。
        </p>
      )}
      {query.isPending || query.isFetching ? (
        <p>正在读取邀请…</p>
      ) : query.isError ? (
        <p role="alert">邀请服务暂不可用。</p>
      ) : (
        <>
          {!query.data.canNotify && (
            <p role="status">当前实例尚未配置邀请邮件服务。</p>
          )}
          {unconfirmed && (
            <div>
              <p>
                保留的原邀请：{unconfirmed.email} · {unconfirmed.key}
              </p>
              <Button
                disabled={busy}
                onClick={() =>
                  void run(
                    () =>
                      createInvitation(requestScope(), unconfirmed.key, {
                        email: unconfirmed.email,
                        role: unconfirmed.role,
                      }),
                    true,
                  )
                }
              >
                核实原邀请
              </Button>
            </div>
          )}
          {creating && !unconfirmed && (
            <InvitationDrawer onClose={() => setCreating(false)}>
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const input = invitationInput.safeParse(
                    Object.fromEntries(new FormData(event.currentTarget)),
                  );
                  if (!input.success) {
                    setError("请填写有效邮箱和角色。");
                    return;
                  }
                  const key = crypto.randomUUID();
                  try {
                    sessionStorage.setItem(
                      storageKey,
                      JSON.stringify({ key, input: input.data }),
                    );
                    pendingChanged();
                    void run(
                      () => createInvitation(requestScope(), key, input.data),
                      true,
                    );
                  } catch {
                    setError("无法保存邀请标识，本次尚未发送。");
                  }
                }}
              >
                <p>
                  邀请加入企业空间，并预先指定角色；接受后再到「资源与额度」设置成员消费上限。
                </p>
                <div className={design.fields}>
                  <label>
                    受邀邮箱
                    <Input type="email" name="email" required maxLength={200} />
                  </label>
                  <label>
                    邀请角色
                    <select name="role" required>
                      {assignableRoles.map((role) => (
                        <option key={role} value={role}>
                          {roles[role]}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
                <div className={design.notice}>
                  无需填写姓名或密码。受邀人登录并验证此邮箱后，自行确认是否加入。
                </div>
                <footer>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setCreating(false)}
                  >
                    取消
                  </Button>
                  <Button disabled={busy} type="submit">
                    发送邀请邮件
                  </Button>
                </footer>
              </form>
            </InvitationDrawer>
          )}
          {query.data.items.length === 0 ? (
            <p>暂无邀请记录。</p>
          ) : (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <caption>
                  显示最近 {query.data.items.length} 条，共 {query.data.total}{" "}
                  条邀请
                </caption>
                <thead>
                  <tr>
                    <th>邮箱</th>
                    <th>角色</th>
                    <th>状态</th>
                    <th>通知</th>
                    <th>有效期</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {query.data.items.map((inv) => (
                    <tr key={inv.id}>
                      <td>{inv.contact}</td>
                      <td>{roles[inv.role]}</td>
                      <td>{states[inv.state]}</td>
                      <td>{deliveries[inv.deliveryState]}</td>
                      <td>{new Date(inv.expiresAt).toLocaleString("zh-CN")}</td>
                      <td>
                        {inv.state === "pending" ? (
                          <>
                            <Button
                              variant="outline"
                              disabled={busy || !query.data.canNotify}
                              onClick={() =>
                                void run(() =>
                                  invitationAction(
                                    requestScope(),
                                    inv.id,
                                    "resend",
                                  ),
                                )
                              }
                            >
                              重发邮件
                            </Button>
                            <Button
                              variant="outline"
                              disabled={busy}
                              onClick={() => {
                                if (
                                  window.confirm(
                                    `取消发送给 ${inv.contact} 的邀请？`,
                                  )
                                )
                                  void run(() =>
                                    invitationAction(
                                      requestScope(),
                                      inv.id,
                                      "cancel",
                                    ),
                                  );
                              }}
                            >
                              取消邀请
                            </Button>
                          </>
                        ) : inv.state === "accepting" ? (
                          <Button
                            variant="outline"
                            disabled={busy}
                            onClick={() =>
                              void run(() =>
                                readAdminInvitation(requestScope(), inv.id),
                              )
                            }
                          >
                            查询原结果
                          </Button>
                        ) : (
                          "—"
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </section>
  );
}
export function RecipientInvitation({
  userId,
  id,
}: {
  userId: string;
  id: string;
}) {
  const query = useQuery({
    queryKey: ["recipient-invitation", userId, id],
    queryFn: ({ signal }) => recipientInvitation(userId, id, undefined, signal),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const controller = useRef<AbortController | null>(null),
    active = useRef(false);
  useEffect(() => {
    const c = new AbortController();
    controller.current = c;
    return () => c.abort();
  }, []);
  async function act(action: "accept" | "decline") {
    const c = controller.current;
    if (!c || c.signal.aborted || active.current) return;
    active.current = true;
    setBusy(true);
    setError("");
    try {
      await recipientInvitation(userId, id, action, c.signal);
      if (!c.signal.aborted) await query.refetch();
    } catch {
      if (!c.signal.aborted)
        setError("本次结果未确认，请查询原邀请。不要另建邀请或重复加入。");
    } finally {
      active.current = false;
      if (!c.signal.aborted) setBusy(false);
    }
  }
  const data =
    query.isSuccess && !query.isFetching ? query.data.invitation : null;
  return (
    <main className={design.recipient}>
      <header className={design.brand}>
        <Image
          src="/console/sumi-logo.png"
          width={48}
          height={48}
          alt="硕米智能引擎"
        />
        <strong>硕米智能引擎</strong>
        <Link href="/">返回官网</Link>
      </header>
      <div className={design.content}>
        <section className={design.lead}>
          <h1>你收到一份企业协作邀请</h1>
          <p>
            企业邀请不会自动生效。
            <br />
            验证邀请账号后，由你主动确认是否加入。
          </p>
          <ol>
            <li>确认邀请企业与邀请人</li>
            <li>查看预分配角色与权限范围</li>
            <li>点击「接受邀请」后才建立成员关系</li>
          </ol>
        </section>
        <section className={design.recipientCard}>
          <h2>加入企业空间</h2>
          {error && <p role="alert">{error}</p>}
          {query.isPending || query.isFetching ? (
            <p>正在核实邀请和登录邮箱…</p>
          ) : query.isError ? (
            <>
              <p role="alert">
                无法读取此邀请。请使用受邀且已验证的邮箱登录；尚无账户请在官方登录页面注册并完成邮箱验证。
              </p>
              <Button asChild>
                <a
                  href={`/api/zitadel-auth/login?returnTo=${encodeURIComponent(`/invitations/${id}`)}`}
                >
                  前往官方登录
                </a>
              </Button>
            </>
          ) : data ? (
            <>
              <dl>
                <div>
                  <dt>邀请企业</dt>
                  <dd>{data.organizationName || data.organizationId}</dd>
                </div>
                <div>
                  <dt>受邀邮箱</dt>
                  <dd>{data.contact}</dd>
                </div>
                <div>
                  <dt>邀请角色</dt>
                  <dd>
                    {roles[data.role]}
                    <PermissionScope permissions={data.permissions} />
                  </dd>
                </div>
                <div>
                  <dt>邀请人</dt>
                  <dd>{data.creatorId}</dd>
                </div>
                <div>
                  <dt>有效期</dt>
                  <dd>{new Date(data.expiresAt).toLocaleString("zh-CN")}</dd>
                </div>
                <div>
                  <dt>状态</dt>
                  <dd>{states[data.state]}</dd>
                </div>
              </dl>
              {data.state === "pending" ? (
                <>
                  <p>
                    接受后获得上述企业角色权限。此操作不改变你原有企业、套餐或余额。
                  </p>
                  <footer>
                    <Button
                      variant="outline"
                      disabled={busy}
                      onClick={() => void act("decline")}
                    >
                      拒绝邀请
                    </Button>
                    <Button disabled={busy} onClick={() => void act("accept")}>
                      接受邀请
                    </Button>
                  </footer>
                </>
              ) : data.state === "accepted" ? (
                <>
                  <p>企业已授予访问权限。重新登录后选择该企业进入工作台。</p>
                  <Button asChild>
                    <a href="/api/zitadel-auth/login?returnTo=%2Fworkbench">
                      重新登录并进入工作台
                    </a>
                  </Button>
                </>
              ) : data.state === "accepting" ? (
                <p>加入结果待核实。这里只查询已发起的授权，不再次添加成员。</p>
              ) : (
                <p>此邀请已结束，无法继续加入。</p>
              )}
            </>
          ) : null}
          <Button
            variant="outline"
            disabled={busy || query.isFetching}
            onClick={() => void query.refetch()}
          >
            查询原邀请
          </Button>
        </section>
      </div>
    </main>
  );
}

function InvitationDrawer({
  children,
  onClose,
}: {
  children: React.ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null),
    close = useRef<HTMLButtonElement>(null),
    id = useId();
  useEffect(() => {
    const element = ref.current,
      previous = document.activeElement;
    element?.showModal();
    close.current?.focus();
    return () => {
      element?.close();
      if (previous instanceof HTMLElement && previous.isConnected)
        previous.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className={design.drawer}
      aria-labelledby={id}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <header>
        <h2 id={id}>邀请成员</h2>
        <Button
          ref={close}
          variant="ghost"
          aria-label="关闭邀请"
          onClick={onClose}
        >
          ×
        </Button>
      </header>
      {children}
    </dialog>
  );
}
