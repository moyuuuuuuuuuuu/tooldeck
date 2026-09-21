package platform

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationLifecycle(t *testing.T) {
	s := testServer(t)
	owner := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	other := Principal{UserID: "bob", Session: true, Tools: []string{"*"}}
	tool := Tool{ID: "mine", Owner: "alice", ReviewStatus: "approved", Manifest: Manifest{Name: "demo"}}
	s.store.State.Tools[tool.ID] = tool
	call := func(p Principal, action string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		s.publication(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"`+action+`"}`)), p, tool.ID)
		if w.Code != want {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	call(other, "withdraw", 403)
	key := owner
	key.Session = false
	call(key, "withdraw", 403)
	call(owner, "withdraw", 200)
	withdrawn := s.store.State.Tools[tool.ID]
	if canUseTool(other, withdrawn) || !canUseTool(owner, withdrawn) {
		t.Fatal("withdrawal access incorrect")
	}
	call(owner, "resubmit", 200)
	if s.store.State.Tools[tool.ID].ReviewStatus != "pending" {
		t.Fatal("review bypassed")
	}
	call(owner, "resubmit", 409)
	call(owner, "withdraw", 200)
	no := false
	s.store.State.ReviewRequired = &no
	call(owner, "resubmit", 200)
	if !canUseTool(other, s.store.State.Tools[tool.ID]) {
		t.Fatal("review disabled not honored")
	}
	s.store.State.Tools["foreign"] = Tool{ID: "foreign", Owner: "bob"}
	s.store.State.Tools["play"] = Tool{ID: "play", Owner: "alice", Playground: true}
	w := httptest.NewRecorder()
	s.myTools(w, httptest.NewRequest("GET", "/", nil), owner)
	if w.Code != 200 || strings.Contains(w.Body.String(), "foreign") || strings.Contains(w.Body.String(), `"id":"play"`) {
		t.Fatal(w.Body.String())
	}
	call(owner, "withdraw", 200)
	w = httptest.NewRecorder()
	s.reviewEndpoint(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"status":"approved"}`)), Principal{Admin: true}, []string{"reviews", tool.ID})
	if w.Code != 409 {
		t.Fatal("stale review accepted", w.Code)
	}
}

func TestDraftRequiresSuccessfulBuildBeforeSubmission(t *testing.T) {
	s := testServer(t)
	owner := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	yes := true
	tool := Tool{ID: "draft", Owner: "alice", Public: &yes, BuildStatus: "pending", ReviewStatus: "draft", Manifest: Manifest{Name: "demo"}}
	s.store.State.Tools[tool.ID] = tool
	call := func(path string, body string, want int) {
		w := httptest.NewRecorder()
		s.publication(w, httptest.NewRequest("POST", path, strings.NewReader(body)), owner, tool.ID)
		if w.Code != want {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	call("/", `{"action":"submit"}`, 409)
	tool = s.store.State.Tools[tool.ID]
	tool.BuildStatus = "ready"
	s.store.State.Tools[tool.ID] = tool
	call("/", `{"action":"submit"}`, 200)
	if s.store.State.Tools[tool.ID].ReviewStatus != "pending" {
		t.Fatal("successful build was not submitted for review")
	}
}

func TestDeleteToolLifecycle(t *testing.T) {
	s := testServer(t)
	owner := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	tool := Tool{ID: "mine", Owner: "alice", ReviewStatus: "approved", BuildStatus: "ready", Artifact: "artifact-1", Manifest: Manifest{Name: "demo"}}
	s.store.State.Tools[tool.ID] = tool
	call := func(want int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		s.deleteTool(w, httptest.NewRequest("DELETE", "/", nil), owner, tool.ID)
		if w.Code != want {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		return w
	}
	call(409)

	tool.ReviewStatus = "pending"
	s.store.State.Tools[tool.ID] = tool
	s.store.State.ToolEnv = map[string]map[string]string{"alice:demo:developer:alice": {"TOKEN": "encrypted"}}
	packageDir := filepath.Join(s.store.Root, "packages", tool.ID)
	artifactDir := filepath.Join(s.store.Root, "artifacts", tool.Artifact)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	call(200)
	if _, ok := s.store.State.Tools[tool.ID]; ok {
		t.Fatal("deleted review tool remained in store")
	}
	if len(s.store.State.ToolEnv) != 0 {
		t.Fatal("deleted tool environment remained in store")
	}
	for _, path := range []string{packageDir, artifactDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("tool data was not removed: %s", path)
		}
	}
}

func TestDeleteToolRejectsActiveRun(t *testing.T) {
	s := testServer(t)
	owner := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	tool := Tool{ID: "mine", Owner: "alice", ReviewStatus: "pending", BuildStatus: "ready", Manifest: Manifest{Name: "demo"}}
	s.store.State.Tools[tool.ID] = tool
	s.store.State.Runs["active"] = Run{ID: "active", ToolID: tool.ID, Status: "running"}
	w := httptest.NewRecorder()
	s.deleteTool(w, httptest.NewRequest("DELETE", "/", nil), owner, tool.ID)
	if w.Code != 409 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
