package platform

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPersonalToolOptions(t *testing.T) {
	s := testServer(t)
	p := Principal{ID: "session", UserID: "alice", Session: true, Tools: []string{"*"}}
	var z bytes.Buffer
	zw := zip.NewWriter(&z)
	f, _ := zw.Create("tooldeck.json")
	f.Write([]byte(manifestJSON))
	f, _ = zw.Create("main.py")
	f.Write([]byte("print('{}')"))
	zw.Close()
	upload := func(owner Principal, name string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		file, _ := mw.CreateFormFile("file", "tool.zip")
		file.Write(z.Bytes())
		mw.WriteField("third_party", "true")
		mw.WriteField("allowed_hosts", "api.example.com")
		mw.WriteField("notify_result", "true")
		mw.WriteField("api_enabled", "false")
		mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/tools", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		s.uploadTool(w, r, owner)
		return w
	}
	w := upload(p, "echo")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		Data Tool `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	tool := response.Data
	if tool.Owner != "alice" || tool.APIEnabled == nil || *tool.APIEnabled || !tool.Notify || tool.Manifest.Execution.Mode != "async" || !tool.Manifest.Network.Enabled {
		t.Fatal(tool)
	}
	foreign := Principal{UserID: "bob", Session: true, Tools: []string{"*"}}
	if canUseTool(foreign, tool) {
		t.Fatal("foreign user can access personal tool")
	}
	if w := upload(foreign, "echo"); w.Code != 409 {
		t.Fatal("tool name takeover accepted", w.Code)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"input":{"text":"hello"}}`))
	w = httptest.NewRecorder()
	s.createRun(w, r, Principal{ID: "key", UserID: "alice", Tools: []string{"echo"}}, tool.ID)
	if w.Code != 403 {
		t.Fatal("disabled API accepted", w.Code)
	}
	s.store.Lock()
	ready := s.store.State.Tools[tool.ID]
	ready.BuildStatus = "ready"
	s.store.State.Tools[tool.ID] = ready
	s.store.Unlock()
	r = httptest.NewRequest("POST", "/", strings.NewReader(`{"input":{"text":"hello"}}`))
	w = httptest.NewRecorder()
	s.createRun(w, r, p, tool.ID)
	if w.Code != 202 {
		t.Fatal("web execution rejected", w.Code, w.Body.String())
	}
	var result struct {
		Data Run `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	run := result.Data
	run.Status = "succeeded"
	run.Created = time.Now()
	s.store.State.Runs[run.ID] = run
	w = httptest.NewRecorder()
	s.notifications(w, httptest.NewRequest("GET", "/", nil), p, []string{"notifications"})
	if !strings.Contains(w.Body.String(), run.ID) {
		t.Fatal("missing completion notification")
	}
	w = httptest.NewRecorder()
	s.notifications(w, httptest.NewRequest("GET", "/", nil), foreign, []string{"notifications"})
	if strings.Contains(w.Body.String(), run.ID) {
		t.Fatal("notification leaked")
	}
	w = httptest.NewRecorder()
	s.notifications(w, httptest.NewRequest("POST", "/", nil), p, []string{"notifications", run.ID})
	if w.Code != 200 || !s.store.State.Runs[run.ID].NotificationRead {
		t.Fatal("notification read not persisted")
	}
}
