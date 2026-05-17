import Link from "next/link";

export default function UnauthorizedPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-100 px-6">
      <section className="w-full max-w-md rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
        <h1 className="text-lg font-semibold text-zinc-950">Access denied</h1>
        <p className="mt-3 text-sm leading-6 text-zinc-600">
          Your ZITADEL account is authenticated, but it is not allowed to access
          ListingKit.
        </p>
        <div className="mt-6 flex gap-3">
          <a
            href="/api/zitadel-auth/logout"
            className="inline-flex items-center rounded-md bg-zinc-950 px-4 py-2 text-sm font-medium text-white"
          >
            Sign out
          </a>
          <Link
            href="/"
            className="inline-flex items-center rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700"
          >
            Back
          </Link>
        </div>
      </section>
    </main>
  );
}
