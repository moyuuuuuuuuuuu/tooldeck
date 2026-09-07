package platform

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractPackage rejects traversal, links, duplicate names and oversized archives before publishing.
func ExtractPackage(zipPath, dest string) (Manifest, error) {
	var m Manifest
	z, e := zip.OpenReader(zipPath)
	if e != nil {
		return m, e
	}
	defer z.Close()
	if len(z.File) > 2000 {
		return m, errors.New("too many files (max 2000)")
	}
	seen := map[string]bool{}
	var total uint64
	for _, f := range z.File {
		p := strings.TrimSuffix(f.Name, "/")
		if !validPath(p) || f.Mode()&os.ModeSymlink != 0 || !f.Mode().IsRegular() && !f.FileInfo().IsDir() {
			return m, errors.New("unsafe archive entry")
		}
		if seen[strings.ToLower(p)] {
			return m, errors.New("duplicate archive entry")
		}
		seen[strings.ToLower(p)] = true
		if f.UncompressedSize64 > 64<<20 {
			return m, errors.New("entry too large")
		}
		total += f.UncompressedSize64
		if total > 128<<20 {
			return m, errors.New("archive exceeds 128 MB expanded")
		}
	}
	if !seen["tooldeck.json"] {
		return m, errors.New("tooldeck.json must be at ZIP root")
	}
	if e = os.MkdirAll(dest, 0755); e != nil {
		return m, e
	}
	for _, f := range z.File {
		p := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(p, 0755); e != nil {
				return m, e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return m, e
		}
		r, err := f.Open()
		if err != nil {
			return m, err
		}
		w, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			r.Close()
			return m, err
		}
		n, err := io.Copy(w, io.LimitReader(r, 64<<20+1))
		w.Close()
		r.Close()
		if err != nil {
			return m, err
		}
		if n > 64<<20 {
			return m, errors.New("entry too large")
		}
	}
	b, e := os.ReadFile(filepath.Join(dest, "tooldeck.json"))
	if e != nil {
		return m, e
	}
	if len(b) > 128<<10 {
		return m, errors.New("manifest too large")
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if e = d.Decode(&m); e != nil {
		return m, e
	}
	if e = m.Validate(); e != nil {
		return m, e
	}
	f, e := os.Stat(filepath.Join(dest, m.Entrypoint))
	if (e != nil || !f.Mode().IsRegular()) && m.BuildCommand == "" {
		return m, errors.New("entrypoint must be an existing regular file")
	}
	return m, nil
}
