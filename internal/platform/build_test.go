package platform

import (
	"archive/tar"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorStartsPendingBuild(t *testing.T) {
	s := testServer(t)
	tool := Tool{ID: "pending", Owner: "alice", BuildStatus: "pending"}
	s.store.State.Tools[tool.ID] = tool
	w := httptest.NewRecorder()
	s.buildEndpoint(w, httptest.NewRequest("POST", "/", nil), Principal{UserID: "alice", Session: true}, tool.ID)
	if w.Code != 200 || s.store.State.Tools[tool.ID].BuildStatus != "queued" {
		t.Fatalf("pending build was not queued: %d %s", w.Code, w.Body.String())
	}
}

func TestBuildArtifactExtraction(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		var data bytes.Buffer
		w := tar.NewWriter(&data)
		name := "node_modules/pkg/main.js"
		if unsafe {
			name = "../outside"
		}
		w.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: 1})
		w.Write([]byte("x"))
		if !unsafe {
			w.WriteHeader(&tar.Header{Name: "node_modules/.bin/pkg", Typeflag: tar.TypeSymlink, Linkname: "../pkg/main.js", Mode: 0777})
		}
		w.Close()
		dest := filepath.Join(t.TempDir(), "artifact")
		e := extractBuild(&data, dest)
		if unsafe && e == nil {
			t.Fatal("traversal accepted")
		}
		if !unsafe {
			if e != nil {
				t.Fatal(e)
			}
			if b, e := os.ReadFile(filepath.Join(dest, "node_modules/.bin/pkg")); e != nil || string(b) != "x" {
				t.Fatal("safe dependency link broken", e)
			}
		}
	}
	var data bytes.Buffer
	w := tar.NewWriter(&data)
	w.WriteHeader(&tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "/etc"})
	w.Close()
	if extractBuild(&data, filepath.Join(t.TempDir(), "bad")) == nil {
		t.Fatal("absolute symlink accepted")
	}
}
func TestRuntimeVersions(t *testing.T) {
	for language, versions := range runtimeVersions {
		for _, v := range versions {
			if _, e := runtimeVersion(Manifest{Runtime: language, RuntimeVersion: v}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if _, e := runtimeVersion(Manifest{Runtime: "node", RuntimeVersion: "8.3"}); e == nil {
		t.Fatal("mismatched version accepted")
	}
}
