package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserSerializesSameToolWhileAPIStaysConcurrent(t *testing.T) {
	s := testServer(t)
	var manifest Manifest
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Execution.Mode = "async"
	s.store.State.Tools["v1"] = Tool{ID: "v1", Owner: "caller", Manifest: manifest, BuildStatus: "ready"}
	manifest.Version = "2.0.0"
	s.store.State.Tools["v2"] = Tool{ID: "v2", Owner: "caller", Manifest: manifest, BuildStatus: "ready"}
	web := Principal{Session: true, UserID: "caller", Tools: []string{"*"}}
	api := Principal{ID: "key", UserID: "caller", Tools: []string{"*"}}
	create := func(p Principal, id string) *httptest.ResponseRecorder {
		t.Helper()
		body := `{"input":{"text":"hello"}}`
		if !p.Session {
			body = `{"input":{"text":"hello"},"callback_url":"https://example.com/hook"}`
		}
		r := httptest.NewRequest("POST", "/api/v1/tools/"+id+"/runs", strings.NewReader(body))
		w := httptest.NewRecorder()
		s.createRun(w, r, p, id)
		return w
	}
	if w := create(web, "v1"); w.Code != 202 {
		t.Fatalf("first browser run: %d %s", w.Code, w.Body.String())
	}
	if w := create(web, "v2"); w.Code != 409 {
		t.Fatalf("browser bypassed family limit: %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		if w := create(api, "v1"); w.Code != 202 {
			t.Fatalf("API run %d was blocked: %d %s", i, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.browserActiveRun(w, httptest.NewRequest("GET", "/api/v1/tools/v1/active-run", nil), web, "v1")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"source":"web"`) {
		t.Fatalf("browser run not restored: %d %s", w.Code, w.Body.String())
	}
	s.store.Lock()
	for id, run := range s.store.State.Runs {
		if run.Source == "web" {
			run.Status = "succeeded"
			s.store.State.Runs[id] = run
		}
	}
	s.store.Unlock()
	if w := create(web, "v2"); w.Code != 202 {
		t.Fatalf("browser still blocked after completion: %d %s", w.Code, w.Body.String())
	}
}
