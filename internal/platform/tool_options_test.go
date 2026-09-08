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
		mw.WriteField("mode", "async")
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
	if tool.Owner != "alice" || tool.APIEnabled == nil || *tool.APIEnabled || tool.Manifest.Execution.Mode != "async" || !tool.Manifest.Network.Enabled {
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
	if strings.Contains(w.Body.String(), run.ID) {
		t.Fatal("ordinary completion generated a notification")
	}
	run.CallbackFailed = true
	s.store.State.Runs[run.ID] = run
	w = httptest.NewRecorder()
	s.notifications(w, httptest.NewRequest("GET", "/", nil), p, []string{"notifications"})
	if !strings.Contains(w.Body.String(), run.ID) {
		t.Fatal("missing callback failure notification")
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

func TestUploadManifestDefaultsAndExplicitOverrides(t *testing.T) {
	s := testServer(t)
	p := Principal{ID: "session", UserID: "alice", Session: true, Tools: []string{"*"}}
	upload := func(manifest string, fields map[string]string) Tool {
		var archive bytes.Buffer
		zw := zip.NewWriter(&archive)
		file, _ := zw.Create("tooldeck.json")
		file.Write([]byte(manifest))
		file, _ = zw.Create("main.py")
		file.Write([]byte("print('{}')"))
		zw.Close()

		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, _ := mw.CreateFormFile("file", "tool.zip")
		part.Write(archive.Bytes())
		for name, value := range fields {
			mw.WriteField(name, value)
		}
		mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/tools", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		s.uploadTool(w, r, p)
		if w.Code != 201 {
			t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Data Tool `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		return response.Data
	}

	base := strings.Replace(manifestJSON, `"name":"echo"`, `"name":"manifest-defaults"`, 1)
	base = strings.Replace(base, `"runtime":"python"`, `"runtime":"python","runtime_version":"3.11","build_command":"python -m compileall ."`, 1)
	base = strings.Replace(base, `"enabled":false,"allowed_hosts":[]`, `"enabled":true,"allowed_hosts":["manifest.example.com"]`, 1)
	tool := upload(base, nil)
	if tool.BuildStatus != "pending" || tool.ReviewStatus != "draft" {
		t.Fatalf("new upload must wait for author build and submission: %+v", tool)
	}
	if tool.Manifest.RuntimeVersion != "3.11" || tool.Manifest.BuildCommand != "python -m compileall ." || strings.Join(tool.Manifest.Network.AllowedHosts, ",") != "manifest.example.com" {
		t.Fatalf("manifest values were not preserved: %+v", tool.Manifest)
	}

	override := strings.Replace(base, `"name":"manifest-defaults"`, `"name":"manifest-overrides"`, 1)
	tool = upload(override, map[string]string{
		"runtime_version": "",
		"build_command":   "",
		"allowed_hosts":   "author.example.com, cdn.example.com",
	})
	if tool.Manifest.RuntimeVersion != "3.12" || tool.Manifest.BuildCommand != "" || strings.Join(tool.Manifest.Network.AllowedHosts, ",") != "author.example.com,cdn.example.com" {
		t.Fatalf("explicit overrides were not applied: %+v", tool.Manifest)
	}
}
