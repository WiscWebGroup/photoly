package storage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TreeEntry separates stable resource identity from a content version (ETag).
type TreeEntry struct {
	Path      string
	Directory bool
	Size      int64
	Modified  int64
	Identity  string
	Version   string
}
type TreeStore interface {
	PutVersion(context.Context, string, io.Reader, string) error
	CopyTo(context.Context, string, io.Writer, TreeEntry) error
	Walk(context.Context, string, func(TreeEntry) error) error
	origin() string
	Store
	List(context.Context, string) ([]TreeEntry, error)
	Property(context.Context, string) (TreeEntry, error)
	Mkdir(context.Context, string) error
	Move(context.Context, string, string, string) error
}
type DAV struct {
	base               *url.URL
	username, password string
	client             *http.Client
}

func init() {
	Register(Driver{Type: "webdav", Label: "WebDAV", Fields: []Field{{Name: "url", Label: "Directory URL", Required: true}, {Name: "username", Label: "Username", Secret: true}, {Name: "password", Label: "Password", Secret: true}}, New: func(v map[string]string) (Store, error) { return NewDAV(v) }})
}
func NewDAV(values map[string]string) (*DAV, error) {
	u, e := url.Parse(values["url"])
	if e != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid_webdav_url")
	}
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		u.Host = u.Hostname()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/"
	u.RawPath = ""
	if _, e = remoteParts(strings.Trim(u.Path, "/"), true); e != nil {
		return nil, e
	}
	d := &DAV{base: u, username: values["username"], password: values["password"]}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca := os.Getenv("STORAGE_WEBDAV_CA_FILE"); ca != "" {
		b, e := os.ReadFile(ca)
		if e != nil {
			return nil, errors.New("webdav_ca_unavailable")
		}
		pool, e := x509.SystemCertPool()
		if e != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(b) {
			return nil, errors.New("invalid_webdav_ca")
		}
		tlsConfig.RootCAs = pool
	}
	tr := &http.Transport{Proxy: nil, TLSClientConfig: tlsConfig, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second, DisableKeepAlives: true, DialContext: d.dial}
	d.client = &http.Client{Transport: tr, Timeout: 5 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return d, nil
}
func (d *DAV) origin() string { return d.base.Scheme + "://" + d.base.Host }
func (d *DAV) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	allowed := false
	for _, v := range strings.Split(os.Getenv("STORAGE_WEBDAV_ALLOWED_HOSTS"), ",") {
		if strings.EqualFold(strings.TrimSpace(v), d.origin()) {
			allowed = true
		}
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil {
		return nil, errors.New("webdav_dns_failed")
	}
	if len(ips) == 0 {
		return nil, errors.New("webdav_dns_failed")
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		// Metadata/link-local/multicast/unspecified are never allowed, including via DNS.
		blocked := (!ip.IsGlobalUnicast() && !ip.IsLoopback()) || ip.String() == "168.63.129.16" || ip.String() == "fd00:ec2::254" || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip) || netip.MustParsePrefix("192.0.0.0/24").Contains(ip) || netip.MustParsePrefix("198.18.0.0/15").Contains(ip)
		if blocked || (ip.IsPrivate() || ip.IsLoopback()) && !allowed {
			return nil, errors.New("webdav_address_not_allowed")
		}
	}
	var last error
	for _, ip := range ips {
		c, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if e == nil {
			return c, nil
		}
		last = e
	}
	return nil, last
}
func remoteParts(p string, internal bool) ([]string, error) {
	if p == "" || p == "." {
		return nil, nil
	}
	if strings.HasPrefix(p, "/") {
		return nil, errors.New("invalid_remote_path")
	}
	parts := strings.Split(strings.TrimSuffix(p, "/"), "/")
	for _, part := range parts {
		if !ValidComponent(part) || !internal && part == SystemDirectory {
			return nil, errors.New("invalid_remote_path")
		}
		// Never let a second decoder turn a literal encoded segment into traversal.
		decoded, e := url.PathUnescape(part)
		if e == nil && (decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\\x00")) {
			return nil, errors.New("invalid_remote_path")
		}
	}
	return parts, nil
}
func (d *DAV) address(key string) (*url.URL, error) {
	parts, e := remoteParts(key, true)
	if e != nil {
		return nil, e
	}
	u := *d.base
	u.Path = d.base.Path + strings.Join(parts, "/")
	u.RawPath = ""
	return &u, nil
}
func (d *DAV) request(ctx context.Context, method, key string, body io.Reader, headers map[string]string) (*http.Response, error) {
	u, e := d.address(key)
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, method, u.String(), body)
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(d.username, d.password)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, e := d.client.Do(req)
	if e != nil {
		return nil, errors.New("webdav_connection_failed")
	}
	// Redirects are refused: credentials are never forwarded and a collection URL
	// must be canonical. This also prevents redirect-based SSRF.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		resp.Body.Close()
		return nil, errors.New("webdav_redirect_refused_use_canonical_url")
	}
	if resp.StatusCode == 404 {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode == 409 || resp.StatusCode == 412 || resp.StatusCode == 423 {
		resp.Body.Close()
		return nil, errors.New("webdav_conflict_rescan_required")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("webdav_request_failed_%d", resp.StatusCode)
	}
	return resp, nil
}
func (d *DAV) relative(href string) (string, error) {
	u, e := url.Parse(href)
	if e != nil {
		return "", errors.New("invalid_webdav_href")
	}
	u = d.base.ResolveReference(u)
	if !strings.EqualFold(u.Scheme, d.base.Scheme) || !strings.EqualFold(u.Host, d.base.Host) || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("webdav_href_outside_root")
	}
	root := strings.TrimSuffix(d.base.Path, "/")
	if u.Path == root || u.Path == root+"/" {
		return "", nil
	}
	if !strings.HasPrefix(u.Path, root+"/") {
		return "", errors.New("webdav_href_outside_root")
	}
	rel := strings.TrimSuffix(strings.TrimPrefix(u.Path, root+"/"), "/")
	if _, e = remoteParts(rel, true); e != nil {
		return "", e
	}
	return rel, nil
}

