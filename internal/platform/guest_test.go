package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGuestIsolation(t *testing.T) {
	s := testServer(t)
	tool := Tool{ID: "public", Owner: "alice", ReviewStatus: "approved", BuildStatus: "ready", BuildLog: "private-build-log", Manifest: Manifest{Name: "public"}}
	tool.Manifest.Execution.Mode = "async"
	tool.Manifest.Input = Schema{Type: "object"}
	s.store.State.Tools[tool.ID] = tool
	for _, kind := range []string{"env", "secret", "private", "pending", "rejected", "withdrawn", "building", "playground"} {
		x := tool
		x.ID = kind
		switch kind {
		case "env":
			x.Manifest.Env = []EnvField{{Name: "API_KEY"}}
		case "secret":
			x.Manifest.Secrets = []string{"API_KEY"}
		case "private":
			v := false
			x.Public = &v
		case "pending", "rejected":
			x.ReviewStatus = kind
		case "withdrawn":
			x.Withdrawn = true
		case "building":
			x.BuildStatus = "building"
		case "playground":
			x.Playground = true
		}
		s.store.State.Tools[kind] = x
	}
	call := func(method, path, body string, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("Referer", "http://example.com/explore")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	w := call("GET", "/api/public/tools", "", nil, 200)
	if strings.Contains(w.Body.String(), "private-build-log") || !strings.Contains(w.Body.String(), `"id":"env"`) {
		t.Fatal("private data exposed")
	}
	var list struct{ Data []Tool }
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Data) != 3 {
		t.Fatal("unexpected public list", w.Body.String())
	}
	// Discovery must hide withdrawn tools even from their author/admin.
	s.store.State.Keys["admin-session"] = Credential{Session: true, UserID: "admin", Hash: hash("test-session"), Expires: time.Now().Add(time.Hour)}
	adminRequest := httptest.NewRequest("GET", "/api/v1/tools", nil)
	adminRequest.Header.Set("Authorization", "Bearer test-session")
	adminResponse := httptest.NewRecorder()
	s.Handler().ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != 200 || strings.Contains(adminResponse.Body.String(), `"id":"withdrawn"`) {
		t.Fatal("withdrawn tool exposed on homepage")
	}
	cookie := w.Result().Cookies()[0]
	for _, id := range []string{"env", "secret", "private", "pending", "rejected", "withdrawn", "building", "playground"} {
		call("POST", "/api/public/tools/"+id+"/runs", `{"input":{}}`, cookie, 404)
	}
	w = call("POST", "/api/public/tools/public/runs", `{"input":{}}`, cookie, 202)
	var run struct{ Data Run }
	json.Unmarshal(w.Body.Bytes(), &run)
	call("GET", "/api/public/runs/"+run.Data.ID, "", cookie, 200)
	call("GET", "/api/public/runs/"+run.Data.ID, "", nil, 404)
	call("GET", "/api/public/runs/"+run.Data.ID+"/events", "", nil, 404)
	call("GET", "/api/public/tools/public/environment", "", cookie, 404)
	call("GET", "/api/v1/tools", "", cookie, 401)
	call("POST", "/api/v1/tools/public/runs", `{"input":{}}`, nil, 401)
	call("POST", "/api/v1/tools/public/runs", `{"input":{}}`, cookie, 401)
	r := httptest.NewRequest("POST", "/api/public/tools/public/runs", strings.NewReader(`{"input":{}}`))
	r.Header.Set("Origin", "https://foreign.example")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site write allowed")
	}
}
