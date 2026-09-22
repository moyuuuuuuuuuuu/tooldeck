package platform

import (
	"context"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type adminToolRow struct {
	Tool     Tool           `json:"tool"`
	Statuses map[string]int `json:"statuses"`
	Total    int            `json:"total"`
	LastRun  *time.Time     `json:"last_run_at,omitempty"`
}

type adminRunRow struct {
	ID       string     `json:"run_id"`
	ToolID   string     `json:"tool_id"`
	Owner    string     `json:"owner"`
	Status   string     `json:"status"`
	Source   string     `json:"source,omitempty"`
	Created  time.Time  `json:"created_at"`
	Started  *time.Time `json:"started_at,omitempty"`
	Duration int64      `json:"duration_ms"`
}

func monitorPage(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func (s *Server) adminToolsEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Admin || !p.Session {
		fail(w, 403, "admin session required")
		return
	}
	if r.Method != http.MethodGet || len(parts) > 3 {
		fail(w, 405, "method not allowed")
		return
	}
	s.store.Lock()
	rows := make([]adminToolRow, 0, len(s.store.State.Tools))
	index := make(map[string]int, len(s.store.State.Tools))
	for _, tool := range s.store.State.Tools {
		if tool.Playground {
			continue
		}
		release := s.store.State.Releases[toolFamily(tool)]
		tool.DefaultVersion = release.Default == tool.ID
		if release.Canary == tool.ID {
			tool.CanaryPercent = release.Percent
		}
		index[tool.ID] = len(rows)
		rows = append(rows, adminToolRow{Tool: tool, Statuses: map[string]int{}})
	}
	for _, run := range s.store.State.Runs {
		if i, ok := index[run.ToolID]; ok {
			rows[i].Total++
			rows[i].Statuses[run.Status]++
			if rows[i].LastRun == nil || run.Created.After(*rows[i].LastRun) {
				created := run.Created
				rows[i].LastRun = &created
			}
		}
	}
	s.store.Unlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].Tool.Created.After(rows[j].Tool.Created) })
	if len(parts) == 3 {
		if parts[2] == "summary" {
			choices := make([]map[string]string, 0, len(rows))
			published, queued, running := 0, 0, 0
			for _, row := range rows {
				choices = append(choices, map[string]string{"id": row.Tool.ID, "title": row.Tool.Manifest.Title, "version": row.Tool.Manifest.Version})
				if (row.Tool.ReviewStatus == "" || row.Tool.ReviewStatus == "approved") && (row.Tool.BuildStatus == "" || row.Tool.BuildStatus == "ready") && !row.Tool.Withdrawn {
					published++
				}
				queued += row.Statuses["queued"]
				running += row.Statuses["running"] + row.Statuses["canceling"]
			}
			jsonResponse(w, 200, map[string]any{"versions": len(rows), "published": published, "queued": queued, "running": running, "tools": choices})
			return
		}
		for _, row := range rows {
			if row.Tool.ID == parts[2] {
				jsonResponse(w, 200, row)
				return
			}
		}
		fail(w, 404, "tool unavailable")
		return
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	filtered := make([]adminToolRow, 0, len(rows))
	for _, row := range rows {
		if search == "" || strings.Contains(strings.ToLower(row.Tool.Manifest.Title+" "+row.Tool.Manifest.Name+" "+row.Tool.Owner+" "+row.Tool.ID), search) {
			row.Tool.BuildLog = ""
			filtered = append(filtered, row)
		}
	}
	page, size := monitorPage(r)
	start, end := pageBounds(len(filtered), page, size)
	jsonResponse(w, 200, map[string]any{"items": filtered[start:end], "total": len(filtered), "page": page, "page_size": size})
}

func (s *Server) adminRunsEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Admin || !p.Session {
		fail(w, 403, "admin session required")
		return
	}
	if len(parts) == 4 && parts[3] == "terminate" && r.Method == http.MethodPost {
		s.adminTerminateRun(w, r, parts[2])
		return
	}
	if r.Method != http.MethodGet || len(parts) != 2 {
		fail(w, 405, "method not allowed")
		return
	}
	toolID, status := r.URL.Query().Get("tool_id"), r.URL.Query().Get("status")
	s.store.Lock()
	rows := make([]adminRunRow, 0, len(s.store.State.Runs))
	for _, run := range s.store.State.Runs {
		if toolID != "" && run.ToolID != toolID || status != "" && run.Status != status {
			continue
		}
		rows = append(rows, adminRunRow{ID: run.ID, ToolID: run.ToolID, Owner: run.Owner, Status: run.Status, Source: run.Source, Created: run.Created, Started: run.Started, Duration: run.Duration})
	}
	s.store.Unlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].Created.After(rows[j].Created) })
	page, size := monitorPage(r)
	start, end := pageBounds(len(rows), page, size)
	jsonResponse(w, 200, map[string]any{"items": rows[start:end], "total": len(rows), "page": page, "page_size": size})
}

func (s *Server) adminTerminateRun(w http.ResponseWriter, r *http.Request, id string) {
	s.store.Lock()
	run, ok := s.store.State.Runs[id]
	if !ok {
		s.store.Unlock()
		fail(w, 404, "run unavailable")
		return
	}
	if !runActive(run.Status) {
		s.store.Unlock()
		jsonResponse(w, 200, run)
		return
	}
	previous := run
	run.Status = "canceled"
	s.store.State.Runs[id] = run
	if err := s.store.save(); err != nil {
		s.store.State.Runs[id] = previous
		s.store.Unlock()
		fail(w, 500, err)
		return
	}
	s.store.Unlock()
	s.cancelMu.Lock()
	cancel := s.cancels[id]
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if previous.Status != "queued" {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		output, err := exec.CommandContext(ctx, "docker", "rm", "-f", "tooldeck-run-"+id).CombinedOutput()
		stop()
		if err != nil && !strings.Contains(strings.ToLower(string(output)), "no such container") {
			s.store.Lock()
			current := s.store.State.Runs[id]
			current.Status = "cancel_failed"
			current.CancelError = "强制终止容器失败：" + err.Error()
			s.store.State.Runs[id] = current
			_ = s.store.save()
			s.store.Unlock()
			fail(w, 502, current.CancelError)
			return
		}
	}
	jsonResponse(w, 200, run)
}
