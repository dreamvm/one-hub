package image_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"one-api/common/config"
	img "one-api/common/image"
)

const publicMediaAddress = "93.184.216.34"

func publicLookup(_ context.Context, _ string) ([]net.IPAddr, error) {
	return []net.IPAddr{{IP: net.ParseIP(publicMediaAddress)}}, nil
}

func setMediaFixture(t *testing.T, client *http.Client, proxy string) {
	t.Helper()
	oldClient, oldProxy, oldWorker, oldKey := img.ImageHttpClients, config.ChatImageRequestProxy, config.CFWorkerImageUrl, config.CFWorkerImageKey
	img.ImageHttpClients, config.ChatImageRequestProxy, config.CFWorkerImageUrl = client, proxy, ""
	t.Cleanup(func() {
		img.ImageHttpClients, config.ChatImageRequestProxy, config.CFWorkerImageUrl, config.CFWorkerImageKey = oldClient, oldProxy, oldWorker, oldKey
	})
}

func TestMediaAddressPolicy(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.1.1", "0.0.0.0", "224.0.0.1", "240.1.2.3", "192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "::", "::1", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "2001:db8::1", "2001::1", "3fff::1", "2001:4860::1%eth0"} {
		if img.PublicMediaIPForTest(address) {
			t.Errorf("allowed special address %s", address)
		}
	}
	for _, address := range []string{publicMediaAddress, "8.8.8.8", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if !img.PublicMediaIPForTest(address) {
			t.Errorf("rejected public address %s", address)
		}
	}
}

func TestMediaRejectsDNSAndURLVariantsBeforeDial(t *testing.T) {
	cases := []struct {
		name, target string
		addresses    []net.IPAddr
	}{
		{"private DNS", "http://example.com/image", []net.IPAddr{{IP: net.ParseIP("10.1.2.3")}}},
		{"mixed DNS", "http://example.com/image", []net.IPAddr{{IP: net.ParseIP(publicMediaAddress)}, {IP: net.ParseIP("::1")}}},
		{"empty DNS", "http://example.com/image", nil},
		{"mapped", "http://[::ffff:127.0.0.1]/image", nil},
		{"IDNA normalized loopback", "http://１２７。０。０。１/image", nil},
		{"zone", "http://[fe80::1%25lo0]/image", nil},
		{"userinfo", "http://example.com@127.0.0.1/image", nil},
		{"fragment", "http://127.0.0.1/image#example.com", nil},
		{"file scheme", "file:///tmp/image", nil},
		{"scheme alias", "http+unix://example.com/image", nil},
		{"opaque", "http:example.com/image", nil},
		{"bad port", "http://example.com:65536/image", nil},
		{"integer IP resolver", "http://2130706433/image", []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
		{"trailing dot", "http://localhost./image", []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dials atomic.Int32
			client := img.MediaClientForTest(func(context.Context, string) ([]net.IPAddr, error) { return tc.addresses, nil }, func(context.Context, string, string) (net.Conn, error) {
				dials.Add(1)
				return nil, errors.New("unexpected dial")
			}, nil)
			setMediaFixture(t, client, "")
			response, err := img.RequestFile(tc.target, "base64")
			if response != nil {
				response.Body.Close()
			}
			if err == nil || dials.Load() != 0 {
				t.Fatalf("err=%v dials=%d", err, dials.Load())
			}
		})
	}
}

