package workflows_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaintainedStdlibBoundsRangeCount(t *testing.T) {
	t.Setenv("GODEBUG", "")
	for _, tc := range []struct {
		name, header string
		want         int
	}{
		{"full", "", http.StatusOK},
		{"single", "bytes=0-9", http.StatusPartialContent},
		{"two", "bytes=0-3,8-11", http.StatusPartialContent},
		{"over default limit", "bytes=" + strings.Repeat("0-0,", 200) + "0-0", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/fixture.txt", nil)
			req.Header.Set("Range", tc.header)
			w := httptest.NewRecorder()
			http.ServeContent(w, req, "fixture.txt", time.Time{}, strings.NewReader(strings.Repeat("x", 1024)))
			require.Equal(t, tc.want, w.Code)
		})
	}
}
