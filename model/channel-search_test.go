package model_test

import (
	"path/filepath"
	"testing"

	"one-api/model"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestChannelNameSearchIncludesGroupedChannels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channels.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	fixtures := []model.Channel{
		{Id: 49, Name: "k_2", Tag: "Gemini", Type: 25, Status: 1, Group: "vip", Models: "gemini-test", Key: "private-fixture"},
		{Id: 50, Name: "other-key", Tag: "Gemini", Type: 25, Status: 1, Group: "vip", Models: "gemini-test"},
		{Id: 51, Name: "k_2-standalone", Type: 1, Status: 1, Group: "default", Models: "gpt-test"},
		{Id: 52, Name: "k_2-disabled", Tag: "Disabled", Type: 25, Status: 2, Group: "vip"},
		{Id: 53, Name: "k_2-deleted", Tag: "Deleted", Type: 25, Status: 1},
	}
	require.NoError(t, db.Create(&fixtures).Error)
	require.NoError(t, db.Delete(&model.Channel{}, 53).Error)
	tests := []struct {
		name      string
		filter    model.Channel
		filterTag int
		ids       []int
	}{
		{"non-representative member name", model.Channel{Name: "k_2", Status: 1}, 0, []int{49, 51}},
		{"tag name", model.Channel{Name: "Gemini"}, 0, []int{50}},
		{"name plus tag filter", model.Channel{Name: "k_2", Tag: "Gemini"}, 0, []int{49}},
		{"only grouped", model.Channel{Name: "k_2", Status: 1}, 2, []int{49}},
		{"only ungrouped", model.Channel{Name: "k_2"}, 1, []int{51}},
		{"combined filters", model.Channel{Name: "k_2", Type: 25, Status: 1, Group: "vip", Models: "gemini-test"}, 0, []int{49}},
		{"unknown", model.Channel{Name: "missing"}, 0, []int{}},
		{"quoted search is data", model.Channel{Name: "' OR 1=1 --"}, 0, []int{}},
		{"unfiltered grouped list", model.Channel{}, 0, []int{50, 51, 52}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := model.SearchChannelsParams{Channel: tt.filter, FilterTag: tt.filterTag, PaginationParams: model.PaginationParams{Page: 1, Size: 10, Order: "id"}}
			result, err := model.GetChannelsList(&params)
			require.NoError(t, err)
			ids := []int{}
			for _, c := range *result.Data {
				ids = append(ids, c.Id)
				require.Empty(t, c.Key, "list responses must not expose channel keys")
			}
			require.Equal(t, tt.ids, ids)
			require.Equal(t, int64(len(tt.ids)), result.TotalCount)
		})
	}
	params := model.SearchChannelsParams{Channel: model.Channel{Name: "k_2", Status: 1}, PaginationParams: model.PaginationParams{Page: 2, Size: 1, Order: "id"}}
	page, err := model.GetChannelsList(&params)
	require.NoError(t, err)
	require.Equal(t, int64(2), page.TotalCount)
	require.Len(t, *page.Data, 1)
	require.Equal(t, 51, (*page.Data)[0].Id)
}
