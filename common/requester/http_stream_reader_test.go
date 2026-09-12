package requester_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"one-api/common/requester"
)

func TestOptInStreamCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		check      bool
		want       error
	}{{"legacy", "partial\n", false, io.EOF}, {"truncated", "partial\n", true, io.ErrUnexpectedEOF}, {"unterminated_final_line", "done", true, io.EOF}} {
		t.Run(tc.name, func(t *testing.T) {
			done := false
			handler := func(line *[]byte, data chan string, errs chan error) {
				done = string(*line) == "done"
				data <- string(*line)
			}
			var checks []func() error
			if tc.check {
				checks = append(checks, func() error {
					if !done {
						return io.ErrUnexpectedEOF
					}
					return nil
				})
			}
			stream, e := requester.RequestStream[string](nil, &http.Response{Body: io.NopCloser(strings.NewReader(tc.body))}, handler, checks...)
			require.Nil(t, e)
			defer stream.Close()
			data, errs := stream.Recv()
			received := false
			for {
				select {
				case <-data:
					received = true
				case err := <-errs:
					require.ErrorIs(t, err, tc.want)
					require.True(t, received)
					return
				case <-time.After(3 * time.Second):
					t.Fatal("stream did not finish")
				}
			}
		})
	}
}
