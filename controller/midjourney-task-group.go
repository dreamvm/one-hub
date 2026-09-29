package controller

import "one-api/model"

// Upstream IDs can repeat across channels and can have multiple local rows
// (for example, an upstream already-existing-task response).
func groupMidjourneyTasks(tasks []*model.Midjourney) (map[int]map[string][]*model.Midjourney, []int) {
	groups := make(map[int]map[string][]*model.Midjourney)
	var missing []int
	for _, task := range tasks {
		if task.MjId == "" {
			missing = append(missing, task.Id)
			continue
		}
		if groups[task.ChannelId] == nil {
			groups[task.ChannelId] = make(map[string][]*model.Midjourney)
		}
		groups[task.ChannelId][task.MjId] = append(groups[task.ChannelId][task.MjId], task)
	}
	return groups, missing
}
