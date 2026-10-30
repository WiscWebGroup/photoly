package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	dbgen "github.com/WiscWebGroup/photoly/photoly_server/internal/db/gen"
	"github.com/jackc/pgx/v5"
)

// Source identifies a user's durable storage, independently of the current request.
type Source struct {
	ID      int64
	UserID  int64
	Kind    string
	Root    string
	Enabled bool
	Config  []byte
	Secret  []byte
}

func GetSource(ctx context.Context, q dbgen.DBTX, id int64) (Source, error) {
	var s Source
	err := q.QueryRow(ctx, `SELECT id,user_id,kind,root_path,enabled,config,secret FROM storage_source WHERE id=$1`, id).Scan(&s.ID, &s.UserID, &s.Kind, &s.Root, &s.Enabled, &s.Config, &s.Secret)
	if err == nil && !s.Enabled && ctx.Value(inactiveKey{}) != true {
		err = errors.New("storage_source_disabled")
	}
	return s, err
}

func ValidComponent(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00") && len(s) <= 255
}

func canonicalRoot(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, e := filepath.EvalSymlinks(root); e == nil {
		root = resolved
	}
	return root, nil
}

// SafePath refuses links in every existing component, including the leaf. The
// configured upload root itself may be a deployment symlink.
func SafePath(root string, parts ...string) (string, error) {
	root, err := canonicalRoot(root)
	if err != nil {
		return "", err
	}
	p := root
	for _, part := range parts {
		if !ValidComponent(part) {
			return "", errors.New("invalid_file_name")
		}
		p = filepath.Join(p, part)
		fi, e := os.Lstat(p)
		if e == nil && fi.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic_links_not_supported")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return p, nil
}

func SourceRoot(root string, s Source) (string, error) {
	if filepath.IsAbs(s.Root) {
		return "", errors.New("external_storage_not_supported")
	}
	return SafePath(root, s.Root)
}

// Folder resolves a tree by IDs. A mount stops traversal even though its UI parent
// is the user's root. No descendant stores an absolute path.
func Folder(ctx context.Context, q dbgen.DBTX, root string, id int64) (string, Source, error) {
	requestedID := id
	canonical, e := canonicalRoot(root)
	if e != nil {
		return "", Source{}, e
	}
	root = canonical
	var parts []string
	seen := map[int64]bool{}
	var source Source
	for {
		if seen[id] {
			return "", source, errors.New("folder_cycle")
		}
		seen[id] = true
		var name string
		var parent, sid int64
		var mount bool
		if err := q.QueryRow(ctx, `SELECT ns_name,ns_parent_id,source_id,is_mount FROM namespace WHERE ns_id=$1`, id).Scan(&name, &parent, &sid, &mount); err != nil {
			return "", source, err
		}
		if source.ID == 0 {
			s, err := GetSource(ctx, q, sid)
			if err != nil {
				return "", s, err
			}
			source = s
		}
		if source.ID != sid {
			return "", source, errors.New("cross_source_move")
		}
		if parent == -1 || mount {
			break
		}
		parts = append(parts, name)
		id = parent
	}
	if source.Kind != "local" {
		return "", source, nil
	}
	base, err := SourceRoot(root, source)
	if err != nil {
		return "", source, err
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	// Validate the source directory too, rather than resolving a user-created link.
	rel, err := filepath.Rel(root, base)
	if err != nil {
		return "", source, err
	}
	p, err := SafePath(root, append([]string{rel}, parts...)...)
	if err != nil {
		return "", source, err
	}
	st, err := os.Lstat(p)
	if err != nil {
		return "", source, errors.New("storage_folder_unavailable")
	}
	if !st.IsDir() {
		return "", source, errors.New("not_a_directory")
	}
	var expected string
	if err = q.QueryRow(ctx, `SELECT CASE WHEN n.ns_parent_id=-1 THEN s.root_identity ELSE n.fs_identity END FROM namespace n JOIN storage_source s ON s.id=n.source_id WHERE n.ns_id=$1`, requestedID).Scan(&expected); err != nil {
		return "", source, err
	}
	if expected != "" && expected != Identity(st) {
		return "", source, errors.New("folder_identity_changed")
	}
	return p, source, nil
}

// AvailableName checks both indexed and unindexed disk entries. The actual
// installation also uses a no-replace rename, protecting external writers.
func AvailableName(ctx context.Context, q dbgen.DBTX, root string, ns int64, name string, exceptPhoto, exceptFolder int64) (string, error) {
	if !ValidComponent(name) {
		return "", errors.New("invalid_file_name")
	}
	dir, s, err := Folder(ctx, q, root, ns)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		candidate := name
		if n > 0 {
			tail := fmt.Sprintf(" (%d)%s", n, ext)
			r := []rune(stem)
			for len(string(r))+len(tail) > 255 && len(r) > 0 {
				r = r[:len(r)-1]
			}
			candidate = string(r) + tail
		}
		var taken bool
		err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM namespace WHERE ns_parent_id=$1 AND ns_name=$2 AND ns_id<>$4) OR EXISTS(SELECT 1 FROM photo WHERE ns_id=$1 AND file_name=$2 AND photo_id<>$3) OR EXISTS(SELECT 1 FROM media_asset a JOIN photo p USING(photo_id) WHERE p.ns_id=$1 AND a.role='live_original' AND EXISTS(SELECT 1 FROM photo_media m WHERE m.photo_id=p.photo_id AND m.live) AND coalesce(nullif(a.display_name,''),a.filename)=$2 AND p.photo_id<>$3)`, ns, candidate, exceptPhoto, exceptFolder).Scan(&taken)
		if err != nil {
			return "", err
		}
		if !taken && s.Kind == "local" {
			_, e := os.Lstat(filepath.Join(dir, candidate))
			if e == nil {
				taken = true
			} else if !os.IsNotExist(e) {
				return "", e
			}
		}
		if !taken {
			return candidate, nil
		}
	}
}

type location struct {
	store Store
	key   string
}

var locations sync.Map // absolute cache path -> immutable source-specific location

func Bind(path string, store Store, key string) {
	locations.Store(filepath.Clean(path), location{store, key})
}
func remoteLocation(path string) (location, bool) {
	v, ok := locations.Load(filepath.Clean(path))
	if !ok {
		return location{}, false
	}
	return v.(location), true
}

func AvatarPath(root, uuid string) (string, error) {
	return SafePath(root, ".assets", uuid, "avatar.jpg")
}

// Resolve locates a persisted asset, never interpreting a user supplied path.
func Resolve(ctx context.Context, q dbgen.DBTX, root, owner, photoUUID, role string) (string, error) {
	canonical, e := canonicalRoot(root)
	if e != nil {
		return "", e
	}
	root = canonical
	var pid, ns, sid int64
	var filename string
	err := q.QueryRow(ctx, `SELECT p.photo_id,p.ns_id,p.source_id,a.filename FROM photo p JOIN "user" u USING(user_id) JOIN media_asset a USING(photo_id) WHERE u.uuid=$1 AND p.photo_uuid=$2 AND a.role=$3`, owner, photoUUID, role).Scan(&pid, &ns, &sid, &filename)
	if err != nil {
		return "", err
	}
	s, err := GetSource(ctx, q, sid)
	if err != nil {
		return "", err
	}
	if s.Kind == "local" {
		if role == "original" || role == "live_original" {
			dir, _, err := Folder(ctx, q, root, ns)
			if err != nil {
				return "", err
			}
			rel, e := filepath.Rel(root, dir)
			if e != nil {
				return "", e
			}
			return SafePath(root, append(strings.Split(rel, string(filepath.Separator)), filename)...)
		}
		return SafePath(root, ".assets", owner, photoUUID, filename)
	}
	if s.Kind != "s3" {
		return "", errors.New("unsupported_storage_source")
	}
	p, err := SafePath(root, ".remote", owner, fmt.Sprint(sid), photoUUID, filename)
	if err != nil {
		return "", err
	}
	st, err := SourceStore(s)
	if err != nil {
		return "", err
	}
	Bind(p, st, owner+"/"+fmt.Sprint(sid)+"/"+photoUUID+"/"+filename)
	return p, nil
}

// ResolveLegacyName keeps public UUID URLs stable while resolving through assets.
func ResolveLegacyName(ctx context.Context, q dbgen.DBTX, root, owner, name string) (string, error) {
	if name == owner+".jpg" {
		return AvatarPath(root, owner)
	}
	if !ValidComponent(name) {
		return "", errors.New("invalid_file_name")
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	role := "original"
	for suffix, r := range map[string]string{"_thumbnail": "thumbnail", "_preview": "preview", "_live": "live_original"} {
		if strings.HasSuffix(stem, suffix) {
			stem = strings.TrimSuffix(stem, suffix)
			role = r
			break
		}
	}
	if role == "live_original" && filepath.Ext(name) == ".mp4" {
		role = "live_preview"
	}
	return Resolve(ctx, q, root, owner, stem, role)
}

func CreateAssets(ctx context.Context, q dbgen.DBTX, root string, pid int64, uuid, filename, format string, ns int64) error {
	_, s, err := Folder(ctx, q, root, ns)
	if err != nil {
		return err
	}
	original := filename
	if s.Kind == "s3" {
		original = "original." + format
	}
	live, err := AvailableName(ctx, q, root, ns, strings.TrimSuffix(filename, filepath.Ext(filename))+"_live.mov", pid, 0)
	if err != nil {
		return err
	}
	liveDisplay := live
	if s.Kind == "s3" {
		live = "live.mov"
	}
	for role, name := range map[string]string{"original": original, "thumbnail": "thumbnail." + format, "preview": "preview.jpg", "live_original": live, "live_preview": "live.mp4"} {
		display := name
		if role == "original" {
			display = filename
		}
		if role == "live_original" {
			display = liveDisplay
		}
		if _, err = q.Exec(ctx, `INSERT INTO media_asset(photo_id,role,filename,display_name) VALUES($1,$2,$3,$4)`, pid, role, name, display); err != nil {
			return err
		}
	}
	return nil
}

// Usage includes arbitrary regular files in the local source and persistent local
// derivatives. Remote cache and scratch files are excluded.
func Usage(root, owner string) int64 {
	var n int64
	for _, parts := range [][]string{{owner}, {".assets", owner}} {
		dir, err := SafePath(root, parts...)
		if err != nil {
			continue
		}
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return nil
			}
			if d.Type().IsRegular() {
				if st, e := d.Info(); e == nil {
					n += st.Size()
				}
			}
			return nil
		})
	}
	return n
}

type inactiveKey struct{}

func AssetPaths(ctx context.Context, q dbgen.DBTX, root string, pid int64) ([]string, error) {
	ctx = context.WithValue(ctx, inactiveKey{}, true)
	var owner, uuid string
	if err := q.QueryRow(ctx, `SELECT u.uuid,p.photo_uuid FROM photo p JOIN "user" u USING(user_id) WHERE photo_id=$1`, pid).Scan(&owner, &uuid); err != nil {
		return nil, err
	}
	var live bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM photo_media WHERE photo_id=$1 AND live)`, pid).Scan(&live); err != nil {
		return nil, err
	}
	paths := []string{}
	for _, role := range []string{"original", "thumbnail", "preview", "live_original", "live_preview"} {
		if role == "live_original" && !live {
			continue
		}
		p, err := Resolve(ctx, q, root, owner, uuid, role)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func CreateFolder(ctx context.Context, q dbgen.DBTX, f *Files, root string, uid, parent int64, name string) (int64, string, error) {
	name, err := AvailableName(ctx, q, root, parent, name, 0, 0)
	if err != nil {
		return 0, "", err
	}
	dir, s, err := Folder(ctx, q, root, parent)
	if err != nil {
		return 0, "", err
	}
	if s.UserID != uid {
		return 0, "", errors.New("folder_not_owned")
	}
	if s.Kind == "local" {
		for {
			err = f.Mkdir(filepath.Join(dir, name))
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrExist) {
				return 0, "", err
			}
			name, err = AvailableName(ctx, q, root, parent, name, 0, 0)
			if err != nil {
				return 0, "", err
			}
		}
	}
	var id int64
	err = q.QueryRow(ctx, `INSERT INTO namespace(ns_name,ns_parent_id,user_id) VALUES($1,$2,$3) RETURNING ns_id`, name, parent, uid).Scan(&id)
	if err == nil && s.Kind == "local" {
		if st, e := os.Lstat(filepath.Join(dir, name)); e == nil {
			_, err = q.Exec(ctx, `UPDATE namespace SET fs_identity=$2 WHERE ns_id=$1`, id, Identity(st))
		}
	}
	return id, name, err
}

// ClientError classifies expected library-operation failures, not backend outages.
func ClientError(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "invalid_file_name", "symbolic_links_not_supported", "cross_source_move", "folder_cycle", "protected_folder", "folder_not_owned", "photo_not_owned", "storage_source_disabled", "folder_identity_changed", "not_a_directory", "storage_folder_unavailable":
		return true
	}
	return errors.Is(err, os.ErrExist)
}

