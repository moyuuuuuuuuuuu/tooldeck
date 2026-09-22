package platform

import "net/http"

// activeBrowserRun requires the store lock. Browser runs are serialized per
// caller and tool family; API-key and OAuth runs keep their own concurrency.
func (s *Server) activeBrowserRun(owner string, tool Tool) *Run {
	family := toolFamily(tool)
	var active *Run
	for _, run := range s.store.State.Runs {
		if run.Owner != owner || !runActive(run.Status) || (run.Source != "web" && run.Source != "guest") {
			continue
		}
		runTool, ok := s.store.State.Tools[run.ToolID]
		if !ok || toolFamily(runTool) != family {
			continue
		}
		if active == nil || run.Created.After(active.Created) {
			copy := run
			active = &copy
		}
	}
	return active
}

func (s *Server) browserActiveRun(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if !p.Session && !p.Guest {
		fail(w, 403, "browser session required")
		return
	}
	s.store.Lock()
	tool, ok := s.store.State.Tools[id]
	if !ok || !canUseTool(p, tool) {
		s.store.Unlock()
		fail(w, 404, "tool unavailable")
		return
	}
	active := s.activeBrowserRun(p.owner(), tool)
	s.store.Unlock()
	jsonResponse(w, 200, active)
}
