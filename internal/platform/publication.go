package platform

import (
	"net/http"
	"sort"
)

func (s *Server) myTools(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Session {
		fail(w, 403, "需要登录")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	s.store.Lock()
	list := []Tool{}
	for _, t := range s.store.State.Tools {
		if !t.Playground && toolOwner(t) == p.owner() {
			list = append(list, t)
		}
	}
	s.store.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
	jsonResponse(w, 200, list)
}

func (s *Server) publication(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if !p.Session {
		fail(w, 403, "需要登录")
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	var b struct {
		Action string `json:"action"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	if b.Action != "withdraw" && b.Action != "submit" && b.Action != "resubmit" {
		fail(w, 422, "操作无效")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	t, ok := s.store.State.Tools[id]
	if !ok || t.Playground {
		fail(w, 404, "工具不存在")
		return
	}
	if toolOwner(t) != p.owner() {
		fail(w, 403, "只能操作自己上传的工具")
		return
	}
	old := t
	if b.Action == "withdraw" {
		t.Withdrawn = true
	} else {
		if !t.Withdrawn && t.ReviewStatus != "draft" && t.ReviewStatus != "rejected" {
			fail(w, 409, "当前状态不可提交")
			return
		}
		if t.BuildStatus != "" && t.BuildStatus != "ready" {
			fail(w, 409, "请先完成构建")
			return
		}
		t.Withdrawn = false
		t.ReviewStatus = "approved"
		t.ReviewNote = ""
		if (t.Public == nil || *t.Public) && (s.store.State.ReviewRequired == nil || *s.store.State.ReviewRequired) {
			t.ReviewStatus = "pending"
		}
	}
	s.store.State.Tools[id] = t
	if e := s.store.save(); e != nil {
		s.store.State.Tools[id] = old
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 200, t)
}
