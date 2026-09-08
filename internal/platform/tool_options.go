package platform

import (
	"net/http"
	"sort"
)

func canUseTool(p Principal, t Tool) bool {
	if p.Guest {
		return guestTool(t)
	}
	if !p.allows(t.Manifest.Name) {
		return false
	}
	if p.Admin || t.Owner != "" && t.Owner == p.owner() {
		return true
	}
	return !t.Withdrawn && (t.BuildStatus == "" || t.BuildStatus == "ready") && (t.Public == nil || *t.Public) && (t.ReviewStatus == "" || t.ReviewStatus == "approved")
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	if r.Method == "GET" && len(parts) == 1 {
		list := []Run{}
		for _, run := range s.store.State.Runs {
			if run.Owner == p.owner() && !run.NotificationRead && run.CallbackFailed {
				list = append(list, run)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
		if len(list) > 100 {
			list = list[:100]
		}
		jsonResponse(w, 200, list)
		return
	}
	if r.Method == "POST" && len(parts) == 2 {
		run, ok := s.store.State.Runs[parts[1]]
		if !ok || run.Owner != p.owner() {
			fail(w, 404, "not found")
			return
		}
		old := run
		run.NotificationRead = true
		s.store.State.Runs[run.ID] = run
		if e := s.store.save(); e != nil {
			s.store.State.Runs[run.ID] = old
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
		return
	}
	fail(w, 405, "method not allowed")
}
