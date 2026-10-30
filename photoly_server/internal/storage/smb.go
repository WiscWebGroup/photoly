package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path"
	"strings"
	"time"

	smb "github.com/cloudsoda/go-smb2"
	"github.com/google/uuid"
)

type SMB struct{ host, share, directory, username, password, domain string }

func init() {
	Register(Driver{Type: "smb", Label: "SMB", Fields: []Field{{Name: "server", Required: true}, {Name: "share", Required: true}, {Name: "subdirectory"}, {Name: "username", Secret: true}, {Name: "password", Secret: true}, {Name: "domain", Secret: true}}, New: func(v map[string]string) (Store, error) { return NewSMB(v) }})
}
func NewSMB(v map[string]string) (*SMB, error) {
	host := strings.ToLower(strings.TrimSpace(v["server"]))
	if host == "" || strings.ContainsAny(host, "/\\@?#\x00 ") {
		return nil, errors.New("invalid_smb_server")
	}
	if ip, e := netip.ParseAddr(strings.Trim(host, "[]")); e == nil {
		host = ip.String()
	} else if strings.Contains(host, ":") {
		return nil, errors.New("invalid_smb_server")
	}
	share := v["share"]
	if !ValidComponent(share) || strings.ContainsAny(share, ":*?\"<>|") {
		return nil, errors.New("invalid_smb_share")
	}
	directory := strings.ReplaceAll(v["subdirectory"], "\\", "/")
	parts, e := smbParts(directory, false)
	if e != nil {
		return nil, e
	}
	return &SMB{host: host, share: share, directory: strings.Join(parts, "/"), username: v["username"], password: v["password"], domain: v["domain"]}, nil
}
func smbParts(key string, internal bool) ([]string, error) {
	parts, e := remoteParts(key, internal)
	if e != nil {
		return nil, e
	}
	for _, p := range parts {
		if strings.ContainsAny(p, ":*?\"<>|") || strings.TrimRight(p, " .") != p {
			return nil, errors.New("invalid_smb_name")
		}
	}
	return parts, nil
}
func (d *SMB) origin() string { return "smb://" + strings.ToLower(d.host) }
func (d *SMB) key(key string) (string, error) {
	parts, e := smbParts(key, true)
	if e != nil {
		return "", e
	}
	return strings.Trim(strings.Join(append([]string{d.directory}, parts...), "/"), "/"), nil
}

type smbConnection struct {
	share   *smb.Share
	session *smb.Session
	conn    net.Conn
	guards  []*smb.File
}

func (c *smbConnection) close() {
	for i := len(c.guards) - 1; i >= 0; i-- {
		_ = c.guards[i].Close()
	}
	_ = c.share.Umount()
	_ = c.session.Logoff()
	_ = c.conn.Close()
}
func (d *SMB) connect(ctx context.Context) (*smbConnection, error) {
	allowed := false
	for _, v := range strings.Split(os.Getenv("STORAGE_SMB_ALLOWED_HOSTS"), ",") {
		if strings.EqualFold(strings.TrimSpace(v), d.host) {
			allowed = true
		}
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", d.host)
	if e != nil || len(ips) == 0 {
		return nil, errors.New("smb_dns_failed")
	}
	for _, ip := range ips {
		ip = ip.Unmap()
		if (!ip.IsGlobalUnicast() && !ip.IsLoopback()) || ip.IsLinkLocalUnicast() || ip.String() == "168.63.129.16" || ip.String() == "fd00:ec2::254" || netip.MustParsePrefix("100.64.0.0/10").Contains(ip) || netip.MustParsePrefix("198.18.0.0/15").Contains(ip) || netip.MustParsePrefix("192.0.0.0/24").Contains(ip) || (ip.IsPrivate() || ip.IsLoopback()) && !allowed {
			return nil, errors.New("smb_address_not_allowed")
		}
	}
	var conn net.Conn
	for _, ip := range ips {
		conn, e = (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), "445"))
		if e == nil {
			break
		}
	}
	if e != nil {
		return nil, errors.New("smb_connection_failed")
	}
	deadline := time.Now().Add(5 * time.Minute)
	if v, ok := ctx.Deadline(); ok && v.Before(deadline) {
		deadline = v
	}
	_ = conn.SetDeadline(deadline)
	dial := smb.Dialer{Negotiator: smb.Negotiator{RequireMessageSigning: true}, Initiator: &smb.NTLMInitiator{User: d.username, Password: d.password, Domain: d.domain}}
	session, e := dial.DialConn(ctx, conn, net.JoinHostPort(d.host, "445"))
	if e != nil {
		conn.Close()
		return nil, errors.New("smb_authentication_or_negotiation_failed")
	}
	session = session.WithContext(ctx)
	share, e := session.Mount(d.share)
	if e != nil {
		session.Logoff()
		conn.Close()
		return nil, errors.New("smb_share_unavailable")
	}
	return &smbConnection{share: share.WithContext(ctx), session: session, conn: conn}, nil
}

