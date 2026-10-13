package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlaygroundEnvironmentFailureUsesBookwormAndRetainsLogs(t *testing.T) {
	s := testServer(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$1\" = build ]; then printf '%s\\n' \"$*\" >&2; echo 'repository unavailable' >&2; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	tool := Tool{ID: "play-review", Owner: "admin", BuildStatus: "building", Playground: true, Manifest: Manifest{Runtime: "php", RuntimeVersion: "8.1", Entrypoint: "main.php"}}
	s.store.State.Tools[tool.ID] = tool
	s.buildTool(context.Background(), tool)
	result := s.store.State.Tools[tool.ID]
	if result.BuildStatus != "failed" || result.BuildFailureStage != "environment" {
		t.Fatalf("wrong failure: %+v", result)
	}
	if !strings.Contains(result.BuildLog, "BASE_IMAGE=php:8.1-cli-bookworm") || !strings.Contains(result.BuildLog, "repository unavailable") {
		t.Fatalf("missing build choice or diagnostic: %s", result.BuildLog)
	}
	if !strings.Contains(result.BuildError, "重试") {
		t.Fatal("missing retry direction")
	}
}

func TestJSONInputReportsLineAndColumn(t *testing.T) {
	schema := Schema{Type: "string", Format: "json"}
	err := validateInput(schema, "{\n  \"a\":\n}", "input.text")
	if err == nil || !strings.Contains(err.Error(), "第 3 行，第 1 列") {
		t.Fatalf("missing position: %v", err)
	}
	for _, valid := range []string{`{"a":1}`, `{"big":1e999}`, `null`} {
		if err := validateInput(schema, valid, "input.text"); err != nil {
			t.Fatalf("valid JSON rejected: %v", err)
		}
	}
}

func TestRunHistoryFilterTitlesPaginationAndIsolation(t *testing.T) {
	s := testServer(t)
	login := httptest.NewRecorder()
	s.Handler().ServeHTTP(login, httptest.NewRequest("POST", "/api/core/login", strings.NewReader(`{"username":"admin","password":"test-password-123456"}`)))
	var auth struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &auth); err != nil || auth.Data.AccessToken == "" {
		t.Fatalf("login: %s", login.Body.String())
	}
	s.store.State.Tools["tool"] = Tool{ID: "tool", Manifest: Manifest{Title: "JSON 格式化"}}
	now := time.Now()
	s.store.State.Runs["ok"] = Run{ID: "ok", ToolID: "tool", Owner: "admin", Status: "succeeded", Created: now}
	s.store.State.Runs["error"] = Run{ID: "error", ToolID: "tool", Owner: "admin", Status: "failed", Created: now.Add(-time.Second)}
	s.store.State.Runs["foreign"] = Run{ID: "foreign", ToolID: "tool", Owner: "someone", Status: "succeeded", Created: now}
	s.store.State.Runs["deleted"] = Run{ID: "deleted", ToolID: "gone", ToolTitle: "历史工具", Owner: "admin", Status: "succeeded", Created: now.Add(-2 * time.Second)}
	req := httptest.NewRequest("GET", "/api/v1/runs?mine=1&status=succeeded&page=1&page_size=1", nil)
	req.Header.Set("Authorization", "Bearer "+auth.Data.AccessToken)
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, req)
	var page struct {
		Data struct {
			Items []Run `json:"items"`
			Total int   `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Data.Total != 2 || len(page.Data.Items) != 1 || page.Data.Items[0].ID != "ok" || page.Data.Items[0].ToolTitle != "JSON 格式化" {
		t.Fatalf("bad filtered page: %s", response.Body.String())
	}
	req = httptest.NewRequest("GET", "/api/v1/runs?mine=1&status=succeeded&page=2&page_size=1", nil)
	req.Header.Set("Authorization", "Bearer "+auth.Data.AccessToken)
	response = httptest.NewRecorder()
	s.Handler().ServeHTTP(response, req)
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 1 || page.Data.Items[0].ToolTitle != "历史工具" {
		t.Fatalf("deleted title lost: %s", response.Body.String())
	}
}
