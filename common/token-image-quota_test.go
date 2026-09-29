package common_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/types"
)

func TestImageQuotaEstimateBoundaries(t *testing.T) {
	for _, n := range []int{-1, math.MinInt, math.MaxInt, math.MaxInt/1000 + 1} {
		for _, input := range []any{types.ImageRequest{N: n}, types.ImageEditRequest{N: n}} {
			_, err := common.CountTokenImage(input)
			require.Error(t, err)
		}
	}
	for _, tc := range []struct {
		request types.ImageRequest
		want    int
	}{
		{types.ImageRequest{N: 0}, 0},
		{types.ImageRequest{N: 2}, 2000},
		{types.ImageRequest{N: math.MaxInt / 1000}, (math.MaxInt / 1000) * 1000},
		{types.ImageRequest{Model: "recraftv3", Style: "vector_illustration", N: 2}, 4000},
		{types.ImageRequest{Model: "dall-e-3", Size: "1024x1024", Quality: "hd", N: 1}, 2000},
	} {
		got, err := common.CountTokenImage(tc.request)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}
