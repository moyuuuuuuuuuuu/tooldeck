package platform

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const manifestJSON = `{"schema_version":1,"name":"echo","version":"1.0.0","runtime":"python","entrypoint":"main.py","execution":{"mode":"sync","timeout_seconds":10,"memory_mb":128},"input_schema":{"type":"object","properties":{"text":{"type":"string","minLength":1}},"required":["text"]},"ui_schema":{},"network":{"enabled":false,"allowed_hosts":[]},"secrets":[]}`

func testServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("TOOLDECK_MASTER_KEY", strings.Repeat("ab", 32))
	t.Setenv("TOOLDECK_DATA_VOLUME", "")
	t.Setenv("TOOLDECK_OAUTH_INTROSPECTION_URL", "")
	s, e := New(t.TempDir(), "test-password-123456")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "a/../../escape", "a\\escape", "C:/escape"} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, _ := z.Create("tooldeck.json")
			w.Write([]byte(manifestJSON))
			w, _ = z.Create(name)
			w.Write([]byte("x"))
			z.Close()
			p := filepath.Join(t.TempDir(), "t.zip")
			os.WriteFile(p, b.Bytes(), 0600)
			if _, e := ExtractPackage(p, filepath.Join(t.TempDir(), "out")); e == nil {
				t.Fatal("accepted unsafe archive")
			}
		})
	}
}
func TestArchiveLinksAndDuplicate(t *testing.T) {
	for _, link := range []bool{true, false} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		w, _ := z.Create("tooldeck.json")
		w.Write([]byte(manifestJSON))
		if link {
			h := &zip.FileHeader{Name: "main.py"}
			h.SetMode(os.ModeSymlink | 0777)
			w, _ = z.CreateHeader(h)
			w.Write([]byte("/etc/passwd"))
		} else {
			for i := 0; i < 2; i++ {
				w, _ = z.Create("main.py")
				w.Write([]byte("print(1)"))
			}
		}
		z.Close()
		p := filepath.Join(t.TempDir(), "t.zip")
		os.WriteFile(p, b.Bytes(), 0600)
		if _, e := ExtractPackage(p, filepath.Join(t.TempDir(), "out")); e == nil {
			t.Fatal("accepted link or duplicate")
		}
	}
}
func TestInputValidation(t *testing.T) {
	var m Manifest
	json.Unmarshal([]byte(manifestJSON), &m)
	for _, v := range []map[string]any{{}, {"text": ""}, {"text": 3.0}, {"text": "ok", "extra": true}} {
		if validateInput(m.Input, v, "input") == nil {
			t.Fatalf("accepted invalid input %#v", v)
		}
	}
	if e := validateInput(m.Input, map[string]any{"text": "你好"}, "input"); e != nil {
		t.Fatal(e)
	}
}
func TestProxyRejectsPrivateAndUnlisted(t *testing.T) {
	for _, v := range []string{"127.0.0.1", "10.0.0.1", "192.168.31.192", "169.254.169.254", "100.100.0.1", "::1", "fc00::1", "198.18.0.1"} {
		if publicIP(net.ParseIP(v)) {
			t.Fatalf("accepted %s", v)
		}
	}
	for _, u := range []string{"http://127.0.0.1/", "http://example.net/"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", u, nil)
		egressProxy([]string{"example.com"}).ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal("proxy allowed invalid destination")
		}
	}
}
func TestAuthenticationAndScope(t *testing.T) {
	s := testServer(t)
	k := Credential{ID: "key1", Hash: hash("td_key_test"), Tools: []string{"echo"}, Expires: time.Now().Add(time.Hour)}
	s.store.State.Keys[k.ID] = k
	r := httptest.NewRequest("GET", "/api/v1/tools", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r.Header.Set("X-API-Key", "td_key_test")
	p, e := s.authenticate(r)
	if e != nil || !p.allows("echo") || p.allows("other") || p.Admin {
		t.Fatal("invalid credential scope")
	}
	r = httptest.NewRequest("GET", "/api/v1/keys", nil)
	r.Header.Set("X-API-Key", "td_key_test")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("API key gained admin")
	}
}
func TestSecretEncryptionAndRestart(t *testing.T) {
	s := testServer(t)
	c, e := s.encrypt("third-party-secret")
	if e != nil || strings.Contains(c, "third-party-secret") {
		t.Fatal(e)
	}
	v, e := s.decrypt(c)
	if e != nil || v != "third-party-secret" {
		t.Fatal(e)
	}
	s.store.State.Runs["test"] = Run{ID: "test", Status: "running"}
	if e = s.store.save(); e != nil {
		t.Fatal(e)
	}
	st, e := OpenStore(s.store.Root)
	if e != nil || st.State.Runs["test"].Status != "failed" {
		t.Fatal("restart replayed side effect")
	}
}
func TestForeignFileDenied(t *testing.T) {
	s := testServer(t)
	s.store.State.Files["file_x"] = File{ID: "file_x", Size: 3, MIME: "image/png"}
	s.store.State.Owners["file_x"] = "key1"
	sc := Schema{Type: "string", Format: "tooldeck-file"}
	if s.validateFileInput(sc, "file_x", "key2", nil, "") == nil {
		t.Fatal("foreign file accepted")
	}
}
func TestIdempotencyAndCancel(t *testing.T) {
	s := testServer(t)
	var m Manifest
	json.Unmarshal([]byte(manifestJSON), &m)
	m.Execution.Mode = "async"
	s.store.State.Tools["tool1"] = Tool{ID: "tool1", Manifest: m}
	s.store.State.Keys["key1"] = Credential{ID: "key1", Hash: hash("td_test"), Tools: []string{"echo"}, Expires: time.Now().Add(time.Hour)}
	call := func(text string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/tools/tool1/runs", strings.NewReader(`{"input":{"text":"`+text+`"}}`))
		r.Header.Set("X-API-Key", "td_test")
		r.Header.Set("Idempotency-Key", "same")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if call("hello").Code != 202 || call("hello").Code != 202 || len(s.store.State.Runs) != 1 {
		t.Fatal("duplicate execution")
	}
	if call("different").Code != 409 {
		t.Fatal("idempotency conflict not detected")
	}
	for id := range s.store.State.Runs {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/runs/"+id+"/cancel", nil)
		r.Header.Set("X-API-Key", "td_test")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 || s.store.State.Runs[id].Status != "canceled" {
			t.Fatal("cancel failed")
		}
	}
}
