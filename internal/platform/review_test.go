package platform

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewVisibilityAndSettings(t *testing.T) {
	s := testServer(t)
	yes, no := true, false
	owner := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	other := Principal{UserID: "bob", Session: true, Tools: []string{"*"}}
	admin := Principal{Admin: true, Session: true, UserID: "admin"}
	tool := Tool{ID: "reviewed", Owner: "alice", Public: &yes, ReviewStatus: "pending", Manifest: Manifest{Name: "echo"}}
	if canUseTool(other, tool) || !canUseTool(owner, tool) {
		t.Fatal("pending visibility incorrect")
	}
	s.store.State.Tools[tool.ID] = tool
	s.store.State.Users["alice"] = User{ID: "alice", Username: "alice@example.com", Nickname: "Alice", Email: "alice@example.com", EmailVerified: true}
	w := httptest.NewRecorder()
	s.reviewEndpoint(w, httptest.NewRequest("GET", "/", nil), admin, []string{"reviews"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"nickname":"Alice"`) || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatal("review list author information is missing or unsafe", w.Body.String())
	}
	call := func(p Principal, path, body string, parts []string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		s.reviewEndpoint(w, httptest.NewRequest("POST", path, strings.NewReader(body)), p, parts)
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	call(other, "/", `{"status":"approved"}`, []string{"reviews", tool.ID}, 403)
	draft := tool
	draft.ID = "draft"
	draft.ReviewStatus = "draft"
	s.store.State.Tools[draft.ID] = draft
	call(admin, "/", `{"status":"approved"}`, []string{"reviews", draft.ID}, 409)
	call(admin, "/", `{"status":"approved"}`, []string{"reviews", tool.ID}, 200)
	if !canUseTool(other, s.store.State.Tools[tool.ID]) {
		t.Fatal("approved public tool hidden")
	}
	call(admin, "/", `{"status":"rejected","note":"requires changes"}`, []string{"reviews", tool.ID}, 200)
	if canUseTool(other, s.store.State.Tools[tool.ID]) {
		t.Fatal("rejected tool visible")
	}
	tool.Public = &no
	tool.ReviewStatus = "approved"
	if canUseTool(other, tool) || !canUseTool(owner, tool) {
		t.Fatal("private visibility incorrect")
	}
	call(admin, "/", `{"required":false}`, []string{"review-settings"}, 200)
	st, e := OpenStore(s.store.Root)
	if e != nil || st.State.ReviewRequired == nil || *st.State.ReviewRequired {
		t.Fatal("review switch not persisted", e)
	}
	if s.store.State.Tools["reviewed"].ReviewStatus != "rejected" {
		t.Fatal("setting modified existing review status")
	}
}
