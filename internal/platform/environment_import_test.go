package platform

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyEnvironmentImportIsScoped(t *testing.T) {
	s := testServer(t)
	s.store.State.Tools["t"] = Tool{ID: "t", Owner: "alice", Manifest: Manifest{Name: "one", EnvMode: "user", Env: []EnvField{{Name: "API_KEY", Required: true}}}}
	cipher, _ := s.encrypt("legacy-value")
	s.store.State.AccountEnv = map[string]map[string]string{"alice": {"API_KEY": cipher}}
	p := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	call := func() int {
		w := httptest.NewRecorder()
		s.accountEnvironment(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"copy_to_tool":"t","import_keys":["API_KEY"]}`)), p)
		return w.Code
	}
	if call() != 200 {
		t.Fatal("import failed")
	}
	tool := s.store.State.Tools["t"]
	env, _, err := s.toolEnvironment(tool, "alice")
	if err != nil || env[0] != "API_KEY=legacy-value" {
		t.Fatal(env, err)
	}
	tool.Manifest.Name = "two"
	if _, _, err = s.toolEnvironment(tool, "alice"); err == nil {
		t.Fatal("cross tool leaked")
	}
	if call() != 409 {
		t.Fatal("overwrote personal value")
	}
	if s.store.State.AccountEnv["alice"]["API_KEY"] != cipher {
		t.Fatal("legacy value destroyed")
	}
}
