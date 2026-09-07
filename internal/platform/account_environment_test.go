package platform

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountEnvironment(t *testing.T) {
	s := testServer(t)
	p := Principal{UserID: "alice", Session: true}
	w := httptest.NewRecorder()
	s.accountEnvironment(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"values":{"API_KEY":"account-secret"}}`)), p)
	if w.Code != 200 || strings.Contains(w.Body.String(), "account-secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	tool := Tool{Owner: "author", Manifest: Manifest{Name: "test", EnvMode: "user", Env: []EnvField{{Name: "API_KEY", Required: true}}}}
	env, _, err := s.toolEnvironment(tool, "alice")
	if err == nil {
		t.Fatal(env, err)
	}
	if _, _, err = s.toolEnvironment(tool, "bob"); err == nil {
		t.Fatal("cross-user leak")
	}
	cipher, _ := s.encrypt("override")
	s.store.State.ToolEnv = map[string]map[string]string{envKey(tool, "alice"): {"API_KEY": cipher}}
	env, _, err = s.toolEnvironment(tool, "alice")
	if err != nil || env[0] != "API_KEY=override" {
		t.Fatal(env, err)
	}
	other := tool
	other.Manifest.Name = "another-tool"
	if _, _, e := s.toolEnvironment(other, "alice"); e == nil {
		t.Fatal("cross-tool leak")
	}
	tool.Manifest.Env = nil
	env, _, err = s.toolEnvironment(tool, "alice")
	if err != nil || len(env) != 0 {
		t.Fatal("undeclared env injected")
	}
	w = httptest.NewRecorder()
	s.accountEnvironment(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"values":{"PATH":"bad"}}`)), p)
	if w.Code != 422 {
		t.Fatal("reserved accepted")
	}
	w = httptest.NewRecorder()
	s.accountEnvironment(w, httptest.NewRequest("GET", "/", nil), Principal{UserID: "alice"})
	if w.Code != 403 {
		t.Fatal("non-session access")
	}
}
