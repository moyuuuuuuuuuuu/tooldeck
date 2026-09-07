package platform

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolEnvironmentIsolation(t *testing.T) {
	s := testServer(t)
	yes := true
	tool := Tool{ID: "envtool", Owner: "alice", Public: &yes, ReviewStatus: "approved", Manifest: Manifest{Name: "envtest", Env: []EnvField{{Name: "API_KEY", Required: true, Sensitive: true}}}}
	s.store.State.Tools[tool.ID] = tool
	alice := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	bob := Principal{UserID: "bob", Session: true, Tools: []string{"*"}}
	call := func(p Principal, method, body string, want int) string {
		t.Helper()
		w := httptest.NewRecorder()
		s.environmentEndpoint(w, httptest.NewRequest(method, "/?scope="+func() string {
			if p.UserID == "alice" {
				return "shared"
			}
			return "personal"
		}(), strings.NewReader(body)), p, tool.ID)
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret-value") {
			t.Fatal("secret returned")
		}
		return w.Body.String()
	}
	if _, _, e := s.toolEnvironment(tool, "alice"); e == nil {
		t.Fatal("missing required accepted")
	}
	call(bob, "POST", `{"values":{"API_KEY":"bad"}}`, 200)
	valuesBefore, _, errBefore := s.toolEnvironment(tool, "bob")
	if errBefore != nil || valuesBefore[0] != "API_KEY=bad" {
		t.Fatal("personal override ignored")
	}
	call(bob, "POST", `{"delete":["API_KEY"]}`, 200)
	call(alice, "POST", `{"values":{"API_KEY":"secret-value"}}`, 200)
	values, _, e := s.toolEnvironment(tool, "bob")
	if e != nil || values[0] != "API_KEY=secret-value" {
		t.Fatal(values, e)
	}
	call(bob, "GET", "", 200)
	data, _ := os.ReadFile(filepath.Join(s.store.Root, "state.json"))
	if strings.Contains(string(data), "secret-value") {
		t.Fatal("plaintext on disk")
	}
	version := tool
	version.ID = "new-version"
	values, _, e = s.toolEnvironment(version, "bob")
	if e != nil || len(values) != 1 {
		t.Fatal("cross-version config lost")
	}
	call(alice, "POST", `{"mode":"user"}`, 200)
	tool = s.store.State.Tools[tool.ID]
	if _, _, e = s.toolEnvironment(tool, "alice"); e == nil {
		t.Fatal("shared config leaked into personal mode")
	}
	call(bob, "POST", `{"values":{"API_KEY":"bob-value"}}`, 200)
	values, _, e = s.toolEnvironment(tool, "bob")
	if e != nil || values[0] != "API_KEY=bob-value" {
		t.Fatal(values, e)
	}
	if _, _, e = s.toolEnvironment(tool, "alice"); e == nil {
		t.Fatal("personal config crossed user boundary")
	}
	call(bob, "POST", `{"fields":[]}`, 403)
	call(alice, "POST", `{"fields":[{"name":"HTTP_PROXY"}]}`, 422)
	call(bob, "POST", `{"delete":["API_KEY"]}`, 200)
	if _, _, e = s.toolEnvironment(tool, "bob"); e == nil {
		t.Fatal("deleted value still present")
	}
}
