package photo

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// allowPrivate lets tests download from a loopback server; never set in production.
var allowPrivate bool

var nonGlobal = mustPrefixes("0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
	"192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
	"240.0.0.0/4", "255.255.255.255/32", "::/128", "::1/128", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32",
	"2002::/16", "fc00::/7", "fe80::/10")

func mustPrefixes(v ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(v))
	for i, s := range v {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// isGlobal is ipaddress.is_global plus the multicast/6to4/teredo/mapped exclusions.
func isGlobal(ip netip.Addr) bool {
	if ip.Is4In6() || ip.IsMulticast() || ip.IsUnspecified() || ip.Zone() != "" {
		return false
	}
	for _, p := range nonGlobal {
		if p.Contains(ip) {
			// 2001::/23 contains the globally reachable 2001:1::1 etc. exceptions; none are public web hosts.
			return false
		}
	}
	return true
}

// publicTarget is remote_image.public_target: validate the URL and pick one resolved public address.
func (s *Service) publicTarget(ctx context.Context, raw string) (*url.URL, string, string, netip.Addr, error) {
	var zero netip.Addr
	if len(raw) > 8192 || strings.ContainsRune(raw, '\\') || strings.IndexFunc(raw, func(r rune) bool { return r < 33 }) >= 0 {
		return nil, "", "", zero, Err("invalid_image_url", 400)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, "", "", zero, Err("invalid_image_url", 400)
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return nil, "", "", zero, Err("non_public_image_url", 400)
	}
	host := u.Hostname()
	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{ip}
	} else {
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, "", "", zero, Err("non_public_image_url", 400)
		}
		addrs = ips
	}
	for _, a := range addrs {
		if !allowPrivate && !isGlobal(a) {
			return nil, "", "", zero, Err("non_public_image_url", 400)
		}
	}
	return u, host, port, addrs[0], nil
}

// Download is remote_image.download_image: a bounded fetch with DNS validation and a pinned address.
func (s *Service) Download(ctx context.Context, raw, dest string) error {
	total := time.Duration(s.Cfg.OnlineDownloadTimeout * float64(time.Second))
	ctx, cancel := context.WithTimeout(ctx, total)
	defer cancel()
	limit := s.Cfg.OnlineDownloadMaxBytes
	timedOut := func(err error) bool {
		var ne net.Error
		return errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout())
	}
	for hop := 0; hop < 4; hop++ {
		u, host, port, ip, err := s.publicTarget(ctx, raw)
		if err != nil {
			return err
		}
		connect := time.Duration(s.Cfg.OnlineDownloadConnectTimeout * float64(time.Second))
		pinned := net.JoinHostPort(ip.String(), port)
		tr := &http.Transport{
			Proxy:                 nil,
			DisableCompression:    true,
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 5 * time.Second,
			DialContext: func(c context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: connect}).DialContext(c, network, pinned)
			},
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		req.Header.Set("Accept", "image/*")
		req.Header.Set("Accept-Encoding", "identity")
		client := &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			tr.CloseIdleConnections()
			if timedOut(err) {
				return Err("download_timeout", 408)
			}
			return Err("download_failed", 400)
		}
		switch resp.StatusCode {
		case 301, 302, 303, 307, 308:
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			tr.CloseIdleConnections()
			if hop == 3 || loc == "" {
				return Err("too_many_redirects", 400)
			}
			next, err := u.Parse(loc)
			if err != nil {
				return Err("invalid_image_url", 400)
			}
			raw = next.String()
			continue
		}
		err = func() error {
			defer resp.Body.Close()
			defer tr.CloseIdleConnections()
			if resp.StatusCode != 200 {
				return Err("download_http_error", 400)
			}
			if l := resp.Header.Get("Content-Length"); l != "" {
				n, err := strconv.ParseInt(l, 10, 64)
				if err != nil || n > limit {
					return Err("download_too_large", 413)
				}
			}
			if ce := strings.ToLower(resp.Header.Get("Content-Encoding")); ce != "" && ce != "identity" {
				return Err("unsupported_content_encoding", 400)
			}
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()
			buf := make([]byte, 64*1024)
			var size int64
			for {
				n, rerr := resp.Body.Read(buf)
				if n > 0 {
					size += int64(n)
					if size > limit {
						return Err("download_too_large", 413)
					}
					if DiskFree(filepath.Dir(dest)) < uint64(n)+uint64(s.Cfg.UploadDiskReserveBytes) {
						return Err("insufficient_disk_space", 507)
					}
					if _, werr := out.Write(buf[:n]); werr != nil {
						return werr
					}
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					if timedOut(rerr) || ctx.Err() != nil {
						return Err("download_timeout", 408)
					}
					return Err("download_failed", 400)
				}
			}
			if ctx.Err() != nil {
				return Err("download_timeout", 408)
			}
			return nil
		}()
		_ = host
		return err
	}
	return Err("too_many_redirects", 400)
}
