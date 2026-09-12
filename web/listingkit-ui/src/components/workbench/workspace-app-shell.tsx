"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { useTheme } from "next-themes";
import { Menu, X } from "lucide-react";
import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import { OrganizationSwitcher, workbenchErrorMessage } from "@/components/workbench/organization-switcher";
import { Button } from "@/components/ui/button";
import { ConsoleNavigation } from "@/components/workbench/console/console-navigation";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";

const NO_ORGANIZATION_ROUTE = "/workbench/no-organization";
const MOBILE_NAVIGATION_ID = "workbench-mobile-navigation";
const subscribeToHydration = () => () => {};
const getClientHydrationSnapshot = () => true;
const getServerHydrationSnapshot = () => false;
export function WorkspaceAppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname() ?? "/workbench";
  const router = useRouter();
  const context = useWorkbenchContext();
  // Personal identity is bootstrapped by the server page, independently of enterprise grants.
  const isPersonalProfile = pathname === "/workbench/account/profile";
  const authenticationError = [context.blockingError, context.error].find(error => error?.code === "AUTHENTICATION_REQUIRED");

  const shouldRedirectToNoOrganization =
    !context.isLoading &&
    !context.error &&
    !context.blockingError &&
    context.organizations.length === 0 &&
    !isPersonalProfile &&
    pathname !== NO_ORGANIZATION_ROUTE;
  const shouldLeaveNoOrganization =
    !context.isLoading &&
    !context.error &&
    !context.blockingError &&
    context.organizations.length > 0 &&
    pathname === NO_ORGANIZATION_ROUTE;

  const redirectToLogin = () => {
    const returnTo = `${pathname}${window.location.search}`;
    router.replace(`/login?returnTo=${encodeURIComponent(returnTo)}`);
  };

  useEffect(() => {
    if (shouldRedirectToNoOrganization) {
      router.replace(NO_ORGANIZATION_ROUTE);
    } else if (shouldLeaveNoOrganization) {
      router.replace("/workbench");
    }
  }, [router, shouldLeaveNoOrganization, shouldRedirectToNoOrganization]);

  if (authenticationError) {
    const match = pathname === "/capture/1688" && typeof window !== "undefined" && !window.location.search
      ? /^#operationKey=([0-9a-f-]{36})$/.exec(window.location.hash) : null;
    const browserRecovery = !!match && isAcquisitionUUID(match[1]!);
    return <AccessState action={browserRecovery ? () => window.location.reload() : redirectToLogin} code={authenticationError.code} browserRecovery={browserRecovery} />;
  }

  if (context.isLoading && !isPersonalProfile) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background px-6">
        <p className="text-sm text-muted-foreground" role="status">
          正在加载工作台...
        </p>
      </main>
    );
  }

  if (context.blockingError && !isPersonalProfile) {
    return (
      <AccessState
        action={
          context.blockingError.code === "AUTHENTICATION_REQUIRED"
            ? redirectToLogin
            : () => window.location.reload()
        }
        code={context.blockingError.code}
      />
    );
  }

  if (context.error && !isPersonalProfile) {
    return (
      <AccessState
        action={
          context.error.code === "AUTHENTICATION_REQUIRED"
            ? redirectToLogin
            : context.retry
        }
        code={context.error.code}
      />
    );
  }

  if (shouldRedirectToNoOrganization || shouldLeaveNoOrganization) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background px-6">
        <p className="text-sm text-muted-foreground" role="status">
          正在打开企业访问说明...
        </p>
      </main>
    );
  }

  return (
    <WorkbenchFrame key={pathname} pathname={pathname}>
      {context.selectionRequired && !isPersonalProfile ? (
        <section
          className="flex min-h-[40vh] items-center justify-center px-6 text-center"
          role="status"
        >
          <div>
            <h1 className="text-xl font-semibold">请选择企业以继续</h1>
            <p className="mt-2 text-sm text-muted-foreground">
              选择后才会加载该企业的工作台数据。
            </p>
          </div>
        </section>
      ) : (
        children
      )}
    </WorkbenchFrame>
  );
}

