// Generate notices from the resolved Go module graph, including local replacements.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type module struct {
	Path, Version, Dir string
	Main               bool
	Replace            *module
}
type notice struct{ File, SHA256 string }
type entry struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Source      string   `json:"source"`
	License     string   `json:"license"`
	Status      string   `json:"status"`
	Files       []notice `json:"files"`
	Replacement string   `json:"replacement,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	raw, err := exec.Command("go", "list", "-mod=readonly", "-m", "-json", "all").Output()
	if err != nil {
		return err
	}
	deps, err := exec.Command("go", "list", "-mod=readonly", "-deps", "-json", "./cmd/photoly").Output()
	if err != nil {
		return err
	}
	used := map[string]bool{}
	depdec := json.NewDecoder(bytes.NewReader(deps))
	for {
		var d struct{ Module *module }
		err = depdec.Decode(&d)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if d.Module != nil {
			used[d.Module.Path] = true
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var mods []module
	for {
		var m module
		err = dec.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !m.Main && used[m.Path] {
			mods = append(mods, m)
		}
	}
	sort.Slice(mods, func(i, j int) bool { return mods[i].Path < mods[j].Path })
	var out strings.Builder
	out.WriteString("Photoly Go dependency notices\nModules retain their own licenses. Modules linked into cmd/photoly, including local replacements.\n\n")
	var entries []entry
	for _, m := range mods {
		dir := m.Dir
		if m.Replace != nil {
			dir = m.Replace.Dir
		}
		if dir == "" {
			return fmt.Errorf("missing source for %s; run go mod download all", m.Path)
		}
		e := entry{Name: m.Path, Version: m.Version, Source: "https://pkg.go.dev/" + m.Path + "@" + m.Version, Status: "notice-collected"}
		if m.Replace != nil {
			e.Replacement = m.Replace.Path
		}
		var names []string
		err = filepath.WalkDir(dir, func(path string, f fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if f.IsDir() {
				if f.Name() == "testdata" || f.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			lower := strings.ToLower(f.Name())
			if strings.HasPrefix(lower, "license") || strings.HasPrefix(lower, "licence") || strings.HasPrefix(lower, "notice") || strings.HasPrefix(lower, "copying") || strings.HasPrefix(lower, "copyright") {
				name, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				names = append(names, filepath.ToSlash(name))
			}
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(names)
		if len(names) == 0 {
			return fmt.Errorf("no license for %s", m.Path)
		}
		out.WriteString(m.Path + "@" + m.Version + "\n" + e.Source + "\n\n")
		for _, name := range names {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			e.Files = append(e.Files, notice{name, hex.EncodeToString(sum[:])})
			out.WriteString(name + "\n" + string(b) + "\n\n")
			text := string(b)
			lic := ""
			switch {
			case strings.Contains(text, "GNU LESSER GENERAL PUBLIC LICENSE") && strings.Contains(text, "Version 3, 29 June 2007"):
				lic = "LGPL-3.0-only"
			case strings.Contains(text, "Apache License"):
				lic = "Apache-2.0"
			case strings.Contains(text, "Permission is hereby granted"):
				lic = "MIT"
			case strings.Contains(text, "Redistribution and use in source and binary forms"):
				if strings.Contains(text, "Neither the name") {
					lic = "BSD-3-Clause"
				} else {
					lic = "BSD-2-Clause"
				}
			case strings.Contains(text, "ISC License") || strings.Contains(text, "Permission to use, copy, modify, and distribute"):
				lic = "ISC"
			case strings.Contains(text, "Mozilla Public License"):
				lic = "MPL-2.0"
			}
			if lic != "" && !strings.Contains(e.License, lic) {
				if e.License != "" {
					e.License += " AND "
				}
				e.License += lic
			}
		}
		// The SMB snapshot contains separately licensed Go-derived source files.
		if m.Path == "github.com/cloudsoda/go-smb2" && m.Replace != nil {
			for _, name := range []string{"all.go", "filepath.go"} {
				b, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					return err
				}
				header, _, _ := strings.Cut(string(b), "package ")
				if strings.Contains(header, "Copyright") {
					sum := sha256.Sum256([]byte(header))
					e.Files = append(e.Files, notice{name + ":header", hex.EncodeToString(sum[:])})
					out.WriteString(name + " copyright header\n" + header + "\n")
				}
			}
			e.License = "BSD-2-Clause AND BSD-3-Clause AND AGPL-3.0-or-later"
		}
		if e.License == "" {
			return fmt.Errorf("unreviewed license for %s", m.Path)
		}
		entries = append(entries, e)
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	for name, data := range map[string][]byte{"third-party-notices.txt": []byte(out.String()), "dependencies.json": b} {
		p := filepath.Join("internal/legal/data", name)
		if len(os.Args) > 1 && os.Args[1] == "--check" {
			old, err := os.ReadFile(p)
			if err != nil || !bytes.Equal(old, data) {
				return fmt.Errorf("stale %s", p)
			}
		} else {
			if err := os.WriteFile(p, data, 0644); err != nil {
				return err
			}
		}
	}
	fmt.Printf("Collected %d Go module notices\n", len(entries))
	return nil
}