// Pin every ancestor against replacement while resolving the requested path.
func (c *smbConnection) guard(key string) error {
	parents := []string{""}
	parts := strings.Split(key, "/")
	for i := 1; i < len(parts); i++ {
		parents = append(parents, strings.Join(parts[:i], "/"))
	}
	for _, p := range parents {
		found := false
		for _, g := range c.guards {
			if strings.ReplaceAll(g.Name(), "\\", "/") == p {
				found = true
				break
			}
		}
		if found {
			continue
		}
		f, e := c.share.OpenGuarded(p, "guard")
		if e != nil {
			return smbError(e)
		}
		c.guards = append(c.guards, f)
	}
	return nil
}
func smbError(e error) error {
	if e == nil {
		return nil
	}
	if os.IsNotExist(e) {
		return ErrNotFound
	}
	if os.IsExist(e) {
		return errors.New("smb_conflict_rescan_required")
	}
	return fmt.Errorf("smb_operation_failed: %w", e)
}
func smbEntry(key string, info os.FileInfo) TreeEntry {
	v := TreeEntry{Path: key, Directory: info.IsDir(), Size: info.Size(), Modified: info.ModTime().UnixNano()}
	if st, ok := info.(*smb.FileStat); ok {
		if st.FileId != 0 {
			v.Identity = fmt.Sprintf("%x", st.FileId)
		}
		v.Version = fmt.Sprintf("%s:%d:%d:%d", v.Identity, v.Size, v.Modified, st.ChangeTime.UnixNano())
	}
	return v
}
func (d *SMB) open(ctx context.Context, key, mode string) (*smbConnection, *smb.File, error) {
	k, e := d.key(key)
	if e != nil {
		return nil, nil, e
	}
	c, e := d.connect(ctx)
	if e != nil {
		return nil, nil, e
	}
	if e = c.guard(k); e != nil {
		c.close()
		return nil, nil, e
	}
	f, e := c.share.OpenGuarded(k, mode)
	if e != nil {
		c.close()
		return nil, nil, smbError(e)
	}
	return c, f, nil
}
func (d *SMB) Property(ctx context.Context, key string) (TreeEntry, error) {
	c, f, e := d.open(ctx, key, "read")
	if e != nil {
		return TreeEntry{}, e
	}
	defer c.close()
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return TreeEntry{}, smbError(e)
	}
	return smbEntry(key, st), nil
}
func (d *SMB) List(ctx context.Context, key string) ([]TreeEntry, error) {
	c, f, e := d.open(ctx, key, "read")
	if e != nil {
		return nil, e
	}
	defer c.close()
	defer f.Close()
	entries, e := f.Readdir(-1)
	if e != nil {
		return nil, smbError(e)
	}
	out := []TreeEntry{}
	for _, st := range entries {
		if st.Name() == "." || st.Name() == ".." || st.Name() == SystemDirectory {
			continue
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("smb_reparse_point_refused")
		}
		p := path.Join(key, st.Name())
		if _, e = smbParts(p, false); e != nil {
			return nil, e
		}
		// QUERY_INFO supplies a stable file index and change time, unlike directory entries.
		child, e := c.share.OpenGuarded(path.Join(d.directory, p), "read")
		if e != nil {
			return nil, smbError(e)
		}
		info, e := child.Stat()
		child.Close()
		if e != nil {
			return nil, smbError(e)
		}
		out = append(out, smbEntry(p, info))
	}
	return out, nil
}

type smbReader struct {
	*smb.File
	c *smbConnection
}