function WorkbenchFrame({ children, pathname }: { children: ReactNode; pathname: string }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const context = useWorkbenchContext();
  const contextConfirmed = !context.isLoading && !context.isSwitching && !context.error && !context.blockingError;
  const { resolvedTheme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(subscribeToHydration, getClientHydrationSnapshot, getServerHydrationSnapshot);
  const light = mounted && resolvedTheme === "light";
  function closeNavigation() { setMobileOpen(false); trigger.current?.focus(); }
  return <div onKeyDown={(event) => { if (mobileOpen && event.key === "Escape") { event.preventDefault(); closeNavigation(); } }}>
    <a className="sr-only focus:not-sr-only" href="#console-main">跳到页面内容</a>
    <div className="console-frame">
      <aside className="console-sidebar">
        <Link href="/workbench" className="console-brand" prefetch={false}><Image src="/console/sumi-logo.png" alt="" width={42} height={42} unoptimized /><span><strong>硕米智能引擎</strong><small>SUMI AI ENGINE</small></span></Link>
        <ConsoleNavigation key={pathname} pathname={pathname} ariaLabel="工作台导航" />
        <p className="console-sidebar-footer">SUMI AI ENGINE</p>
      </aside>
      <div className="console-body">
        <header className="console-topbar">
          <Button ref={trigger} className="md:hidden" variant="ghost" size="icon" aria-controls={MOBILE_NAVIGATION_ID} aria-expanded={mobileOpen} aria-label={mobileOpen ? "关闭工作台导航" : "打开工作台导航"} onClick={() => setMobileOpen((value) => !value)}>{mobileOpen ? <X aria-hidden="true" /> : <Menu aria-hidden="true" />}</Button>
          <div className="console-top-context"><p className="console-tagline">打造属于自己的AI电商团队</p><div className="console-organization">{contextConfirmed ? <OrganizationSwitcher /> : <span className="text-xs text-muted-foreground">企业上下文尚未确认</span>}</div></div>
          <div className="console-global-actions">
            <Button disabled title="客服服务暂未接入" variant="outline">联系客服</Button>
            <Button disabled title="通知服务暂未接入" variant="outline">通知</Button>
            <button className="console-theme-toggle" type="button" role="switch" aria-label="浅色模式" aria-checked={light} onClick={() => setTheme(light ? "dark" : "light")}>{light ? "浅色" : "深色"}<span aria-hidden="true" /></button>
            <details className="console-user"><summary>我的账户 ⌄</summary><div><p className="break-words text-xs text-muted-foreground">已登录账号</p><p className="mt-1 break-all text-sm">{contextConfirmed ? context.user?.id : null}</p><Link href="/workbench/account/profile" prefetch={false}>账户资料</Link><a href="/api/zitadel-auth/logout">退出登录</a></div></details>
          </div>
        </header>
        {contextConfirmed && context.effectiveOrganization && context.effectiveOrganization.id !== context.homeOrganizationId ? <div className="console-delegation"><DelegatedOperationIndicator effectiveOrganization={context.effectiveOrganization} homeOrganizationId={context.homeOrganizationId} organizations={context.organizations} /></div> : null}
        {mobileOpen ? <div className="console-mobile-nav" id={MOBILE_NAVIGATION_ID}><ConsoleNavigation key={pathname} pathname={pathname} ariaLabel="移动工作台导航" onNavigate={closeNavigation} /></div> : null}
        <main className="console-content" id="console-main" tabIndex={-1}>{children}</main>
      </div>
    </div>
  </div>;
}
function DelegatedOperationIndicator({
  effectiveOrganization,
  homeOrganizationId,
  organizations,
}: {
  effectiveOrganization: ReturnType<
    typeof useWorkbenchContext
  >["effectiveOrganization"];
  homeOrganizationId: string | null;
  organizations: ReturnType<typeof useWorkbenchContext>["organizations"];
}) {
  if (
    !effectiveOrganization ||
    !homeOrganizationId ||
    effectiveOrganization.id === homeOrganizationId
  ) {
    return null;
  }
  const effectiveOrganizationName =
    effectiveOrganization.name.trim() || "当前企业";
  const homeOrganizationName =
    organizations.find((organization) => organization.id === homeOrganizationId)
      ?.name || "其他归属企业";
  return (
    <p
      aria-label="企业代管状态"
      className="min-w-0 text-right text-xs font-medium text-amber-700 dark:text-amber-300"
      role="status"
    >
      <span className="block truncate">
        正在代管{effectiveOrganizationName}
      </span>
      <span className="block truncate text-muted-foreground">
        账号归属{homeOrganizationName}
      </span>
    </p>
  );
}

function AccessState({ action, code, browserRecovery = false }: { action: () => void; code: string; browserRecovery?: boolean }) {
  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-6">
      <section className="max-w-md rounded-xl border border-border bg-card p-6 text-center shadow-sm">
        <h1 className="text-lg font-semibold" role="alert">
          {workbenchErrorMessage(code)}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {browserRecovery ? "请保留此恢复页面。在新标签页登录后，返回此页刷新并核实原操作；不会重新提交。" : "为保护企业数据，工作台内容已停止加载。"}
        </p>
        {browserRecovery ? <a className="mt-4 block text-sm underline" href="/login?returnTo=%2Fworkbench" target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">在新标签页重新登录</a> : null}
        <Button className="mt-5" onClick={action} variant="outline">
          {browserRecovery ? "登录完成后刷新此页核实" : "重新加载"}
        </Button>
      </section>
    </main>
  );
}
