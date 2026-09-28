package image

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/proxy"

	"one-api/common/utils"
)

var errMediaTarget = errors.New("media target must use a public HTTP(S) address")

var specialMediaRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicMediaIP(ip netip.Addr) bool {
	if !ip.IsValid() || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range specialMediaRanges {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

type mediaLookup func(context.Context, string) ([]net.IPAddr, error)
type mediaDial func(context.Context, string, string) (net.Conn, error)

// Match net/http's IDNA conversion before validation, DNS and TLS. ASCII
// hostnames retain the standard resolver's handling of case and trailing dots.
func mediaHostname(host string) (string, error) {
	for _, r := range host {
		if r >= 128 {
			ascii, err := idna.Lookup.ToASCII(host)
			if err != nil {
				return "", errMediaTarget
			}
			return ascii, nil
		}
	}
	return host, nil
}

func mediaAddresses(ctx context.Context, target *url.URL, lookup mediaLookup) ([]netip.Addr, error) {
	if target == nil || target.Opaque != "" || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		return nil, errMediaTarget
	}
	if port := target.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, errMediaTarget
		}
	}
	host, err := mediaHostname(target.Hostname())
	if err != nil || strings.Contains(host, "%") {
		return nil, errMediaTarget
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicMediaIP(ip) {
			return nil, errMediaTarget
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	addresses, err := lookup(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errMediaTarget
	}
	ips := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		ip, ok := netip.AddrFromSlice(address.IP)
		if !ok || address.Zone != "" || !publicMediaIP(ip) {
			return nil, errMediaTarget
		}
		ips = append(ips, ip.Unmap())
	}
	return ips, nil
}

// This transport is only for user-selected media. Administrator-configured
// provider, proxy and Worker endpoints have a different trust boundary.
type mediaTransport struct {
	lookup    mediaLookup
	dial      mediaDial
	tlsConfig *tls.Config
}

func newMediaTransport() *mediaTransport {
	return &mediaTransport{
		lookup: net.DefaultResolver.LookupIPAddr,
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: time.Duration(utils.GetOrDefault("connect_timeout", 5)) * time.Second}
			return dialer.DialContext(ctx, network, address)
		},
	}
}

func (m *mediaTransport) tlsFor(host string) *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if m.tlsConfig != nil {
		cfg = m.tlsConfig.Clone()
	}
	cfg.ServerName = host
	return cfg
}

func mediaProxy(ctx context.Context) (*url.URL, error) {
	address, _ := ctx.Value(utils.ProxySock5AddrKey).(string)
	if address == "" {
		address, _ = ctx.Value(utils.ProxyHTTPAddrKey).(string)
	}
	if address == "" {
		return nil, nil
	}
	p, err := url.Parse(address)
	if err != nil || p.Hostname() == "" || p.Fragment != "" {
		return nil, errors.New("invalid media proxy")
	}
	switch p.Scheme {
	case "http", "https", "socks5", "socks5h":
		host, err := mediaHostname(p.Hostname())
		if err != nil {
			return nil, errors.New("invalid media proxy hostname")
		}
		if host != p.Hostname() {
			if p.Port() != "" {
				host = net.JoinHostPort(host, p.Port())
			}
			p.Host = host
		}
		return p, nil
	default:
		return nil, errors.New("unsupported media proxy")
	}
}

