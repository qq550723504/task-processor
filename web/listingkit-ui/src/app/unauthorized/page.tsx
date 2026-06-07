import Link from "next/link";

export default function UnauthorizedPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6">
      <section className="w-full max-w-md rounded-lg border border-border bg-card p-6 shadow-sm">
        <h1 className="text-lg font-semibold text-foreground">Access denied</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          Your ZITADEL account is authenticated, but it is not allowed to access
          ListingKit.
        </p>
        <div className="mt-6 flex gap-3">
          <a
            href="/api/zitadel-auth/logout"
            className="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground"
          >
            Sign out
          </a>
          <Link
            href="/"
            className="inline-flex items-center rounded-md border border-border bg-background px-4 py-2 text-sm font-medium text-muted-foreground"
          >
            Back
          </Link>
        </div>
      </section>
    </main>
  );
}
