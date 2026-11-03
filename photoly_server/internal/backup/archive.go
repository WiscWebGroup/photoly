package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
	"github.com/klauspost/compress/zstd"
)

type Part struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Archive struct {
	Root      string `json:"root"`
	Parts     []Part `json:"parts"`
	Inventory []Part `json:"inventory"`
	Bytes     int64  `json:"bytes"`
	Files     int64  `json:"files"`
}
type entry struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256,omitempty"`
	Directory bool   `json:"directory,omitempty"`
}
type splitWriter struct {
	dir, prefix string
	limit       int64
	f           *os.File
	h           hash.Hash
	n           int64
	parts       []Part
}

func (w *splitWriter) finish() error {
	if w.f == nil {
		return nil
	}
	if e := w.f.Sync(); e != nil {
		return e
	}
	if e := w.f.Close(); e != nil {
		return e
	}
	w.parts = append(w.parts, Part{filepath.Base(w.f.Name()), w.n, hex.EncodeToString(w.h.Sum(nil))})
	w.f = nil
	return nil
}
func (w *splitWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		if w.f == nil {
			name := fmt.Sprintf("%s.%06d", w.prefix, len(w.parts)+1)
			f, e := os.OpenFile(filepath.Join(w.dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return total, e
			}
			w.f = f
			w.h = sha256.New()
			w.n = 0
		}
		n := min(int64(len(p)), w.limit-w.n)
		k, e := w.f.Write(p[:n])
		w.h.Write(p[:k])
		w.n += int64(k)
		total += k
		p = p[k:]
		if e != nil {
			return total, e
		}
		if w.n == w.limit {
			if e = w.finish(); e != nil {
				return total, e
			}
		}
	}
	return total, nil
}
func encryptWriter(dir, prefix, password string, limit int64) (*splitWriter, io.WriteCloser, error) {
	r, e := age.NewScryptRecipient(password)
	if e != nil {
		return nil, nil, e
	}
	w := &splitWriter{dir: dir, prefix: prefix, limit: limit}
	enc, e := age.Encrypt(w, r)
	return w, enc, e
}
func finishEncrypted(enc io.WriteCloser, w *splitWriter) error {
	if e := enc.Close(); e != nil {
		return e
	}
	return w.finish()
}

// WalkDir retains only one directory's entries; file data and checksums are streamed.
func pack(ctx context.Context, root, relative, dir, password string, limit int64, progress func(int64, int64)) (out Archive, err error) {
	out.Root = relative
	src, e := safe(root, relative)
	if e != nil {
		return out, e
	}
	prefix := fmt.Sprintf("archive-%x", sha256.Sum256([]byte(relative)))[:24]
	sw, enc, e := encryptWriter(dir, prefix, password, limit)
	if e != nil {
		return out, e
	}
	defer func() {
		if sw.f != nil {
			sw.f.Close()
		}
	}()
	iw, inv, e := encryptWriter(dir, prefix+"-inventory", password, limit)
	if e != nil {
		return out, e
	}
	defer func() {
		if iw.f != nil {
			iw.f.Close()
		}
	}()
	inventory := json.NewEncoder(inv)
	zw, e := zstd.NewWriter(enc, zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedFastest))
	if e != nil {
		return out, e
	}
	tw := tar.NewWriter(zw)
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		st, e := d.Info()
		if e != nil {
			return e
		}
		if !st.IsDir() && !st.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", path)
		}
		name, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		name = filepath.ToSlash(name)
		h, e := tar.FileInfoHeader(st, "")
		if e != nil {
			return e
		}
		h.Name = name
		h.Uid = 0
		h.Gid = 0
		h.Uname = ""
		h.Gname = ""
		if e = tw.WriteHeader(h); e != nil {
			return e
		}
		record := entry{Name: name, Size: st.Size(), Directory: st.IsDir()}
		if !st.IsDir() {
			f, e := os.Open(path)
			if e != nil {
				return e
			}
			hash := sha256.New()
			n, e := io.Copy(tw, io.TeeReader(contextReader{ctx, f}, hash))
			after, se := f.Stat()
			f.Close()
			if e != nil {
				return e
			}
			if se != nil {
				return se
			}
			if n != st.Size() || after.Size() != st.Size() || !after.ModTime().Equal(st.ModTime()) {
				return errors.New("file changed during backup")
			}
			record.SHA256 = hex.EncodeToString(hash.Sum(nil))
			out.Bytes += n
			out.Files++
			progress(1, n)
		} else {
			record.Size = 0
		}
		return inventory.Encode(record)
	})
	if err != nil {
		return out, err
	}
	for _, close := range []func() error{tw.Close, zw.Close, func() error { return finishEncrypted(enc, sw) }, func() error { return finishEncrypted(inv, iw) }} {
		if e = close(); e != nil {
			return out, e
		}
	}
	out.Parts = sw.parts
	out.Inventory = iw.parts
	return out, nil
}