func (m *mediaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ips, err := mediaAddresses(req.Context(), req.URL, m.lookup)
	if err != nil {
		return nil, err
	}
	p, err := mediaProxy(req.Context())
	if err != nil {
		return nil, err
	}
	host, err := mediaHostname(req.URL.Hostname())
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		pinned := req.Clone(req.Context())
		u := *req.URL
		pinned.URL = &u
		pinned.Host = req.Host
		if pinned.Host == "" {
			pinned.Host = req.URL.Host
		}
		port := req.URL.Port()
		if port == "" {
			port = "80"
			if req.URL.Scheme == "https" {
				port = "443"
			}
		}
		pinned.URL.Host = net.JoinHostPort(ip.String(), port)
		transport := &http.Transport{
			DisableKeepAlives:   true,
			TLSClientConfig:     m.tlsFor(host),
			TLSHandshakeTimeout: 15 * time.Second,
			DialContext: func(_ context.Context, network, address string) (net.Conn, error) {
				return m.dial(req.Context(), network, address)
			},
		}
		if p != nil && (p.Scheme == "socks5" || p.Scheme == "socks5h") {
			d, proxyErr := proxy.FromURL(p, &mediaProxyDialer{ctx: req.Context(), dial: m.dial})
			if proxyErr != nil {
				return nil, proxyErr
			}
			contextDialer, ok := d.(proxy.ContextDialer)
			if !ok {
				return nil, errors.New("media proxy does not support cancellation")
			}
			transport.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
				return contextDialer.DialContext(req.Context(), network, address)
			}
		} else if p != nil {
			transport.DialContext = func(_ context.Context, _, address string) (net.Conn, error) {
				return m.connectProxy(req.Context(), p, address)
			}
		}
		response, requestErr := transport.RoundTrip(pinned)
		err = requestErr
		transport.CloseIdleConnections()
		if err == nil {
			response.Request = req
			return response, nil
		}
		if req.Context().Err() != nil {
			break
		}
	}
	return nil, err
}

type mediaProxyDialer struct {
	ctx  context.Context
	dial mediaDial
}

func (d *mediaProxyDialer) Dial(network, address string) (net.Conn, error) {
	return d.dial(d.ctx, network, address)
}

func proxyAddress(p *url.URL) string {
	port := p.Port()
	if port == "" {
		port = "80"
		if p.Scheme == "https" {
			port = "443"
		}
	}
	return net.JoinHostPort(p.Hostname(), port)
}

func (m *mediaTransport) dialProxy(ctx context.Context, p *url.URL) (net.Conn, error) {
	conn, err := m.dial(ctx, "tcp", proxyAddress(p))
	if err != nil || p.Scheme != "https" {
		return conn, err
	}
	tlsConn := tls.Client(conn, m.tlsFor(p.Hostname()))
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

// Both HTTP and HTTPS media use a CONNECT tunnel to the checked numeric IP.
// Forward proxy requests may resolve the original host again at the proxy.
func (m *mediaTransport) connectProxy(ctx context.Context, p *url.URL, address string) (_ net.Conn, err error) {
	conn, err := m.dialProxy(ctx, p)
	if err != nil {
		return nil, err
	}
	tunnel := &mediaTunnelConn{Conn: conn, stop: context.AfterFunc(ctx, func() { _ = conn.Close() })}
	defer func() {
		if err != nil {
			_ = tunnel.Close()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address, Header: make(http.Header)}
	if p.User != nil {
		password, _ := p.User.Password()
		request.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(p.User.Username()+":"+password)))
	}
	if err = request.Write(conn); err != nil {
		return nil, err
	}
	limitedHeaders := &mediaHeaderReader{reader: conn, remaining: 1 << 20, limited: true}
	tunnel.reader = bufio.NewReader(limitedHeaders)
	for interim := 0; ; interim++ {
		var response *http.Response
		response, err = http.ReadResponse(tunnel.reader, request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode == http.StatusOK {
			break
		}
		if response.StatusCode >= 200 || response.StatusCode == http.StatusSwitchingProtocols || interim >= 5 {
			return nil, errors.New("media proxy CONNECT failed")
		}
	}
	limitedHeaders.limited = false
	return tunnel, nil
}

type mediaHeaderReader struct {
	reader    io.Reader
	remaining int
	limited   bool
}

func (r *mediaHeaderReader) Read(p []byte) (int, error) {
	if r.limited {
		if r.remaining <= 0 {
			return 0, errors.New("media proxy response headers too large")
		}
		if len(p) > r.remaining {
			p = p[:r.remaining]
		}
	}
	n, err := r.reader.Read(p)
	if r.limited {
		r.remaining -= n
	}
	return n, err
}

type mediaTunnelConn struct {
	net.Conn
	reader *bufio.Reader
	stop   func() bool
	once   sync.Once
}

func (c *mediaTunnelConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func (c *mediaTunnelConn) Close() error {
	var err error
	c.once.Do(func() { c.stop(); err = c.Conn.Close() })
	return err
}
