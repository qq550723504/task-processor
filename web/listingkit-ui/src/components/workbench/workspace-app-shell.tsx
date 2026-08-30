"use client";

import { Home } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";

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
  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="border-b border-sidebar-border p-4">
          <Link className="font-semibold tracking-tight" href="/workbench">
            硕米智能引擎
          </Link>
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel>工作</SidebarGroupLabel>
            <SidebarGroupContent>
              <nav aria-label="工作台导航">
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      asChild
                      isActive={
                        pathname === "/workbench" ||
                        pathname.startsWith("/workbench/")
                      }
                    >
                      <Link
                        aria-current={
                          pathname === "/workbench" ||
                          pathname.startsWith("/workbench/")
                            ? "page"
                            : undefined
                        }
                        href="/workbench"
                      >
                        <Home data-icon="inline-start" />
                        <span>工作台</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </nav>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarRail />
      </Sidebar>
      <SidebarInset>
        <header className="flex min-h-16 items-center gap-3 border-b bg-background px-4 sm:px-6">
          <SidebarTrigger />
          <div className="ml-auto">
            <OrganizationSwitcher />
          </div>
        </header>
        <main className="min-w-0 flex-1 bg-muted/20">{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
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
