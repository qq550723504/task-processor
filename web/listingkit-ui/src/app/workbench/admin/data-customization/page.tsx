import { connection } from "next/server";
import { notFound } from "next/navigation";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { SpecialistPage } from "@/components/workbench/data-services/specialist-page";
export default async function Page() { await connection(); if (process.env.LISTINGKIT_DATA_SERVICES_ENABLED !== "true")
    notFound(); const session = await serverAuth(); const identity = readZitadelIdentityFromSession(session); if (typeof identity?.userId !== "string")
    notFound(); return <SpecialistPage userId={identity.userId}/>; }
