export default function NoOrganizationPage() {
  return (
    <section
      aria-labelledby="no-organization-title"
      className="mx-auto w-full max-w-2xl px-4 py-16 text-center sm:px-6"
    >
      <h1
        className="text-2xl font-semibold tracking-tight"
        id="no-organization-title"
      >
        暂无可用企业
      </h1>
      <p className="mt-3 text-sm text-muted-foreground">
        当前账号还没有获得任何企业工作台权限，请联系企业管理员完成授权后重新加载。
      </p>
    </section>
  );
}
