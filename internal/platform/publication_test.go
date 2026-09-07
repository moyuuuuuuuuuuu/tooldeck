package platform

import (
	"net/http/httptest"
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
