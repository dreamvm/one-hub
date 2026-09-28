package image

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// Test-only access permits offline public-address fixtures without exposing a
// production configuration that can relax the media address policy.
var PublicMediaIPForTest = func(value string) bool {
	address, err := netip.ParseAddr(value)
	return err == nil && publicMediaIP(address)
}

func MediaClientForTest(lookup func(context.Context, string) ([]net.IPAddr, error), dial func(context.Context, string, string) (net.Conn, error), tlsConfig *tls.Config) *http.Client {
	return &http.Client{Transport: &mediaTransport{lookup: lookup, dial: dial, tlsConfig: tlsConfig}, Timeout: 15 * time.Second}
}

func SetMediaLimitForTest(limit int64) func() {
	old := maxFileSize
	maxFileSize = limit
	return func() { maxFileSize = old }
}

func SetWorkerClientForTest(client *http.Client) func() {
	old := workerImageClient
	workerImageClient = client
	return func() { workerImageClient = old }
}
