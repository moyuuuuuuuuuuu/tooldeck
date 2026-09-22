package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminMonitorPermissionsAndFilters(t *testing.T) {
	s := testServer(t)
	admin := Principal{Admin: true, Session: true}
	s.store.State.Tools["tool-1"] = Tool{ID: "tool-1", Owner: "author", Manifest: Manifest{Name: "echo", Title: "回声工具"}}
	s.store.State.Runs["run-1"] = Run{ID: "run-1", ToolID: "tool-1", Owner: "caller", Status: "queued", Input: map[string]any{"secret": "private-input"}, Created: time.Now()}
	s.store.State.Runs["run-2"] = Run{ID: "run-2", ToolID: "tool-1", Owner: "caller", Status: "succeeded", Created: time.Now().Add(-time.Minute)}
	for _, p := range []Principal{{}, {Admin: true}, {Session: true}} {
		w := httptest.NewRecorder()
		s.adminRunsEndpoint(w, httptest.NewRequest("GET", "/admin/runs", nil), p, []string{"admin", "runs"})
		if w.Code != 403 {
			t.Fatalf("non-admin session read monitor: %d", w.Code)
		}
		w = httptest.NewRecorder()
		s.adminToolsEndpoint(w, httptest.NewRequest("GET", "/admin/tools", nil), p, []string{"admin", "tools"})
		if w.Code != 403 {
			t.Fatalf("non-admin session read tool overview: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.adminToolsEndpoint(w, httptest.NewRequest("GET", "/admin/tools?search=echo", nil), admin, []string{"admin", "tools"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var tools struct {
		Data struct {
			Items []adminToolRow `json:"items"`
			Total int            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tools); err != nil || tools.Data.Total != 1 || tools.Data.Items[0].Statuses["queued"] != 1 {
		t.Fatalf("tool overview mismatch: %+v, %v", tools, err)
	}
	w = httptest.NewRecorder()
	s.adminRunsEndpoint(w, httptest.NewRequest("GET", "/admin/runs?status=queued&tool_id=tool-1", nil), admin, []string{"admin", "runs"})
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-input") {
		t.Fatalf("monitor list exposed run input: %d %s", w.Code, w.Body.String())
	}
	var runs struct {
		Data struct {
			Items []adminRunRow `json:"items"`
			Total int           `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &runs); err != nil || runs.Data.Total != 1 || runs.Data.Items[0].ID != "run-1" {
		t.Fatalf("filtered runs mismatch: %+v, %v", runs, err)
	}
}

func TestAdminTerminateQueuedRun(t *testing.T) {
	s := testServer(t)
	s.store.State.Runs["run-1"] = Run{ID: "run-1", Status: "queued", Created: time.Now()}
	w := httptest.NewRecorder()
	s.adminRunsEndpoint(w, httptest.NewRequest("POST", "/admin/runs/run-1/terminate", nil), Principal{Admin: true, Session: true}, []string{"admin", "runs", "run-1", "terminate"})
	if w.Code != 200 || s.store.State.Runs["run-1"].Status != "canceled" {
		t.Fatalf("queued run was not terminated: %d %s", w.Code, w.Body.String())
	}
	if next := s.claimRun(); next != nil {
		t.Fatalf("terminated run was claimed: %+v", next)
	}
}
