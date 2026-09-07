package platform

import (
	"archive/zip"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Server) reviewEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Admin {
		fail(w, 403, "admin session required")
		return
	}
	if parts[0] == "review-settings" {
		if r.Method != "GET" && r.Method != "POST" {
			fail(w, 405, "method not allowed")
			return
		}
		var b struct {
			Required bool `json:"required"`
		}
		if r.Method == "POST" {
			if e := decode(w, r, &b); e != nil {
				fail(w, 400, e)
				return
			}
		}
		s.store.Lock()
		defer s.store.Unlock()
		if r.Method == "POST" {
			old := s.store.State.ReviewRequired
			s.store.State.ReviewRequired = &b.Required
			if e := s.store.save(); e != nil {
				s.store.State.ReviewRequired = old
				fail(w, 500, e)
				return
			}
		}
		jsonResponse(w, 200, map[string]bool{"required": s.store.State.ReviewRequired == nil || *s.store.State.ReviewRequired})
		return
	}
	if len(parts) == 3 && parts[2] == "package" && r.Method == "GET" {
		s.store.Lock()
		t, ok := s.store.State.Tools[parts[1]]
		s.store.Unlock()
		if !ok {
			fail(w, 404, "tool not found")
			return
		}
		root := filepath.Join(s.store.Root, "packages", t.ID)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=tool-package.zip")
		z := zip.NewWriter(w)
		defer z.Close()
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			f, e := os.Open(path)
			if e != nil {
				return e
			}
			defer f.Close()
			out, e := z.Create(filepath.ToSlash(rel))
			if e != nil {
				return e
			}
			_, e = io.Copy(out, f)
			return e
		})
		return
	}
	if len(parts) == 1 && r.Method == "GET" {
		s.store.Lock()
		list := []Tool{}
		for _, t := range s.store.State.Tools {
			if !t.Withdrawn && (t.Public == nil || *t.Public) {
				if u, ok := s.store.State.Users[toolOwner(t)]; ok {
					t.Author = &ToolAuthor{ID: u.ID, Username: u.Username, Nickname: u.Nickname, Email: u.Email, EmailVerified: u.EmailVerified, Bio: u.Bio, Created: u.Created}
				}
				list = append(list, t)
			}
		}
		s.store.Unlock()
		sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
		jsonResponse(w, 200, list)
		return
	}
	if len(parts) == 2 && r.Method == "POST" {
		var b struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		if b.Status == "rejected" && strings.TrimSpace(b.Note) == "" {
			fail(w, 422, "请填写驳回原因")
			return
		}
		if (b.Status != "approved" && b.Status != "rejected") || len([]rune(b.Note)) > 500 {
			fail(w, 422, "审核结果无效或备注超过500字")
			return
		}
		s.store.Lock()
		defer s.store.Unlock()
		t, ok := s.store.State.Tools[parts[1]]
		if !ok {
			fail(w, 404, "tool not found")
			return
		}
		if t.Withdrawn {
			fail(w, 409, "工具已下架，请等待作者重新提交")
			return
		}
		if t.ReviewStatus == "draft" {
			fail(w, 409, "作者提交审核后才能进行审核")
			return
		}
		if t.Public != nil && !*t.Public {
			fail(w, 422, "私有工具无需审核")
			return
		}
		if b.Status == "approved" && t.BuildStatus != "" && t.BuildStatus != "ready" {
			fail(w, 409, "构建成功后才能通过审核")
			return
		}
		old := t
		t.ReviewStatus = b.Status
		t.ReviewNote = b.Note
		s.store.State.Tools[t.ID] = t
		if e := s.store.save(); e != nil {
			s.store.State.Tools[t.ID] = old
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, t)
		return
	}
	fail(w, 405, "method not allowed")
}