// EnsureRoots creates only never-initialized default roots. A missing established
// root must be reported by the scanner, not recreated and interpreted as empty.
func EnsureRoots(ctx context.Context, q dbgen.DBTX, root string) error {
	rows, err := q.Query(ctx, `SELECT user_id,root_path FROM storage_source WHERE is_default AND root_identity=''`)
	if err != nil {
		return err
	}
	type pending struct {
		uid   int64
		owner string
	}
	var roots []pending
	for rows.Next() {
		var v pending
		if err = rows.Scan(&v.uid, &v.owner); err != nil {
			rows.Close()
			return err
		}
		roots = append(roots, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range roots {
		p, err := SafePath(root, v.owner)
		if err != nil {
			return err
		}
		if err = mkdirAll(p); err != nil {
			return err
		}
		if err = RecordRootIdentity(ctx, q, v.uid, p); err != nil {
			return err
		}
	}
	return nil
}

// RecordRootIdentity establishes the guard when a root is first created, even if
// its first scan later fails on an unreadable or still-changing file.
func RecordRootIdentity(ctx context.Context, q dbgen.DBTX, uid int64, path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return errors.New("storage root is not a directory")
	}
	_, err = q.Exec(ctx, `UPDATE storage_source SET root_identity=$2 WHERE user_id=$1 AND is_default AND root_identity=''`, uid, Identity(st))
	return err
}
