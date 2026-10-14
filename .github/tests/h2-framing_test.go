package workflows_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestHTTP2ResponseFramingHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields []hpack.HeaderField
		want   []string
	}{
		{"normal", []hpack.HeaderField{{Name: "content-length", Value: "0"}}, []string{"0"}},
		{"identical repeated lengths", []hpack.HeaderField{{Name: "content-length", Value: "0"}, {Name: "content-length", Value: "0"}}, []string{"0"}},
		{"conflicting lengths", []hpack.HeaderField{{Name: "content-length", Value: "0"}, {Name: "content-length", Value: "1"}}, nil},
		{"connection header", []hpack.HeaderField{{Name: "connection", Value: "close"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				preface := make([]byte, len(http2.ClientPreface))
				if _, err = io.ReadFull(conn, preface); err != nil {
					done <- err
					return
				}
				if string(preface) != http2.ClientPreface {
					done <- io.ErrUnexpectedEOF
					return
				}
				fr := http2.NewFramer(conn, conn)
				if err = fr.WriteSettings(); err != nil {
					done <- err
					return
				}
				for {
					frame, err := fr.ReadFrame()
					if err != nil {
						done <- err
						return
					}
					switch f := frame.(type) {
					case *http2.SettingsFrame:
						if !f.IsAck() {
							if err = fr.WriteSettingsAck(); err != nil {
								done <- err
								return
							}
						}
					case *http2.HeadersFrame:
						var block bytes.Buffer
						encoder := hpack.NewEncoder(&block)
						if err = encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
							done <- err
							return
						}
						for _, field := range tc.fields {
							if err = encoder.WriteField(field); err != nil {
								done <- err
								return
							}
						}
						err = fr.WriteHeaders(http2.HeadersFrameParam{StreamID: f.StreamID, EndHeaders: true, EndStream: true, BlockFragment: block.Bytes()})
						done <- err
						return
					}
				}
			}()
			transport := &http2.Transport{AllowHTTP: true, DialTLSContext: func(ctx context.Context, network, address string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Get("http://" + listener.Addr().String() + "/fixture")
			require.NoError(t, err)
			defer response.Body.Close()
			_, err = io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, <-done)
			require.Equal(t, tc.want, response.Header.Values("Content-Length"))
			require.Empty(t, response.Header.Values("Connection"))
		})
	}
}
