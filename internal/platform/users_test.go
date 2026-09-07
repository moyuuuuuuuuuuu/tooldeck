package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUserAccountsAndIsolation(t *testing.T) {
	s := testServer(t)
	s.mailSender = func(email, code string) error { return nil }
	s.store.State.EmailCodes = map[string]EmailCode{}
	for _, name := range []string{"alice", "bob"} {
		s.store.State.EmailCodes[name+"@example.com"] = EmailCode{Hash: emailCodeHash(name+"@example.com", "123456"), Expires: time.Now().Add(time.Minute)}
	}
	call := func(method, path, token, body string, status int) map[string]any {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	login := func(name, password string) string {
		return call("POST", "/api/core/login", "", `{"username":"`+name+`","password":"`+password+`"}`, 200)["data"].(map[string]any)["access_token"].(string)
	}
	call("POST", "/api/core/register", "", `{"email":"alice@example.com","code":"123456","password":"test-password-123456","nickname":"Alice"}`, 201)
	call("POST", "/api/core/register", "", `{"email":"alice@example.com","code":"123456","username":"ALICE","password":"test-password-123456"}`, 409)
	call("POST", "/api/core/register", "", `{"email":"bob@example.com","code":"123456","username":"bob","password":"test-password-123456"}`, 201)
	a, b := login("ALICE@EXAMPLE.COM", "test-password-123456"), login("bob@example.com", "test-password-123456")
	info := call("GET", "/api/core/system/user", a, "", 200)["data"].(map[string]any)
	uid := info["id"].(string)
	if strings.Contains(stringMustJSON(info), "password_hash") {
		t.Fatal("password hash exposed")
	}
	call("PATCH", "/api/core/system/user", a, `{"nickname":"新昵称","email":"alice@example.com","bio":"hello"}`, 200)
	call("POST", "/api/v1/tools", a, "", 400)
	call("GET", "/api/v1/secrets", a, "", 403)
	call("GET", "/api/v1/nodes", a, "", 403)
	key := call("POST", "/api/v1/keys", a, `{"name":"personal","tools":["echo"],"days":1}`, 201)["data"].(map[string]any)
	call("DELETE", "/api/v1/keys/"+key["id"].(string), b, "", 404)
	call("GET", "/api/core/system/user", key["key"].(string), "", 403)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", key["key"].(string))
	p, e := s.authenticate(req)
	if e != nil || p.owner() != uid || p.Admin {
		t.Fatal("API key identity mismatch", p, e)
	}
	s.store.Lock()
	s.store.State.Tools["tool"] = Tool{ID: "tool", Manifest: Manifest{Name: "echo", Title: "Echo", Version: "1"}}
	s.store.State.Runs["owned"] = Run{ID: "owned", ToolID: "tool", Owner: uid, Status: "succeeded", Created: time.Now()}
	s.store.State.Runs["foreign"] = Run{ID: "foreign", ToolID: "tool", Owner: "admin", Status: "failed", Created: time.Now()}
	s.store.State.Owners["private"] = uid
	s.store.State.Files["private"] = File{ID: "private"}
	s.store.Unlock()
	call("GET", "/api/v1/runs/owned", b, "", 404)
	call("GET", "/api/v1/files/private", b, "", 404)
	stats := call("GET", "/api/core/account/stats", a, "", 200)["data"].(map[string]any)
	if stats["summary"].(map[string]any)["total"] != float64(1) {
		t.Fatal(stats)
	}
	call("POST", "/api/core/account/password", a, `{"current_password":"wrong","password":"replacement-password"}`, 422)
	call("POST", "/api/core/account/password", a, `{"current_password":"test-password-123456","password":"replacement-password"}`, 200)
	call("GET", "/api/core/system/user", a, "", 401)
	login("alice@example.com", "replacement-password")
	// API keys survive password changes, as promised in the account UI.
	if _, e = s.authenticate(req); e != nil {
		t.Fatal(e)
	}
	st, e := OpenStore(s.store.Root)
	if e != nil {
		t.Fatal(e)
	}
	if e = bootstrapUser(st, "different-env-password"); e != nil {
		t.Fatal(e)
	}
	if !verifyPassword(st.State.Users["admin"].PasswordHash, "test-password-123456") {
		t.Fatal("bootstrap overwrote existing password")
	}
}
func stringMustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
