package image_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	img "one-api/common/image"
)

func TestMediaIDNAHostDNSAndTLS(t *testing.T) {
	const asciiHost = "xn--bcher-kva.example"
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{asciiHost}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	for _, secure := range []bool{false, true} {
		scheme := "http"
		if secure {
			scheme = "https"
		}
		t.Run(scheme, func(t *testing.T) {
			var sni atomic.Value
			origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Host != asciiHost {
					t.Errorf("Host is not IDNA: %q", r.Host)
				}
				_, _ = io.WriteString(w, "ok")
			}))
			if secure {
				origin.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) { sni.Store(hello.ServerName); return nil, nil }}
				origin.StartTLS()
			} else {
				origin.Start()
			}
			defer origin.Close()
			client := img.MediaClientForTest(func(ctx context.Context, host string) ([]net.IPAddr, error) {
				if host != asciiHost {
					return nil, errors.New("resolver requires ASCII domain")
				}
				return publicLookup(ctx, host)
			}, func(ctx context.Context, network, address string) (net.Conn, error) {
				if address != publicMediaAddress+map[bool]string{false: ":80", true: ":443"}[secure] {
					t.Errorf("unpinned IDNA dial: %q", address)
				}
				return (&net.Dialer{}).DialContext(ctx, network, origin.Listener.Addr().String())
			}, &tls.Config{RootCAs: roots})
			setMediaFixture(t, client, "")
			for _, host := range []string{asciiHost, "bücher.example"} {
				response, err := img.RequestFile(scheme+"://"+host+"/image", "base64")
				if err != nil {
					t.Errorf("IDNA download %q failed: %v", host, err)
					continue
				}
				_, _ = io.ReadAll(response.Body)
				response.Body.Close()
			}
			if secure && sni.Load() != asciiHost {
				t.Errorf("SNI is not IDNA: %v", sni.Load())
			}
		})
	}
}
