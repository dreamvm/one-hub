package image_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"one-api/common/config"
	img "one-api/common/image"
)

func TestMediaRawPublicContractAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "raw fixture") }))
	defer server.Close()
	client := img.MediaClientForTest(publicLookup, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}, nil)
	setMediaFixture(t, client, "")
	config.CFWorkerImageUrl = "http://worker.invalid/unexpected"
	response, err := img.RequestPublicFile(context.Background(), "https://127.0.0.1/private")
	if err == nil {
		response.Body.Close()
		t.Fatal("raw wrapper accepted restricted target")
	}
	response, err = img.RequestPublicFile(context.Background(), "http://example.com/image")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(data) != "raw fixture" {
		t.Fatalf("raw contract failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err = img.RequestPublicFile(ctx, "http://example.com/image")
	if err == nil {
		response.Body.Close()
		t.Fatal("request cancellation was lost")
	}
}

func TestMediaRawEnvironmentProxyPolicy(t *testing.T) {
	for _, mode := range []string{"environment", "explicit", "no proxy"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HTTPS_PROXY", "http://environment-proxy.invalid:8080")
			t.Setenv("NO_PROXY", "")
			t.Setenv("no_proxy", "")
			explicit, expected := "", "environment-proxy.invalid:8080"
			if mode == "explicit" {
				explicit = "http://configured-proxy.invalid:8080"
				expected = "configured-proxy.invalid:8080"
			}
			if mode == "no proxy" {
				t.Setenv("NO_PROXY", "example.com")
				expected = publicMediaAddress + ":443"
			}
			var dialed string
			client := img.MediaClientForTest(publicLookup, func(_ context.Context, _, address string) (net.Conn, error) {
				dialed = address
				return nil, errors.New("fixture stops before network")
			}, nil)
			setMediaFixture(t, client, explicit)
			response, err := img.RequestPublicFile(context.Background(), "https://example.com/image")
			if err == nil {
				response.Body.Close()
				t.Fatal("fixture did not stop")
			}
			if dialed != expected {
				t.Errorf("selected destination %q, expected %q", dialed, expected)
			}
		})
	}
}

func TestMediaRawEnvironmentProxyReevaluatedOnRedirect(t *testing.T) {
	for _, tc := range []struct{ name, start, target, noProxy, expected string }{
		{"scheme changes", "http://first.example/start", "https://second.example/final", "", "https-proxy.invalid:8443"},
		{"leave no proxy", "http://first.example/start", "http://second.example/final", "first.example", "http-proxy.invalid:8080"},
		{"enter no proxy", "http://first.example/start", "http://second.example/final", "second.example", publicMediaAddress + ":80"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HTTP_PROXY", "http://http-proxy.invalid:8080")
			t.Setenv("HTTPS_PROXY", "http://https-proxy.invalid:8443")
			t.Setenv("NO_PROXY", tc.noProxy)
			t.Setenv("no_proxy", "")
			t.Setenv("REQUEST_METHOD", "")
			var dialed string
			client := img.MediaClientForTest(publicLookup, func(_ context.Context, _, address string) (net.Conn, error) {
				dialed = address
				return nil, errors.New("fixture stops before network")
			}, nil)
			checked := client.Transport
			client.Transport = mediaRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/start" {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{tc.target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				return checked.RoundTrip(r)
			})
			setMediaFixture(t, client, "")
			response, err := img.RequestPublicFile(context.Background(), tc.start)
			if err == nil {
				response.Body.Close()
				t.Fatal("fixture did not stop")
			}
			if dialed != tc.expected {
				t.Fatalf("redirect dial %q, expected %q", dialed, tc.expected)
			}
		})
	}
}
