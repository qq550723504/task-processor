import { TaskCreatePage } from "@/components/listingkit/tasks/task-create-page";

export default async function ListingKitNewTaskPage({
  searchParams,
}: {
  searchParams: Promise<{
    fromTask?: string;
    focus?: "text" | "imageUrls";
    issues?: string;
  }>;
}) {
  const { fromTask, focus, issues } = await searchParams;
  const parsedIssues = issues
    ?.split(",")
    .map((value) => value.trim())
    .filter((value): value is "text" | "imageUrls" => value === "text" || value === "imageUrls");

  return (
    <div className="flex flex-1 items-center justify-center py-16">
      <TaskCreatePage fromTask={fromTask} focus={focus} issues={parsedIssues} />
    </div>
  );
}
