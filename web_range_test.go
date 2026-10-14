package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"one-api/router"
)

func TestStaticRangeBound(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	r := gin.New()
	r.GET("/api/range-control", func(c *gin.Context) { c.String(http.StatusOK, c.GetHeader("Range")) })
	router.SetWebRouter(r, buildFS, indexPage)
	for _, tc := range []struct {
		name   string
		ranges []string
		want   int
	}{
		{"normal full download", nil, http.StatusOK},
		{"normal single range", []string{"bytes=0-9"}, http.StatusPartialContent},
		{"normal two ranges", []string{"bytes=0-3,8-11"}, http.StatusPartialContent},
		{"sixteen ranges", []string{"bytes=" + strings.Repeat("0-0,", 15) + "0-0"}, http.StatusPartialContent},
		{"seventeen ranges", []string{"bytes=" + strings.Repeat("0-0,", 16) + "0-0"}, http.StatusRequestedRangeNotSatisfiable},
		{"1024 bytes", []string{"bytes=" + strings.Repeat("0", 1016) + "-1"}, http.StatusPartialContent},
		{"1025 bytes", []string{"bytes=" + strings.Repeat("0", 1017) + "-1"}, http.StatusRequestedRangeNotSatisfiable},
		{"many ranges", []string{"bytes=" + strings.Repeat("0-0,", 32) + "0-0"}, http.StatusRequestedRangeNotSatisfiable},
		{"long range", []string{"bytes=" + strings.Repeat("0", 4096) + "-1"}, http.StatusRequestedRangeNotSatisfiable},
		{"repeated headers", []string{"bytes=0-1", "bytes=2-3"}, http.StatusRequestedRangeNotSatisfiable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/logo-loading.svg", nil)
			for _, v := range tc.ranges {
				req.Header.Add("Range", v)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code)
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/range-control", nil)
	req.Header.Set("Range", "bytes="+strings.Repeat("0-0,", 32)+"0-0")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, req.Header.Get("Range"), w.Body.String())
}
