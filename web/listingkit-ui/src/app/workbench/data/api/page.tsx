import { connection } from "next/server";
import { notFound } from "next/navigation";
import { DataServicesPage } from "@/components/workbench/data-services/data-services-page";
export default async function Page() { await connection(); if (process.env.LISTINGKIT_DATA_SERVICES_ENABLED !== "true")
    notFound(); return <DataServicesPage mode="api"/>; }
