import { TaskCreatePage } from "@/components/listingkit/tasks/task-create-page";

export default async function ListingKitNewTaskPage({
  searchParams,
}: {
  searchParams: Promise<{
    fromTask?: string;
    product_key?: string;
    focus?: "text" | "imageUrls";
    issues?: string;
  }>;
}) {
  const { fromTask, focus, issues, product_key: productKey } = await searchParams;
  const parsedIssues = issues
    ?.split(",")
    .map((value) => value.trim())
    .filter((value): value is "text" | "imageUrls" => value === "text" || value === "imageUrls");

  return (
    <div className="flex flex-1 py-6">
      <TaskCreatePage fromTask={fromTask} focus={focus} issues={parsedIssues} productKey={productKey} />
    </div>
  );
}
