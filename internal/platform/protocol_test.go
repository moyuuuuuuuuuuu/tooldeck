package platform

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExecutionArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, name := range []string{"../escape", "output/../../escape", "/etc/passwd", "output/link"} {
		var b bytes.Buffer
		tw := tar.NewWriter(&b)
		h := &tar.Header{Name: name, Mode: 0644, Size: 1, Typeflag: tar.TypeReg}
		if name == "output/link" {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = "/etc/passwd"
			h.Size = 0
		}
		tw.WriteHeader(h)
		if h.Size != 0 {
			tw.Write([]byte("x"))
		}
		tw.Close()
		if _, e := unpackExecution(b.Bytes(), t.TempDir()); e == nil {
			t.Fatal("unsafe execution archive accepted")
		}
	}
}
func TestOAuthAudienceExpiryAndScope(t *testing.T) {
	s := testServer(t)
	aud := "tooldeck"
	scope := "tool:echo"
	expiry := time.Now().Add(time.Hour).Unix()
	active := true
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "client" || password != "secret" {
			t.Error("missing client authentication")
		}
		if r.Method != "POST" || r.FormValue("token") != "external-token" {
			t.Error("invalid introspection request")
		}
		json.NewEncoder(w).Encode(map[string]any{"active": active, "aud": aud, "sub": "external-user", "scope": scope, "exp": expiry})
	}))
	defer endpoint.Close()
	previous := http.DefaultTransport
	http.DefaultTransport = endpoint.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	s.oauthURL = endpoint.URL
	t.Setenv("TOOLDECK_OAUTH_CLIENT_ID", "client")
	t.Setenv("TOOLDECK_OAUTH_CLIENT_SECRET", "secret")
	t.Setenv("TOOLDECK_OAUTH_AUDIENCE", "tooldeck")
	r := httptest.NewRequest("GET", "/api/v1/tools", nil)
	r.Header.Set("Authorization", "Bearer external-token")
	p, e := s.authenticate(r)
	if e != nil || p.Admin || !p.allows("echo") || p.allows("other") {
		t.Fatal("valid OAuth scope failed", e)
	}
	aud = "other"
	if _, e = s.authenticate(r); e == nil {
		t.Fatal("wrong audience accepted")
	}
	aud = "tooldeck"
	expiry = 1
	if _, e = s.authenticate(r); e == nil {
		t.Fatal("expired token accepted")
	}
	expiry = time.Now().Add(time.Hour).Unix()
	scope = "profile email"
	if _, e = s.authenticate(r); e == nil {
		t.Fatal("missing tool scope accepted")
	}
	scope = "tool:echo"
	active = false
	if _, e = s.authenticate(r); e == nil {
		t.Fatal("inactive token accepted")
	}
}
func TestManifestRejectsUnknownSchemaKeywords(t *testing.T) {
	var m Manifest
	d := json.NewDecoder(strings.NewReader(strings.Replace(manifestJSON, `"minLength":1`, `"minLength":1,"$ref":"https://example.com/schema"`, 1)))
	d.DisallowUnknownFields()
	if d.Decode(&m) == nil {
		t.Fatal("remote schema reference accepted")
	}
}
