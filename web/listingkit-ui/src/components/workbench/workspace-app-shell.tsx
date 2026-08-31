"use client";

import { Home, Menu, Store, X, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import { useWorkbenchContext } from "@/components/providers/workbench-context-provider";
import {
  OrganizationSwitcher,
  workbenchErrorMessage,
} from "@/components/workbench/organization-switcher";
import { Button } from "@/components/ui/button";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";

const NO_ORGANIZATION_ROUTE = "/workbench/no-organization";
const MOBILE_NAVIGATION_ID = "workbench-mobile-navigation";

type WorkbenchNavItem = {
  href: string;
  icon: LucideIcon;
  label: string;
  match: "exact" | "prefix";
};

const WORKBENCH_NAV_GROUPS = [
  {
    label: "工作",
    items: [
      { label: "工作台", href: "/workbench", icon: Home, match: "exact" },
    ],
  },
  {
    label: "店铺中心",
    items: [
      {
        label: "我的店铺",
        href: "/workbench/stores",
        icon: Store,
        match: "prefix",
      },
    ],
  },
] as const satisfies readonly {
  label: string;
  items: readonly WorkbenchNavItem[];
}[];

export function WorkspaceAppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname() ?? "/workbench";
  const router = useRouter();
  const context = useWorkbenchContext();

  const shouldRedirectToNoOrganization =
    !context.isLoading &&
    !context.error &&
    !context.blockingError &&
    context.organizations.length === 0 &&
    pathname !== NO_ORGANIZATION_ROUTE;

  useEffect(() => {
    if (shouldRedirectToNoOrganization) {
      router.replace(NO_ORGANIZATION_ROUTE);
    }
  }, [router, shouldRedirectToNoOrganization]);

  if (context.isLoading) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background px-6">
        <p className="text-sm text-muted-foreground" role="status">
          正在加载工作台...
        </p>
      </main>
    );
  }

  if (context.blockingError) {
    return (
      <AccessState
        action={() => window.location.reload()}
        code={context.blockingError.code}
      />
    );
  }

  if (context.error) {
    return <AccessState action={context.retry} code={context.error.code} />;
  }

  if (shouldRedirectToNoOrganization) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-background px-6">
        <p className="text-sm text-muted-foreground" role="status">
          正在打开企业访问说明...
        </p>
      </main>
    );
  }

  return (
    <WorkbenchFrame pathname={pathname}>
      {context.selectionRequired ? (
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

function WorkbenchFrame({
  children,
  pathname,
}: {
  children: ReactNode;
  pathname: string;
}) {
  const [mobileNavigationOpen, setMobileNavigationOpen] = useState(false);
  const context = useWorkbenchContext();

  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="border-b border-sidebar-border p-4">
          <Link className="font-semibold tracking-tight" href="/workbench">
            硕米智能引擎
          </Link>
        </SidebarHeader>
        <SidebarContent>
          <WorkbenchNavigation ariaLabel="工作台导航" pathname={pathname} />
        </SidebarContent>
        <SidebarRail />
      </Sidebar>
      <SidebarInset>
        <header className="flex min-h-16 items-center gap-3 border-b bg-background px-4 sm:px-6">
          <Button
            aria-controls={MOBILE_NAVIGATION_ID}
            aria-expanded={mobileNavigationOpen}
            aria-label={
              mobileNavigationOpen ? "关闭工作台导航" : "打开工作台导航"
            }
            className="md:hidden"
            onClick={() => setMobileNavigationOpen((current) => !current)}
            size="icon"
            variant="ghost"
          >
            {mobileNavigationOpen ? (
              <X aria-hidden="true" />
            ) : (
              <Menu aria-hidden="true" />
            )}
          </Button>
          <SidebarTrigger
            aria-label="折叠桌面导航"
            className="hidden md:inline-flex"
          />
          <div className="ml-auto flex min-w-0 items-center gap-3">
            <DelegatedOperationIndicator
              effectiveOrganization={context.effectiveOrganization}
              homeOrganizationId={context.homeOrganizationId}
              organizations={context.organizations}
            />
            <OrganizationSwitcher />
          </div>
        </header>
        {mobileNavigationOpen ? (
          <div className="border-b bg-background px-4 py-3 md:hidden">
            <WorkbenchNavigation
              ariaLabel="移动工作台导航"
              id={MOBILE_NAVIGATION_ID}
              onNavigate={() => setMobileNavigationOpen(false)}
              pathname={pathname}
            />
          </div>
        ) : null}
        <main className="min-w-0 flex-1 bg-muted/20">{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
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

function WorkbenchNavigation({
  ariaLabel,
  id,
  onNavigate,
  pathname,
}: {
  ariaLabel: string;
  id?: string;
  onNavigate?: () => void;
  pathname: string;
}) {
  return (
    <nav aria-label={ariaLabel} id={id}>
      {WORKBENCH_NAV_GROUPS.map((group) => (
        <SidebarGroup key={group.label}>
          <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {group.items.map((item) => {
                const active = isActiveWorkbenchNavItem(pathname, item);
                const Icon = item.icon;
                return (
                  <SidebarMenuItem key={item.href}>
                    <SidebarMenuButton asChild isActive={active}>
                      <Link
                        aria-current={active ? "page" : undefined}
                        href={item.href}
                        onClick={onNavigate}
                      >
                        <Icon data-icon="inline-start" />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      ))}
    </nav>
  );
}

function isActiveWorkbenchNavItem(pathname: string, item: WorkbenchNavItem) {
  return item.match === "prefix"
    ? pathname === item.href || pathname.startsWith(`${item.href}/`)
    : pathname === item.href;
}

function AccessState({ action, code }: { action: () => void; code: string }) {
  return (
    <main className="flex min-h-svh items-center justify-center bg-background px-6">
      <section className="max-w-md rounded-xl border bg-card p-6 text-center shadow-sm">
        <h1 className="text-lg font-semibold" role="alert">
          {workbenchErrorMessage(code)}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          为保护企业数据，工作台内容已停止加载。
        </p>
        <Button className="mt-5" onClick={action} variant="outline">
          重新加载
        </Button>
      </section>
    </main>
  );
}
