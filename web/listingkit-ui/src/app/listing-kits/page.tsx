import { Suspense } from "react";

import { TaskListPage } from "@/components/listingkit/task-list-page";

export default function ListingKitTasksPage() {
  return (
    <Suspense>
      <TaskListPage />
    </Suspense>
  );
}
