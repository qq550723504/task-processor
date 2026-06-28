package generation

func CloneTask(task Task) Task {
	cloned := task
	cloned.Lineage = append([]string(nil), task.Lineage...)
	cloned.SourceAssetIDs = append([]string(nil), task.SourceAssetIDs...)
	cloned.Metadata = cloneTaskMetadata(task.Metadata)
	return cloned
}

func CloneTasks(tasks []Task) []Task {
	if len(tasks) == 0 {
		return nil
	}
	cloned := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		cloned = append(cloned, CloneTask(task))
	}
	return cloned
}

func MergeTasks(existing []Task, updates []Task) []Task {
	if len(existing) == 0 {
		return CloneTasks(updates)
	}
	byID := make(map[string]Task, len(existing)+len(updates))
	for _, item := range existing {
		byID[item.ID] = CloneTask(item)
	}
	for _, item := range updates {
		byID[item.ID] = CloneTask(item)
	}
	out := make([]Task, 0, len(byID))
	for _, item := range existing {
		out = append(out, CloneTask(byID[item.ID]))
		delete(byID, item.ID)
	}
	for _, item := range updates {
		if _, ok := byID[item.ID]; !ok {
			continue
		}
		out = append(out, CloneTask(byID[item.ID]))
		delete(byID, item.ID)
	}
	return out
}
