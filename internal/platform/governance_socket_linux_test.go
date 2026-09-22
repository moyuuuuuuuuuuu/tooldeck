//go:build linux

package platform

import (
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorageInventoryKeepsLiveSocketOutOfCleanup(t *testing.T) {
	s := testServer(t)
	job := filepath.Join(s.store.Root, "jobs", "run_socket")
	if err := os.MkdirAll(job, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(job, "proxy.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(job, old, old); err != nil {
		t.Fatal(err)
	}
	items, err := s.storageInventory()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.Path == "jobs/run_socket" {
			found = true
			if !item.Special || item.Candidate {
				t.Fatalf("socket job was offered for cleanup: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("socket job missing from inventory")
	}
	w := httptest.NewRecorder()
	s.storageEndpoint(w, httptest.NewRequest("GET", "/api/v1/storage", nil), Principal{Admin: true}, []string{"storage"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"special_file":true`) {
		t.Fatalf("storage endpoint failed for active socket: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/storage/cleanup", strings.NewReader(`{"preview_token":"`+inventoryToken(items)+`","paths":["jobs/run_socket"]}`))
	s.storageEndpoint(w, request, Principal{Admin: true}, []string{"storage", "cleanup"})
	if w.Code != 409 {
		t.Fatalf("special-file job was accepted for cleanup: %d %s", w.Code, w.Body.String())
	}
	if _, err := treeSize(job); err == nil {
		t.Fatal("strict package accounting accepted a socket")
	}
}