func (r *smbReader) Close() error { e := r.File.Close(); r.c.close(); return e }
func (d *SMB) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	c, f, e := d.open(ctx, key, "read")
	if e != nil {
		return nil, e
	}
	return &smbReader{f, c}, nil
}
func (d *SMB) Stat(ctx context.Context, key string) (Info, error) {
	v, e := d.Property(ctx, key)
	return Info{Size: v.Size}, e
}
func (d *SMB) Put(ctx context.Context, key string, r io.Reader, _ int64) error {
	return d.PutVersion(ctx, key, r, "")
}
func (d *SMB) PutVersion(ctx context.Context, key string, r io.Reader, version string) error {
	mode := "write"
	destination := key
	if version == "" {
		mode = "create"
		if _, e := d.Property(ctx, SystemDirectory); errors.Is(e, ErrNotFound) {
			if e = d.Mkdir(ctx, SystemDirectory); e != nil {
				if _, check := d.Property(ctx, SystemDirectory); check != nil {
					return e
				}
			}
		} else if e != nil {
			return e
		}
		key = SystemDirectory + "/upload-" + uuid.NewString()
	}
	c, f, e := d.open(ctx, key, mode)
	if e != nil {
		return e
	}
	defer c.close()
	defer f.Close()
	if version != "" {
		st, e := f.Stat()
		if e != nil {
			return smbError(e)
		}
		if smbEntry(key, st).Version != version {
			return errors.New("smb_conflict_rescan_required")
		}
		if e = f.Truncate(0); e != nil {
			return smbError(e)
		}
	}
	_, e = io.Copy(f, r)
	if e == nil {
		e = f.Sync()
	}
	if e != nil && version == "" {
		_ = f.RemoveGuarded()
	}
	if e == nil && version == "" {
		var target string
		target, e = d.key(destination)
		if e == nil {
			e = c.guard(target)
		}
		if e == nil {
			e = f.RenameNoReplace(target)
		}
		if e != nil {
			_ = f.RemoveGuarded()
		}
	}
	return smbError(e)
}
func (d *SMB) Mkdir(ctx context.Context, key string) error {
	c, f, e := d.open(ctx, key, "directory")
	if e != nil {
		return e
	}
	defer c.close()
	return smbError(f.Close())
}
func (d *SMB) Move(ctx context.Context, src, dst, version string) error {
	c, f, e := d.open(ctx, src, "mutate")
	if e != nil {
		return e
	}
	defer c.close()
	defer f.Close()
	k, e := d.key(dst)
	if e != nil {
		return e
	}
	if e = c.guard(k); e != nil {
		return e
	}
	st, e := f.Stat()
	if e != nil {
		return smbError(e)
	}
	if version != "" && smbEntry(src, st).Version != version {
		return errors.New("smb_conflict_rescan_required")
	}
	return smbError(f.RenameNoReplace(k))
}
func (d *SMB) Delete(ctx context.Context, key string) error {
	if key == "" || key == "." {
		return errors.New("smb_root_protected")
	}
	c, f, e := d.open(ctx, key, "read")
	if errors.Is(e, ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	st, e := f.Stat()
	var entries []os.FileInfo
	if e == nil && st.IsDir() {
		entries, e = f.Readdir(-1)
	}
	before := TreeEntry{}
	if e == nil {
		before = smbEntry(key, st)
	}
	f.Close()
	c.close()
	if e != nil {
		return smbError(e)
	}
	for _, v := range entries {
		if v.Name() == "." || v.Name() == ".." {
			continue
		}
		if e = d.Delete(ctx, path.Join(key, v.Name())); e != nil {
			return e
		}
	}
	c, f, e = d.open(ctx, key, "mutate")
	if e != nil {
		return e
	}
	defer c.close()
	defer f.Close()
	st, e = f.Stat()
	if e != nil {
		return smbError(e)
	}
	if smbEntry(key, st).Identity != before.Identity {
		return errors.New("smb_conflict_rescan_required")
	}
	return smbError(f.RemoveGuarded())
}

func (d *SMB) DeletePrefix(ctx context.Context, key string) error { return d.Delete(ctx, key) }
func (d *SMB) Check(ctx context.Context) error                    { return checkTree(ctx, d) }
func (d *SMB) Walk(ctx context.Context, root string, visit func(TreeEntry) error) error {
	return walkTree(ctx, d, root, visit)
}
func (d *SMB) CopyTo(ctx context.Context, key string, out io.Writer, expected TreeEntry) error {
	return copyTreeTo(ctx, d, key, out, expected)
}
