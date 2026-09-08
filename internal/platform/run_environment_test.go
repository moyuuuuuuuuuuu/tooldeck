package platform

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEnvironmentOverrideIsEncryptedAndScoped(t *testing.T) {
	s := testServer(t)
	yes := true
	tool := Tool{ID: "api-env", Owner: "author", Public: &yes, ReviewStatus: "approved", BuildStatus: "ready", APIEnabled: &yes, Manifest: Manifest{Name: "api-env", EnvMode: "user", Env: []EnvField{{Name: "API_KEY", Required: true}}, Input: Schema{Type: "object", Properties: map[string]Schema{}}}}
	tool.Manifest.Execution.Mode = "async"
	tool.Manifest.Execution.Timeout = 30
	s.store.State.Tools[tool.ID] = tool
	p := Principal{ID: "key", UserID: "bob", Tools: []string{"*"}}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"input":{},"env":{"API_KEY":"one-time-secret"},"callback_url":"https://callback.example/result"}`))
	s.createRun(w, r, p, tool.ID)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var runID string
	for id, run := range s.store.State.Runs {
		runID = id
		if run.Source != "api_key" || !run.EnvOverridden {
			t.Fatalf("unexpected source metadata: %+v", run)
		}
	}
	if runID == "" || s.store.State.RunEnv[runID]["API_KEY"] == "one-time-secret" {
		t.Fatal("run environment was not encrypted")
	}
	data, _ := os.ReadFile(filepath.Join(s.store.Root, "state.json"))
	if strings.Contains(string(data), "one-time-secret") || strings.Contains(w.Body.String(), "one-time-secret") {
		t.Fatal("run environment leaked")
	}
	values, _, err := s.toolEnvironmentWithOverrides(tool, "bob", map[string]string{"API_KEY": "override"})
	if err != nil || len(values) != 1 || values[0] != "API_KEY=override" {
		t.Fatal(values, err)
	}
}