type davProp struct {
	Type struct {
		Collection *struct{} `xml:"DAV: collection"`
	} `xml:"DAV: resourcetype"`
	Size     int64  `xml:"DAV: getcontentlength"`
	Modified string `xml:"DAV: getlastmodified"`
	ETag     string `xml:"DAV: getetag"`
	ID       struct {
		Href string `xml:"DAV: href"`
	} `xml:"DAV: resource-id"`
}
type davResponse struct {
	Href   string `xml:"DAV: href"`
	Status string `xml:"DAV: status"`
	Props  []struct {
		Status string  `xml:"DAV: status"`
		Prop   davProp `xml:"DAV: prop"`
	} `xml:"DAV: propstat"`
}

func statusCode(s string) int {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}
func (d *DAV) properties(ctx context.Context, key, depth string) ([]TreeEntry, error) {
	body := `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/><d:getetag/><d:resource-id/></d:prop></d:propfind>`
	resp, e := d.request(ctx, "PROPFIND", key, strings.NewReader(body), map[string]string{"Depth": depth, "Content-Type": "application/xml; charset=utf-8"})
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return nil, errors.New("webdav_invalid_multistatus")
	}
	var result struct {
		Responses []davResponse `xml:"DAV: response"`
	}
	if e = xml.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&result); e != nil {
		return nil, errors.New("webdav_invalid_xml")
	}
	entries := []TreeEntry{}
	seen := map[string]bool{}
	for _, r := range result.Responses {
		p, e := d.relative(r.Href)
		if e != nil {
			return nil, e
		}
		if p != strings.Trim(key, "/") && (depth == "0" || path.Dir(p) != strings.Trim(key, "/") && !(key == "" && path.Dir(p) == ".")) {
			return nil, errors.New("webdav_invalid_depth_response")
		}
		if seen[p] {
			return nil, errors.New("webdav_duplicate_href")
		}
		seen[p] = true
		if r.Status != "" && (statusCode(r.Status) < 200 || statusCode(r.Status) >= 300) {
			return nil, errors.New("webdav_partial_listing")
		}
		item := TreeEntry{Path: p}
		found := false
		for _, ps := range r.Props {
			code := statusCode(ps.Status)
			if code == 404 {
				continue
			}
			if code < 200 || code >= 300 {
				return nil, errors.New("webdav_partial_listing")
			}
			found = true
			if ps.Prop.Type.Collection != nil {
				item.Directory = true
			}
			if ps.Prop.Size != 0 {
				item.Size = ps.Prop.Size
			}
			if ps.Prop.ETag != "" {
				item.Version = ps.Prop.ETag
			}
			if ps.Prop.ID.Href != "" {
				item.Identity = ps.Prop.ID.Href
			}
			if t, e := http.ParseTime(ps.Prop.Modified); e == nil {
				item.Modified = t.UnixNano()
			}
		}
		if !found {
			return nil, errors.New("webdav_missing_properties")
		}
		entries = append(entries, item)
	}
	if !seen[strings.Trim(key, "/")] {
		return nil, errors.New("webdav_incomplete_listing")
	}
	return entries, nil
}
func (d *DAV) List(ctx context.Context, key string) ([]TreeEntry, error) {
	all, e := d.properties(ctx, key, "1")
	if e != nil {
		return nil, e
	}
	out := []TreeEntry{}
	for _, v := range all {
		if v.Path != strings.Trim(key, "/") {
			out = append(out, v)
		}
	}
	return out, nil
}
func (d *DAV) Property(ctx context.Context, key string) (TreeEntry, error) {
	all, e := d.properties(ctx, key, "0")
	if e != nil {
		return TreeEntry{}, e
	}
	if len(all) != 1 {
		return TreeEntry{}, errors.New("webdav_invalid_stat")
	}
	return all[0], nil
}
func (d *DAV) Stat(ctx context.Context, key string) (Info, error) {
	v, e := d.Property(ctx, key)
	return Info{Size: v.Size}, e
}
func (d *DAV) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	r, e := d.request(ctx, "GET", key, nil, nil)
	if e != nil {
		return nil, e
	}
	return r.Body, nil
}
func (d *DAV) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	return d.PutVersion(ctx, key, r, "")
}
func (d *DAV) PutVersion(ctx context.Context, key string, r io.Reader, version string) error {
	h := map[string]string{"If-None-Match": "*"}
	if version != "" {
		h = map[string]string{"If-Match": version}
	}
	resp, e := d.request(ctx, "PUT", key, r, h)
	if e != nil {
		return e
	}
	return resp.Body.Close()
}
func checkDAVMutation(r *http.Response) error {
	defer r.Body.Close()
	if r.StatusCode != 207 {
		return nil
	}
	var result struct {
		Responses []davResponse `xml:"DAV: response"`
	}
	if e := xml.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&result); e != nil {
		return e
	}
	// Even a successful subset cannot make a partially applied mutation successful.
	for _, v := range result.Responses {
		if n := statusCode(v.Status); n < 200 || n >= 300 {
			return errors.New("webdav_partial_mutation")
		}
		for _, p := range v.Props {
			if n := statusCode(p.Status); n < 200 || n >= 300 {
				return errors.New("webdav_partial_mutation")
			}
		}
	}
	return nil
}
func (d *DAV) Mkdir(ctx context.Context, key string) error {
	r, e := d.request(ctx, "MKCOL", key, nil, nil)
	if e != nil {
		return e
	}
	return checkDAVMutation(r)
}
func (d *DAV) Move(ctx context.Context, src, dst, version string) error {
	u, e := d.address(dst)
	if e != nil {
		return e
	}
	h := map[string]string{"Destination": u.String(), "Overwrite": "F"}
	if version != "" {
		h["If-Match"] = version
	}
	r, e := d.request(ctx, "MOVE", src, nil, h)
	if e != nil {
		return e
	}
	return checkDAVMutation(r)
}
func (d *DAV) Delete(ctx context.Context, key string) error {
	r, e := d.request(ctx, "DELETE", key, nil, nil)
	if errors.Is(e, ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	return checkDAVMutation(r)
}
func (d *DAV) DeletePrefix(ctx context.Context, key string) error { return d.Delete(ctx, key) }
func (d *DAV) Check(ctx context.Context) error                    { return checkTree(ctx, d) }
func checkTree(ctx context.Context, d TreeStore) error {
	root, e := d.Property(ctx, "")
	if e != nil {
		return e
	}
	if !root.Directory {
		return errors.New("webdav_root_not_directory")
	}
	if _, e = d.List(ctx, ""); e != nil {
		return e
	}
	probe := ".photoly-check-" + uuid.NewString()
	if e = d.Mkdir(ctx, probe); e != nil {
		return e
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = d.Delete(cleanup, probe)
	}()
	if e = d.Put(ctx, probe+"/test", strings.NewReader("photoly"), 7); e != nil {
		return e
	}
	r, e := d.Get(ctx, probe+"/test")
	if e != nil {
		return e
	}
	b, e := io.ReadAll(io.LimitReader(r, 16))
	r.Close()
	if e != nil || string(b) != "photoly" {
		return errors.New("webdav_read_check_failed")
	}
	if e = d.Put(ctx, probe+"/test", strings.NewReader("must not overwrite"), 18); e == nil {
		return errors.New("webdav_conditional_writes_required")
	}
	info, e := d.Property(ctx, probe+"/test")
	if e != nil {
		return e
	}
	if info.Version != "" {
		if e = d.PutVersion(ctx, probe+"/test", strings.NewReader("stale"), `"photoly-stale-version"`); e == nil {
			return errors.New("webdav_conditional_writes_required")
		}
	}
	if e = d.Move(ctx, probe+"/test", probe+"/renamed", ""); e != nil {
		return e
	}
	if e = d.Delete(ctx, probe+"/renamed"); e != nil {
		return e
	}
	return d.Delete(ctx, probe)
}

// Walk never follows resource aliases/cycles, and excludes reserved workspaces.
func (d *DAV) Walk(ctx context.Context, root string, visit func(TreeEntry) error) error {
	return walkTree(ctx, d, root, visit)
}
func walkTree(ctx context.Context, d TreeStore, root string, visit func(TreeEntry) error) error {
	seen := map[string]bool{}
	identities := map[string]bool{}
	var walk func(string) error
	walk = func(key string) error {
		if seen[key] {
			return errors.New("webdav_directory_cycle")
		}
		seen[key] = true
		if len(seen) > 1000000 {
			return errors.New("webdav_tree_too_large")
		}
		entries, e := d.List(ctx, key)
		if e != nil {
			return e
		}
		for _, v := range entries {
			if path.Base(v.Path) == SystemDirectory {
				continue
			}
			if _, e = remoteParts(v.Path, false); e != nil {
				return e
			}
			if v.Directory && v.Identity != "" {
				if identities[v.Identity] {
					return errors.New("webdav_directory_cycle")
				}
				identities[v.Identity] = true
			}
			if e = visit(v); e != nil {
				if v.Directory && errors.Is(e, filepath.SkipDir) {
					continue
				}
				return e
			}
			if v.Directory {
				if e = walk(v.Path); e != nil {
					return e
				}
			}
		}
		return nil
	}
	return walk(root)
}
