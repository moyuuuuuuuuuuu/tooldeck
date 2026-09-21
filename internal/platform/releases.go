package platform

import (
	"crypto/sha256"
	"encoding/binary"
	"net/http"
	"time"
)

type Release struct {
	Default  string    `json:"default_version"`
	Previous string    `json:"previous_version,omitempty"`
	Canary   string    `json:"canary_version,omitempty"`
	Percent  int       `json:"canary_percent"`
	Updated  time.Time `json:"updated_at"`
}

func publishable(t Tool) bool {
	return !t.Playground && !t.Withdrawn && (t.BuildStatus == "" || t.BuildStatus == "ready") && (t.ReviewStatus == "" || t.ReviewStatus == "approved")
}
func (s *Store) seedReleases() {
	if s.State.Releases == nil {
		s.State.Releases = map[string]Release{}
	}
	tools := []Tool{}
	for _, t := range s.State.Tools {
		tools = append(tools, t)
	}
	for _, t := range catalogTools(tools) {
		key := toolFamily(t)
		if _, ok := s.State.Releases[key]; !ok {
			s.State.Releases[key] = Release{Default: t.ID, Updated: time.Now()}
		}
	}
}
func (s *Server) releasedCatalog(p Principal, available []Tool) []Tool {
	s.store.Lock()
	defer s.store.Unlock()
	byID := map[string]Tool{}
	for _, t := range available {
		if publishable(t) {
			byID[t.ID] = t
		}
	}
	out := []Tool{}
	for _, fallback := range catalogTools(available) {
		rel, exists := s.store.State.Releases[toolFamily(fallback)]
		if !exists {
			out = append(out, fallback)
			continue
		}
		t, ok := byID[rel.Default]
		if !ok {
			continue
		}
		digest := sha256.Sum256([]byte(p.owner() + ":" + toolFamily(t)))
		if c, ok := byID[rel.Canary]; ok && int(binary.BigEndian.Uint32(digest[:4])%100) < rel.Percent {
			t = c
		}
		out = append(out, t)
	}
	return out
}
func (s *Server) releaseEndpoint(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	var b struct {
		Action  string `json:"action"`
		Percent int    `json:"percent"`
	}
	if r.Method == "POST" {
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
	} else if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	t, ok := s.store.State.Tools[id]
	if !ok || t.Playground || (!p.Admin && toolOwner(t) != p.owner()) {
		fail(w, 404, "tool not found")
		return
	}
	key := toolFamily(t)
	rel := s.store.State.Releases[key]
	if r.Method == "GET" {
		data := map[string]any{"release": rel, "version": t.Manifest}
		if other := r.URL.Query().Get("compare"); other != "" {
			target, exists := s.store.State.Tools[other]
			if !exists || toolFamily(target) != key {
				fail(w, 404, "version not found")
				return
			}
			data["compare"] = target.Manifest
		}
		jsonResponse(w, 200, data)
		return
	}
	old := rel
	switch b.Action {
	case "default":
		if !publishable(t) {
			fail(w, 409, "只能发布已构建且审核通过的版本")
			return
		}
		if rel.Default != id {
			rel.Previous = rel.Default
			rel.Default = id
		}
		rel.Canary = ""
		rel.Percent = 0
	case "canary":
		base, exists := s.store.State.Tools[rel.Default]
		if !publishable(t) || !exists || !publishable(base) || id == rel.Default || b.Percent < 1 || b.Percent > 99 {
			fail(w, 422, "灰度需要有效默认版本、不同的已发布版本及 1–99% 比例")
			return
		}
		if (t.Public == nil || *t.Public) != (base.Public == nil || *base.Public) {
			fail(w, 422, "灰度版本可见范围必须一致")
			return
		}
		rel.Canary = id
		rel.Percent = b.Percent
	case "stop-canary":
		rel.Canary = ""
		rel.Percent = 0
	case "rollback":
		previous, exists := s.store.State.Tools[rel.Previous]
		if !exists || !publishable(previous) {
			fail(w, 409, "没有可回滚的已发布版本")
			return
		}
		rel.Default, rel.Previous = rel.Previous, rel.Default
		rel.Canary = ""
		rel.Percent = 0
	default:
		fail(w, 422, "invalid release action")
		return
	}
	rel.Updated = time.Now()
	s.store.State.Releases[key] = rel
	if e := s.store.save(); e != nil {
		s.store.State.Releases[key] = old
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 200, rel)
}
