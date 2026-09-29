package image_test

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	img "one-api/common/image"
)

// This proxy only forwards to the local fixture. Numeric public addresses are
// protocol assertions, never real network destinations.
func mediaSOCKSFixture(t *testing.T, origin string, hits *atomic.Int32, originTLS bool) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				read := func(n int) []byte {
					b := make([]byte, n)
					if _, err := io.ReadFull(conn, b); err != nil {
						t.Error(err)
						return nil
					}
					return b
				}
				greeting := read(2)
				if len(greeting) != 2 || greeting[0] != 5 {
					return
				}
				if read(int(greeting[1])) == nil {
					return
				}
				_, _ = conn.Write([]byte{5, 2})
				auth := read(2)
				if len(auth) != 2 || auth[0] != 1 {
					return
				}
				user := read(int(auth[1]))
				length := read(1)
				if len(length) != 1 {
					return
				}
				password := read(int(length[0]))
				if string(user) != "proxyuser" || string(password) != "proxypass" {
					t.Error("SOCKS credentials changed")
					return
				}
				_, _ = conn.Write([]byte{1, 0})
				request := read(10)
				if len(request) != 10 {
					return
				}
				port := uint16(80)
				if originTLS {
					port = 443
				}
				if request[0] != 5 || request[1] != 1 || request[3] != 1 || net.IP(request[4:8]).String() != publicMediaAddress || binary.BigEndian.Uint16(request[8:10]) != port {
					t.Error("SOCKS target was not the approved numeric address")
					return
				}
				hits.Add(1)
				upstream, err := net.Dial("tcp", origin)
				if err != nil {
					t.Error(err)
					return
				}
				defer upstream.Close()
				_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				copied := make(chan struct{})
				go func() { _, _ = io.Copy(upstream, conn); close(copied) }()
				_, _ = io.Copy(conn, upstream)
				_ = conn.Close()
				<-copied
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); workers.Wait() })
	return "socks5://proxyuser:proxypass@" + listener.Addr().String()
}

func TestMediaProxyFailuresNeverFallBack(t *testing.T) {
	for _, stall := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "timeout"}[stall], func(t *testing.T) {
			finished := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { finished <- struct{}{} }()
				if !stall {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				_ = conn.SetReadDeadline(time.Now().Add(time.Second))
				var b [1]byte
				_, err = conn.Read(b[:])
				if !errors.Is(err, io.EOF) {
					t.Errorf("proxy connection not closed on cancellation: %v", err)
				}
			}))
			defer server.Close()
			var dials atomic.Int32
			client := img.MediaClientForTest(publicLookup, func(ctx context.Context, network, address string) (net.Conn, error) {
				dials.Add(1)
				if address != server.Listener.Addr().String() {
					return nil, errors.New("proxy bypass")
				}
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}, nil)
			client.Timeout = 80 * time.Millisecond
			setMediaFixture(t, client, server.URL)
			if _, err := img.RequestFile("http://example.com/image", "base64"); err == nil {
				t.Fatal("failed proxy accepted")
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("proxy handler did not exit")
			}
			if dials.Load() != 1 {
				t.Fatalf("unexpected retry/fallback: %d", dials.Load())
			}
		})
	}
}

func TestMediaTLSVerificationAndPublicIPv6(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
	defer server.Close()
	client := img.MediaClientForTest(publicLookup, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}, &tls.Config{})
	setMediaFixture(t, client, "")
	if _, err := img.RequestFile("https://example.com/image", "base64"); err == nil {
		t.Fatal("untrusted origin certificate accepted")
	}
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "example.com:8080" {
			t.Errorf("Host lost explicit port: %q", r.Host)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer plain.Close()
	img.ImageHttpClients = img.MediaClientForTest(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("2606:4700:4700::1111")}}, nil
	}, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "[2606:4700:4700::1111]:8080" {
			t.Errorf("IPv6 dial changed: %q", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, plain.Listener.Addr().String())
	}, nil)
	response, err := img.RequestFile("http://example.com:8080/image", "base64")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if strings.TrimSpace(string(body)) != "ok" {
		t.Fatal("IPv6 public fixture failed")
	}
}
