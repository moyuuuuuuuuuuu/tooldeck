package platform

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEnvironmentSourceMetadata(t *testing.T) {
	s := testServer(t)
	tool := Tool{ID: "source", Owner: "author", Manifest: Manifest{Name: "source", Env: []EnvField{{Name: "API_KEY"}}}}
	s.store.State.Tools[tool.ID] = tool
	s.store.State.AccountEnv = map[string]map[string]string{"alice": {"API_KEY": "encrypted"}}
	s.store.State.ToolEnv = map[string]map[string]string{}
	p := Principal{UserID: "alice", Admin: true, Session: true, Tools: []string{"*"}}
	source := func() string {
		w := httptest.NewRecorder()
		s.environmentEndpoint(w, httptest.NewRequest("GET", "/", nil), p, tool.ID)
		var body struct {
			Data struct {
				Fields []map[string]any `json:"fields"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		f := body.Data.Fields[0]
		if _, ok := f["value"]; ok {
			t.Fatal("value disclosed")
		}
		return f["source"].(string)
	}
	if source() != "none" {
		t.Fatal("legacy account value must not auto-inject")
	}
	personal := tool
	personal.Manifest.EnvMode = "user"
	s.store.State.ToolEnv[envKey(personal, "alice")] = map[string]string{"API_KEY": "override"}
	if source() != "personal" {
		t.Fatal("personal precedence")
	}
	delete(s.store.State.ToolEnv, envKey(personal, "alice"))
	delete(s.store.State.AccountEnv, "alice")
	shared := tool
	shared.Manifest.EnvMode = "developer"
	s.store.State.ToolEnv[envKey(shared, "author")] = map[string]string{"API_KEY": "shared"}
	if source() != "shared" {
		t.Fatal("shared fallback")
	}
	tool.Manifest.EnvMode = "user"
	s.store.State.Tools[tool.ID] = tool
	if source() != "none" {
		t.Fatal("shared mode bypass")
	}
}