type partsReader struct {
	dir   string
	parts []Part
	f     *os.File
}

func (r *partsReader) Read(p []byte) (int, error) {
	for {
		if r.f == nil {
			if len(r.parts) == 0 {
				return 0, io.EOF
			}
			path, e := safe(r.dir, r.parts[0].Name)
			if e != nil {
				return 0, e
			}
			f, e := os.Open(path)
			if e != nil {
				return 0, e
			}
			r.f = f
			r.parts = r.parts[1:]
		}
		n, e := r.f.Read(p)
		if e == io.EOF {
			r.f.Close()
			r.f = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, e
	}
}
func (r *partsReader) Close() error {
	if r.f != nil {
		return r.f.Close()
	}
	return nil
}
func decryptParts(ctx context.Context, dir, password string, parts []Part) (io.Reader, func(), error) {
	if len(parts) == 0 {
		return nil, nil, errors.New("missing archive parts")
	}
	for _, p := range parts {
		path, e := safe(dir, p.Name)
		if e != nil {
			return nil, nil, e
		}
		st, e := os.Stat(path)
		if e != nil {
			return nil, nil, e
		}
		if st.Size() != p.Size {
			return nil, nil, errors.New("part size mismatch")
		}
		hash, e := digest(path)
		if e != nil {
			return nil, nil, e
		}
		if hash != p.SHA256 {
			return nil, nil, errors.New("part checksum mismatch")
		}
	}
	id, e := age.NewScryptIdentity(password)
	if e != nil {
		return nil, nil, e
	}
	r := &partsReader{dir: dir, parts: parts}
	dec, e := age.Decrypt(contextReader{ctx, r}, id)
	if e != nil {
		r.Close()
		return nil, nil, errors.New("incorrect password or damaged encrypted archive")
	}
	return dec, func() { r.Close() }, nil
}
func unpack(ctx context.Context, dir, target, password string, a Archive, progress func(int64, int64)) error {
	dec, close, e := decryptParts(ctx, dir, password, a.Parts)
	if e != nil {
		return e
	}
	defer close()
	ir, ic, e := decryptParts(ctx, dir, password, a.Inventory)
	if e != nil {
		return e
	}
	defer ic()
	inventory := json.NewDecoder(ir)
	zr, e := zstd.NewReader(dec, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(64<<20))
	if e != nil {
		return e
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	var files, bytes int64
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		var record entry
		if e = inventory.Decode(&record); e != nil {
			return e
		}
		if record.Name != h.Name || record.Size != h.Size || record.Directory != (h.Typeflag == tar.TypeDir) {
			return errors.New("archive inventory mismatch")
		}
		if h.Name != a.Root && !isChild(a.Root, h.Name) {
			return errors.New("archive escaped its root")
		}
		path, e := safe(target, h.Name)
		if e != nil {
			return e
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if exists(path) {
				return errors.New("duplicate archive directory")
			}
			if e = os.Mkdir(path, 0700); e != nil {
				return e
			}
		case tar.TypeReg:
			f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			hash := sha256.New()
			n, e := io.Copy(f, io.TeeReader(contextReader{ctx, tr}, hash))
			if e == nil {
				e = f.Sync()
			}
			ce := f.Close()
			if e != nil {
				return e
			}
			if ce != nil {
				return ce
			}
			if n != record.Size || hex.EncodeToString(hash.Sum(nil)) != record.SHA256 {
				return errors.New("file checksum mismatch")
			}
			files++
			bytes += n
			progress(1, n)
		default:
			return errors.New("unsafe archive entry type")
		}
		if h.Typeflag == tar.TypeReg {
			if e = os.Chtimes(path, time.Now(), h.ModTime); e != nil {
				return e
			}
		}
	}
	// Consume encrypted EOF: truncated age streams must never pass verification.
	if _, e = io.Copy(io.Discard, zr); e != nil {
		return e
	}
	if _, e = io.Copy(io.Discard, dec); e != nil {
		return e
	}
	var extra entry
	if e = inventory.Decode(&extra); e != io.EOF {
		return errors.New("unexpected or damaged inventory data")
	}
	if files != a.Files || bytes != a.Bytes {
		return errors.New("archive totals mismatch")
	}
	// Persist directory entries as well as file contents before publishing a restore.
	if e = filepath.WalkDir(filepath.Join(target, a.Root), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return syncDir(path)
		}
		return nil
	}); e != nil {
		return e
	}
	return syncDir(target)
}
func isChild(root, path string) bool { return len(path) > len(root) && path[:len(root)+1] == root+"/" }