func TestMediaPublicDownloadsAndProxyPinning(t *testing.T) {
	for _, originTLS := range []bool{false, true} {
		for _, proxyKind := range []string{"direct", "http", "https", "socks5", "socks5h"} {
			scheme := "http"
			if originTLS {
				scheme = "https"
			}
			t.Run(scheme+"/"+proxyKind, func(t *testing.T) {
				pixel, _ := base64.StdEncoding.DecodeString(mediaPixel)
				serveMedia := func(w http.ResponseWriter, r *http.Request) {
					if r.Host != "example.com" || r.URL.RawQuery != "sig=a%2Fb&item=1&item=2" {
						t.Errorf("origin changed host/query: %q %q", r.Host, r.URL.RawQuery)
					}
					w.Header().Set("Content-Type", "image/png")
					_, _ = w.Write(pixel)
				}
				origin := httptest.NewUnstartedServer(http.HandlerFunc(serveMedia))
				var sni atomic.Value
				if originTLS {
					origin.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) { sni.Store(hello.ServerName); return nil, nil }}
					origin.StartTLS()
				} else {
					origin.Start()
				}
				defer origin.Close()
				roots := x509.NewCertPool()
				if originTLS {
					roots.AddCert(origin.Certificate())
				}
				proxyURL := ""
				var proxyHits atomic.Int32
				if proxyKind == "http" || proxyKind == "https" {
					p := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						proxyHits.Add(1)
						if r.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("proxyuser:proxypass")) {
							t.Error("proxy authentication missing")
						}
						if r.Method == http.MethodConnect {
							port := ":80"
							if originTLS {
								port = ":443"
							}
							if r.Host != publicMediaAddress+port {
								t.Errorf("CONNECT target was not pinned: %q", r.Host)
							}
							upstream, err := net.Dial("tcp", origin.Listener.Addr().String())
							if err != nil {
								t.Error(err)
								return
							}
							defer upstream.Close()
							conn, _, err := w.(http.Hijacker).Hijack()
							if err != nil {
								t.Error(err)
								return
							}
							defer conn.Close()
							_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
							go func() { _, _ = io.Copy(upstream, conn) }()
							_, _ = io.Copy(conn, upstream)
							return
						}
						t.Errorf("expected CONNECT, got %s", r.Method)
						w.WriteHeader(http.StatusMethodNotAllowed)
					}))
					if proxyKind == "https" {
						p.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
							if hello.ServerName != "" {
								t.Errorf("IP proxy received origin SNI %q", hello.ServerName)
							}
							return nil, nil
						}}
						p.StartTLS()
						roots.AddCert(p.Certificate())
					} else {
						p.Start()
					}
					defer p.Close()
					u, _ := url.Parse(p.URL)
					u.User = url.UserPassword("proxyuser", "proxypass")
					proxyURL = u.String()
				} else if proxyKind == "socks5" || proxyKind == "socks5h" {
					proxyURL = mediaSOCKSFixture(t, origin.Listener.Addr().String(), &proxyHits, originTLS)
					proxyURL = strings.Replace(proxyURL, "socks5://", proxyKind+"://", 1)
				}
				dial := func(ctx context.Context, network, address string) (net.Conn, error) {
					if proxyKind == "direct" {
						if !strings.HasPrefix(address, publicMediaAddress+":") {
							t.Errorf("direct dial was not pinned: %q", address)
						}
						address = origin.Listener.Addr().String()
					}
					return (&net.Dialer{}).DialContext(ctx, network, address)
				}
				client := img.MediaClientForTest(publicLookup, dial, &tls.Config{RootCAs: roots})
				setMediaFixture(t, client, proxyURL)
				mediaURL := scheme + "://example.com/pixel.png?sig=a%2Fb&item=1&item=2"
				mime, data, err := img.GetImageFromUrl(mediaURL)
				if err != nil || mime != "image/png" || data != mediaPixel {
					t.Fatalf("download failed: mime=%q err=%v", mime, err)
				}
				if w, h, err := img.GetImageSizeFromUrl(mediaURL); err != nil || w != 1 || h != 1 {
					t.Fatalf("dimension lookup failed: %dx%d err=%v", w, h, err)
				}
				if originTLS && sni.Load() != "example.com" {
					t.Errorf("origin SNI = %v", sni.Load())
				}
				if proxyKind != "direct" && proxyHits.Load() != 2 {
					t.Errorf("proxy was bypassed: hits=%d", proxyHits.Load())
				}
			})
		}
	}
}

func TestMediaRedirectsAndDNSChanges(t *testing.T) {
	for _, code := range []int{301, 302, 303, 307, 308} {
		for _, target := range []string{"/ok", "http://127.0.0.1/forbidden", "http://blocked.example/forbidden", "/rebind"} {
			t.Run(http.StatusText(code)+target, func(t *testing.T) {
				var hits, lookups atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					if r.URL.Path == "/start" {
						w.Header().Set("Location", target)
						w.WriteHeader(code)
						return
					}
					w.Header().Set("Content-Type", "application/pdf")
					_, _ = io.WriteString(w, "%PDF-fixture")
				}))
				defer server.Close()
				lookup := func(_ context.Context, host string) ([]net.IPAddr, error) {
					count := lookups.Add(1)
					if host == "blocked.example" || (target == "/rebind" && count > 1) {
						return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
					}
					return publicLookup(context.Background(), host)
				}
				client := img.MediaClientForTest(lookup, func(ctx context.Context, network, address string) (net.Conn, error) {
					if address != publicMediaAddress+":80" {
						t.Errorf("unpinned connection %q", address)
					}
					return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
				}, nil)
				setMediaFixture(t, client, "")
				mime, data, err := img.GetImageFromUrl("http://example.com/start")
				if target == "/ok" {
					if err != nil || mime != "application/pdf" || data != base64.StdEncoding.EncodeToString([]byte("%PDF-fixture")) || hits.Load() != 2 {
						t.Fatalf("legitimate PDF redirect changed: mime=%q err=%v hits=%d", mime, err, hits.Load())
					}
				} else if err == nil || hits.Load() != 1 {
					t.Fatalf("unsafe redirect accepted: err=%v hits=%d", err, hits.Load())
				}
			})
		}
	}
}

func TestMediaSizeTimeoutAndSafeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("x", 128))
	}))
	defer server.Close()
	client := img.MediaClientForTest(publicLookup, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}, nil)
	setMediaFixture(t, client, "")
	restore := img.SetMediaLimitForTest(64)
	t.Cleanup(restore)
	response, err := img.RequestFile("http://example.com/large", "base64")
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err == nil {
		t.Fatal("oversized body was accepted")
	}
	client.Timeout = 30 * time.Millisecond
	start := time.Now()
	_, err = img.RequestFile("http://example.com/slow?credential=synthetic-private-value", "base64")
	if err == nil || time.Since(start) > time.Second || strings.Contains(err.Error(), "synthetic-private-value") {
		t.Fatalf("timeout/error boundary failed: %v", err)
	}
}
