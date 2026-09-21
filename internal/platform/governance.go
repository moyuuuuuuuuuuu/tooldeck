package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StorageQuotas struct {
	Site int64 `json:"site_bytes"`
	User int64 `json:"user_bytes"`
	Tool int64 `json:"tool_bytes"`
}
type storageReservation struct {
	Owner, Tool string
	Size        int64
}
type StorageItem struct {
	Path      string    `json:"path"`
	Bytes     int64     `json:"bytes"`
	Modified  time.Time `json:"modified_at"`
	Candidate bool      `json:"candidate"`
}

func treeSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
			return nil
		}
		if !d.IsDir() {
			info, e := d.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return errors.New("storage contains a special file")
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}
func (s *Server) reserveStorage(owner, tool string, size int64) (func(), error) {
	s.store.Lock()
	defer s.store.Unlock()
	site, user, usedTool := int64(0), int64(0), int64(0)
	add := func(o, t string, n int64) {
		site += n
		if o == owner {
			user += n
		}
		if t != "" && t == tool {
			usedTool += n
		}
	}
	for id, f := range s.store.State.Files {
		family := ""
		if r, ok := s.store.State.Runs[f.RunID]; ok {
			family = toolFamily(s.store.State.Tools[r.ToolID])
		}
		add(s.store.State.Owners[id], family, f.Size)
	}
	for _, t := range s.store.State.Tools {
		add(toolOwner(t), toolFamily(t), t.StoredBytes+t.ArtifactBytes)
	}
	for _, r := range s.reservations {
		add(r.Owner, r.Tool, r.Size)
	}
	q := s.store.State.StorageQuotas
	if size < 0 || (q.Site > 0 && site+size > q.Site) || (q.User > 0 && user+size > q.User) || (tool != "" && q.Tool > 0 && usedTool+size > q.Tool) {
		return nil, errors.New("存储配额不足，请清理资源或联系管理员")
	}
	id := ID("quota_")
	s.reservations[id] = storageReservation{owner, tool, size}
	return func() { s.store.Lock(); delete(s.reservations, id); s.store.Unlock() }, nil
}

// Called under the store lock. Candidates are unreferenced entries older than
// 24 hours. Runs, including their retained history, protect package references.
func (s *Server) storageInventory() ([]StorageItem, error) {
	referenced := map[string]bool{}
	for _, t := range s.store.State.Tools {
		referenced["packages/"+t.ID] = true
		if t.Artifact != "" {
			referenced["artifacts/"+t.Artifact] = true
		}
	}
	for id, f := range s.store.State.Files {
		if f.Storage != "bos" {
			referenced["files/"+id] = true
		}
	}
	for _, r := range s.store.State.Runs {
		referenced["packages/"+r.ToolID] = true
		if runActive(r.Status) {
			referenced["jobs/"+r.ID] = true
		}
	}
	for id := range s.activeRuns {
		referenced["jobs/"+id] = true
	}
	buildsActive := false
	for _, t := range s.store.State.Tools {
		if t.BuildStatus == "building" {
			buildsActive = true
		}
	}
	list := []StorageItem{}
	for _, dir := range []string{"packages", "artifacts", "files", "jobs", "uploads", "quarantine"} {
		entries, err := os.ReadDir(filepath.Join(s.store.Root, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			relative := dir + "/" + entry.Name()
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			size, err := treeSize(filepath.Join(s.store.Root, relative))
			if err != nil {
				return nil, err
			}
			candidate := dir != "quarantine" && !referenced[relative] && time.Since(info.ModTime()) > 24*time.Hour
			if buildsActive && (dir == "jobs" || dir == "artifacts") {
				candidate = false
			}
			list = append(list, StorageItem{relative, size, info.ModTime(), candidate})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	return list, nil
}
func inventoryToken(items []StorageItem) string { b, _ := json.Marshal(items); return hash(string(b)) }
func (s *Server) storageEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Admin {
		fail(w, 403, "admin session required")
		return
	}
	if len(parts) == 2 && parts[1] == "quotas" && r.Method == "POST" {
		var q StorageQuotas
		if err := decode(w, r, &q); err != nil {
			fail(w, 400, err)
			return
		}
		if q.Site < 0 || q.User < 0 || q.Tool < 0 {
			fail(w, 422, "quota must be non-negative")
			return
		}
		s.store.Lock()
		defer s.store.Unlock()
		old := s.store.State.StorageQuotas
		s.store.State.StorageQuotas = q
		if err := s.store.save(); err != nil {
			s.store.State.StorageQuotas = old
			fail(w, 500, err)
			return
		}
		jsonResponse(w, 200, q)
		return
	}
	if len(parts) == 1 && r.Method == "GET" {
		s.store.Lock()
		items, err := s.storageInventory()
		quotas := s.store.State.StorageQuotas
		var logical int64
		for _, f := range s.store.State.Files {
			logical += f.Size
		}
		for _, t := range s.store.State.Tools {
			logical += t.StoredBytes + t.ArtifactBytes
		}
		s.store.Unlock()
		if err != nil {
			fail(w, 500, err)
			return
		}
		var total int64
		for _, item := range items {
			total += item.Bytes
		}
		jsonResponse(w, 200, map[string]any{"items": items, "preview_token": inventoryToken(items), "local_bytes": total, "retained_bytes": logical, "quotas": quotas})
		return
	}
	if len(parts) == 2 && parts[1] == "cleanup" && r.Method == "POST" {
		var b struct {
			Token string   `json:"preview_token"`
			Paths []string `json:"paths"`
		}
		if err := decode(w, r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if len(b.Paths) == 0 || len(b.Paths) > 100 {
			fail(w, 422, "select 1..100 previewed entries")
			return
		}
		s.store.Lock()
		defer s.store.Unlock()
		items, err := s.storageInventory()
		if err != nil {
			fail(w, 500, err)
			return
		}
		if b.Token != inventoryToken(items) {
			fail(w, 409, "资源已变化，请重新预览")
			return
		}
		allowed := map[string]bool{}
		for _, item := range items {
			allowed[item.Path] = item.Candidate
		}
		selected := map[string]bool{}
		for _, path := range b.Paths {
			if !allowed[path] || selected[path] {
				fail(w, 409, "目标仍被引用或不在清理预览中")
				return
			}
			selected[path] = true
		}
		batch := filepath.Join(s.store.Root, "quarantine", strconv.FormatInt(time.Now().Unix(), 10)+"-"+ID(""))
		if err = os.MkdirAll(batch, 0700); err != nil {
			fail(w, 500, err)
			return
		}
		moved := []string{}
		for _, path := range b.Paths {
			destination := filepath.Join(batch, strings.ReplaceAll(path, "/", "__"))
			if err = os.Rename(filepath.Join(s.store.Root, path), destination); err != nil {
				jsonResponse(w, 207, map[string]any{"moved": moved, "error": fmt.Sprint(err)})
				return
			}
			moved = append(moved, path)
		}
		jsonResponse(w, 200, map[string]any{"moved": moved, "quarantine": filepath.Base(batch)})
		return
	}
	fail(w, 405, "method not allowed")
}
