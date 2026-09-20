import { RegistrationPage } from "@/components/workbench/referrals/registration-page";

export default async function ReferralRegisterPage({ searchParams }: { searchParams?: Promise<Record<string, string | string[] | undefined>> }) {
  const params = (await searchParams) ?? {};
  const code = typeof params.code === "string" ? params.code : "";
  return <RegistrationPage code={code} />;
}
