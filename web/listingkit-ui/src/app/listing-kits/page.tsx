import { Suspense } from "react";

import { TaskListPage } from "@/components/listingkit/tasks/task-list-page";

export default function ListingKitTasksPage() {
  return (
    <Suspense>
      <TaskListPage />
    </Suspense>
  );
}
