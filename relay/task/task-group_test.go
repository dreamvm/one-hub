package task

import (
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/model"
)

func TestTaskPollBatchesRetainLocalIdentity(t *testing.T) {
	rows := []*model.Task{{ID: 1, ChannelId: 10, TaskID: "same"}, {ID: 2, ChannelId: 20, TaskID: "same"}, {ID: 3, ChannelId: 10, TaskID: "same"}, {ID: 4, ChannelId: 10, TaskID: "different"}, {ID: 5, ChannelId: 10}}
	batches, missing := groupTaskPollBatches(rows)
	require.Equal(t, []int64{5}, missing)
	seen := map[int64]bool{}
	for _, batch := range batches {
		for _, id := range batch.ids {
			row := batch.tasks[id]
			require.Equal(t, batch.channelID, row.ChannelId)
			require.False(t, seen[row.ID])
			seen[row.ID] = true
		}
	}
	require.Equal(t, map[int64]bool{1: true, 2: true, 3: true, 4: true}, seen)
}
