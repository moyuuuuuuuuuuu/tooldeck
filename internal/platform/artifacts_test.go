package platform

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func addTestArtifact(t *testing.T, s *Server, expires time.Time) File {
	t.Helper()
	run := Run{ID: "run_artifact", Owner: "alice", Status: "succeeded", Created: time.Now()}
	file := File{ID: "file_artifact", Name: "result.mp4", MIME: "video/mp4", Size: 5, Storage: "local", RunID: run.ID, ExpiresAt: expires, DownloadsRemaining: 3}
	run.Artifacts = []File{file}
	s.store.State.Runs[run.ID] = run
	s.store.State.Files[file.ID] = file
	s.store.State.Owners[file.ID] = run.Owner
	if err := os.WriteFile(filepath.Join(s.store.Root, "files", file.ID), []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRunArtifactExpiresAfterThreeDownloads(t *testing.T) {
	s := testServer(t)
	file := addTestArtifact(t, s, time.Now().Add(time.Hour))
	p := Principal{UserID: "alice", Session: true, Admin: true}
	for remaining := 2; remaining >= 0; remaining-- {
		w := httptest.NewRecorder()
		s.download(w, httptest.NewRequest("GET", "/", nil), p, file.ID)
		if w.Code != 200 || w.Body.String() != "video" {
			t.Fatalf("download failed: %d %s", w.Code, w.Body.String())
		}
		stored, exists := s.store.State.Files[file.ID]
		if remaining == 0 {
			if exists || len(s.store.State.Runs[file.RunID].Artifacts) != 0 {
				t.Fatal("artifact remained after final download")
			}
		} else if !exists || stored.DownloadsRemaining != remaining {
			t.Fatalf("remaining = %d, file = %#v", remaining, stored)
		}
	}
	if _, err := os.Stat(filepath.Join(s.store.Root, "files", file.ID)); !os.IsNotExist(err) {
		t.Fatal("stored artifact was not removed")
	}
}

func TestSignedArtifactLinkAndExpiryCleanup(t *testing.T) {
	s := testServer(t)
	t.Setenv("TOOLDECK_PUBLIC_URL", "https://tools.example.com")
	file := addTestArtifact(t, s, time.Now().Add(time.Hour))
	link := s.artifactURL(file)
	if !strings.HasPrefix(link, "https://tools.example.com/api/public/artifacts/") {
		t.Fatal(link)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", link, nil)
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || s.store.State.Files[file.ID].DownloadsRemaining != 2 {
		t.Fatalf("signed download failed: %d %s", w.Code, w.Body.String())
	}

	stored := s.store.State.Files[file.ID]
	stored.ExpiresAt = time.Now().Add(-time.Minute)
	s.store.State.Files[file.ID] = stored
	s.updateRunArtifactLocked(stored)
	s.cleanupExpiredArtifacts()
	if _, exists := s.store.State.Files[file.ID]; exists {
		t.Fatal("expired artifact was retained")
	}
}

func TestLegacyArtifactReceivesDefaultPolicy(t *testing.T) {
	s := testServer(t)
	file := addTestArtifact(t, s, time.Time{})
	file.DownloadsRemaining = 0
	s.store.State.Files[file.ID] = file
	s.updateRunArtifactLocked(file)
	s.initializeArtifactPolicies()
	file = s.store.State.Files[file.ID]
	if file.DownloadsRemaining != 3 || file.ExpiresAt.IsZero() {
		t.Fatal(file)
	}
}
