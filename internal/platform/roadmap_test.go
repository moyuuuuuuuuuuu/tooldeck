package platform

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type failingBackend struct{ StateBackend }

func (f failingBackend) Save(State) error { return errors.New("disk unavailable") }
func TestQueueAtomicClaimsAndUserFairness(t *testing.T) {
	s := testServer(t)
	s.queue = queueConfig{Workers: 4, PerUser: 1, Builds: 1}
	for i, id := range []string{"a1", "a2", "b1", "c1", "d1"} {
		owner := id[:1]
		s.store.State.Runs[id] = Run{ID: id, Owner: owner, Status: "queued", Created: time.Now().Add(time.Duration(i) * time.Second)}
	}
	var wg sync.WaitGroup
	claims := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := s.claimRun(); r != nil {
				claims <- r.ID
			}
		}()
	}
	wg.Wait()
	close(claims)
	seen := map[string]bool{}
	for id := range claims {
		if seen[id] {
			t.Fatal("duplicate claim", id)
		}
		seen[id] = true
	}
	if len(seen) != 4 || seen["a2"] {
		t.Fatal("quota/fairness violated", seen)
	}
	s.store.Lock()
	r := s.store.State.Runs["a1"]
	r.Status = "canceled"
	s.store.State.Runs[r.ID] = r
	s.store.Unlock()
	if s.claimRun() != nil {
		t.Fatal("slot released before execution cleanup")
	}
	s.store.Lock()
	delete(s.activeRuns, "a1")
	s.store.Unlock()
	if r := s.claimRun(); r == nil || r.ID != "a2" {
		t.Fatal("slot not released after cleanup")
	}
}
func TestQueueClaimRollbackAndToolLimit(t *testing.T) {
	s := testServer(t)
	s.queue = queueConfig{Workers: 3, PerUser: 2, Builds: 1}
	tool := Tool{ID: "t1", Owner: "author", Manifest: Manifest{Name: "demo"}}
	tool.Manifest.Execution.Concurrency = 1
	s.store.State.Tools[tool.ID] = tool
	other := tool
	other.ID = "t2"
	s.store.State.Tools[other.ID] = other
	s.store.State.Runs["r1"] = Run{ID: "r1", ToolID: "t1", Owner: "a", Status: "queued"}
	backend := s.store.backend
	s.store.backend = failingBackend{backend}
	if s.claimRun() != nil || s.store.State.Runs["r1"].Status != "queued" {
		t.Fatal("failed save stranded claim")
	}
	s.store.backend = backend
	if s.claimRun() == nil {
		t.Fatal("claim failed")
	}
	s.store.State.Runs["r2"] = Run{ID: "r2", ToolID: "t2", Owner: "b", Status: "queued"}
	if s.claimRun() != nil {
		t.Fatal("version bypassed tool-family quota")
	}
}
func TestQueueConfiguration(t *testing.T) {
	for _, value := range []string{"0", "-1", "65", "abc"} {
		t.Setenv("TOOLDECK_WORKER_CONCURRENCY", value)
		if _, err := loadQueueConfig(); err == nil {
			t.Fatal("accepted", value)
		}
	}
}
func TestSQLiteLegacyImportBackupAndExport(t *testing.T) {
	root := t.TempDir()
	legacy := []byte(`{"tools":{},"runs":{"old":{"run_id":"old","status":"running"}},"keys":{},"files":{},"owners":{},"secrets":{},"idempotency":{},"users":{}}`)
	if err := os.WriteFile(filepath.Join(root, "state.json"), legacy, 0600); err != nil {
		t.Fatal(err)
	}
	st, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.State.Runs["old"].Status != "failed" {
		t.Fatal("restart replayed execution")
	}
	backup, err := os.ReadFile(filepath.Join(root, "state.json.pre-sqlite.bak"))
	if err != nil || string(backup) != string(legacy) {
		t.Fatal("backup mismatch", err)
	}
	st.State.Runs["new"] = Run{ID: "new", Status: "queued"}
	if err = st.save(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "state.json"), []byte("invalid stale JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err = ExportState(root, path); err != nil {
		t.Fatal(err)
	}
	if err = ExportState(root, path); err == nil {
		t.Fatal("export overwrote existing backup")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state State
	if err = json.Unmarshal(data, &state); err != nil || state.Runs["new"].Status != "queued" {
		t.Fatal("export lost record", err)
	}
	rollback := t.TempDir()
	if err = os.WriteFile(filepath.Join(rollback, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenStore(rollback)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if restored.State.Runs["new"].Status != "queued" {
		t.Fatal("rollback snapshot lost run")
	}
}
func TestSQLiteInvalidLegacyRemainsRetryable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	os.WriteFile(path, []byte("broken"), 0600)
	if st, err := OpenStore(root); err == nil {
		st.Close()
		t.Fatal("invalid legacy accepted")
	}
	os.WriteFile(path, []byte(`{"tools":{}}`), 0600)
	st, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
}
func TestControllerExclusiveLock(t *testing.T) {
	s := testServer(t)
	if other, err := New(s.store.Root, "test-password-123456"); err == nil {
		other.Close()
		t.Fatal("second controller accepted")
	}
}
func TestAuditRedactsSecretsAndRecordsActor(t *testing.T) {
	s := testServer(t)
	handler := s.auditMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { auditActor(r, "alice"); fail(w, 422, "secret-response") }))
	r := httptest.NewRequest("POST", "/api/v1/secrets?token=secret-query", strings.NewReader(`{"value":"secret-body"}`))
	r.Header.Set("Authorization", "secret-header")
	handler.ServeHTTP(httptest.NewRecorder(), r)
	raw, _ := json.Marshal(s.store.State.Audit)
	if strings.Contains(string(raw), "secret-") {
		t.Fatal("audit leaked secrets", string(raw))
	}
	if len(s.store.State.Audit) != 1 {
		t.Fatal("missing audit")
	}
	for _, v := range s.store.State.Audit {
		if v.Actor != "alice" || v.Status != 422 || v.Error == "" {
			t.Fatal(v)
		}
	}
	w := httptest.NewRecorder()
	s.auditEndpoint(w, httptest.NewRequest("GET", "/", nil), Principal{})
	if w.Code != 403 {
		t.Fatal("audit exposed")
	}
}
func TestReleaseDoesNotFollowNewVersionAndRollsBack(t *testing.T) {
	s := testServer(t)
	p := Principal{Session: true, UserID: "alice", Tools: []string{"*"}}
	v1 := Tool{ID: "v1", Owner: "alice", BuildStatus: "ready", ReviewStatus: "approved", Manifest: Manifest{Name: "demo", Version: "1"}, Created: time.Now()}
	s.store.State.Tools[v1.ID] = v1
	if err := s.store.save(); err != nil {
		t.Fatal(err)
	}
	v2 := v1
	v2.ID = "v2"
	v2.Manifest.Version = "2"
	v2.Created = v1.Created.Add(time.Hour)
	s.store.State.Tools[v2.ID] = v2
	s.store.save()
	if got := s.releasedCatalog(p, []Tool{v1, v2}); len(got) != 1 || got[0].ID != "v1" {
		t.Fatal("new version replaced default", got)
	}
	call := func(action string) {
		t.Helper()
		w := httptest.NewRecorder()
		s.releaseEndpoint(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"`+action+`","percent":50}`)), p, "v2")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	call("canary")
	first := s.releasedCatalog(p, []Tool{v1, v2})[0].ID
	for i := 0; i < 10; i++ {
		if s.releasedCatalog(p, []Tool{v1, v2})[0].ID != first {
			t.Fatal("unstable cohort")
		}
	}
	call("default")
	if s.releasedCatalog(p, []Tool{v1, v2})[0].ID != "v2" {
		t.Fatal("default not switched")
	}
	call("rollback")
	if s.releasedCatalog(p, []Tool{v1, v2})[0].ID != "v1" {
		t.Fatal("rollback failed")
	}
	v2.ReviewStatus = "pending"
	s.store.State.Tools[v2.ID] = v2
	w := httptest.NewRecorder()
	s.releaseEndpoint(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"default"}`)), p, "v2")
	if w.Code != 409 {
		t.Fatal("pending version published")
	}
}
func TestStoragePreviewProtectsReferencesAndRejectsStaleToken(t *testing.T) {
	s := testServer(t)
	old := time.Now().Add(-48 * time.Hour)
	for _, id := range []string{"orphan", "protected"} {
		path := filepath.Join(s.store.Root, "files", id)
		os.WriteFile(path, []byte("data"), 0600)
		os.Chtimes(path, old, old)
	}
	s.store.State.Files["protected"] = File{ID: "protected", Size: 4}
	items, err := s.storageInventory()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Path == "files/protected" && item.Candidate {
			t.Fatal("referenced file selected")
		}
	}
	token := inventoryToken(items)
	s.store.State.Files["orphan"] = File{ID: "orphan", Size: 4}
	call := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"preview_token": token, "paths": []string{"files/orphan"}})
		w := httptest.NewRecorder()
		s.storageEndpoint(w, httptest.NewRequest("POST", "/", strings.NewReader(string(body))), Principal{Admin: true}, []string{"storage", "cleanup"})
		return w
	}
	if w := call(token); w.Code != 409 {
		t.Fatal("stale preview accepted", w.Code)
	}
	delete(s.store.State.Files, "orphan")
	if w := call(token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err = os.Stat(filepath.Join(s.store.Root, "files", "orphan")); !os.IsNotExist(err) {
		t.Fatal("orphan not moved")
	}
	if _, err = os.Stat(filepath.Join(s.store.Root, "files", "protected")); err != nil {
		t.Fatal("referenced file removed")
	}
	matches, _ := filepath.Glob(filepath.Join(s.store.Root, "quarantine", "*", "files__orphan"))
	if len(matches) != 1 {
		t.Fatal("recovery copy missing")
	}
}
func TestStorageQuotaReservations(t *testing.T) {
	s := testServer(t)
	s.store.State.StorageQuotas = StorageQuotas{Site: 10, User: 8, Tool: 6}
	release, err := s.reserveStorage("alice", "demo", 6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.reserveStorage("bob", "demo", 1); err == nil {
		t.Fatal("tool quota bypassed")
	}
	if _, err = s.reserveStorage("alice", "other", 3); err == nil {
		t.Fatal("user quota bypassed")
	}
	if _, err = s.reserveStorage("bob", "other", 5); err == nil {
		t.Fatal("site quota bypassed")
	}
	release()
	release2, err := s.reserveStorage("bob", "demo", 6)
	if err != nil {
		t.Fatal("reservation not released", err)
	}
	release2()
}
func TestNotificationPreferencesOwnershipAndMailRetry(t *testing.T) {
	s := testServer(t)
	s.store.State.Users["alice"] = User{ID: "alice", Email: "alice@example.com", EmailVerified: true}
	s.store.State.NoticePreferences["alice"] = NoticePreferences{InApp: true, Email: true}
	s.store.notice("alice", "build", "tool", "构建完成")
	var id string
	for key := range s.store.State.Notices {
		id = key
	}
	s.noticeSender = func(string, string, string) error { return errors.New("SMTP unavailable") }
	s.deliverNotice()
	n := s.store.State.Notices[id]
	if n.Attempts != 1 || n.EmailStatus != "pending" || n.Next.IsZero() {
		t.Fatal(n)
	}
	n.Next = time.Time{}
	s.store.State.Notices[id] = n
	s.noticeSender = func(string, string, string) error { return nil }
	s.deliverNotice()
	if s.store.State.Notices[id].EmailStatus != "sent" {
		t.Fatal("mail not retried")
	}
	w := httptest.NewRecorder()
	s.inboxEndpoint(w, httptest.NewRequest("POST", "/", strings.NewReader(`{}`)), Principal{Session: true, UserID: "bob"}, []string{"inbox", id})
	if w.Code != 404 {
		t.Fatal("cross-user notification access")
	}
	s.store.State.NoticePreferences["alice"] = NoticePreferences{}
	s.store.notice("alice", "run", "run", "运行完成")
	if len(s.store.State.Notices) != 1 {
		t.Fatal("notification opt-out ignored")
	}
}

func TestWorkerPoolsExecuteConcurrentlyAndShutdown(t *testing.T) {
	s := testServer(t)
	dir := t.TempDir()
	t.Setenv("TEST_DOCKER_DIR", dir)
	script := "#!/bin/sh\ncase \"$1\" in\nrun) touch \"$TEST_DOCKER_DIR/$TOOLDECK_RUN_ID\"; while [ ! -f \"$TEST_DOCKER_DIR/release\" ]; do sleep 0.01; done; cat \"$TEST_DOCKER_DIR/result.tar\";;\nrm) exit 0;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	result := []byte(`{"ok":true}`)
	tw.WriteHeader(&tar.Header{Name: "tooldeck-result.json", Mode: 0600, Size: int64(len(result))})
	tw.Write(result)
	tw.Close()
	os.WriteFile(filepath.Join(dir, "result.tar"), archive.Bytes(), 0600)
	var manifest Manifest
	json.Unmarshal([]byte(manifestJSON), &manifest)
	tool := Tool{ID: "tool", Manifest: manifest}
	s.store.State.Tools[tool.ID] = tool
	for _, id := range []string{"a", "b"} {
		s.store.State.Runs[id] = Run{ID: id, ToolID: tool.ID, Owner: id, Status: "queued", Created: time.Now(), Input: map[string]any{"text": "test"}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, a := os.Stat(filepath.Join(dir, "a"))
		_, b := os.Stat(filepath.Join(dir, "b"))
		if a == nil && b == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			s.Wait()
			t.Fatal("two independent users did not execute concurrently")
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.WriteFile(filepath.Join(dir, "release"), nil, 0600)
	for {
		s.store.Lock()
		done := s.store.State.Runs["a"].Status == "succeeded" && s.store.State.Runs["b"].Status == "succeeded"
		s.store.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			s.Wait()
			t.Fatal("workers did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	s.Wait()
	if len(s.activeRuns) != 0 {
		t.Fatal("shutdown retained slots")
	}
}

func TestStorageCountsLinksWithoutFollowingThem(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "protected")
	if err := os.WriteFile(outside, make([]byte, 4096), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "dependency-link")); err != nil {
		t.Fatal(err)
	}
	size, err := treeSize(dir)
	if err != nil || size != int64(len(outside)) {
		t.Fatal("link was rejected or followed", size, err)
	}
}
func TestRuntimeOverrideIsExplicit(t *testing.T) {
	t.Setenv("TOOLDECK_CONTAINER_RUNTIME", "runsc")
	args := sandboxArgs([]string{"run", "image"})
	found := false
	for i, arg := range args {
		if arg == "--runtime" && i+1 < len(args) && args[i+1] == "runsc" {
			found = true
		}
	}
	if !found {
		t.Fatal("isolation runtime not passed to Docker")
	}
}
func TestStartupReconciliationHonorsInstanceOwnership(t *testing.T) {
	s := testServer(t)
	dir := t.TempDir()
	t.Setenv("TEST_DOCKER_DIR", dir)
	t.Setenv("TEST_INSTANCE", s.store.State.InstanceID)
	script := `#!/bin/sh
case "$1:$2" in
container:ls) printf 'owned\nforeign\n';;
container:inspect)
 if [ "$3" = owned ]; then label="$TEST_INSTANCE"; else label="another-instance"; fi
 printf '[{"Name":"/tooldeck-run-old","Config":{"Labels":{"tooldeck.instance":"%s"}}}]' "$label";;
container:rm) touch "$TEST_DOCKER_DIR/$4";;
esac
`
	os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := s.reconcileContainers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "owned")); err != nil {
		t.Fatal("stale owned container survived", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "foreign")); !os.IsNotExist(err) {
		t.Fatal("foreign container was touched")
	}
}

func TestInboxPreservesLegacyCallbackAlerts(t *testing.T) {
	s := testServer(t)
	p := Principal{Session: true, UserID: "alice"}
	s.store.State.Runs["legacy"] = Run{ID: "legacy", Owner: "alice", CallbackFailed: true}
	w := httptest.NewRecorder()
	s.inboxEndpoint(w, httptest.NewRequest("GET", "/", nil), p, []string{"inbox"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"legacy"`) {
		t.Fatal("legacy notification lost", w.Body.String())
	}
	w = httptest.NewRecorder()
	s.inboxEndpoint(w, httptest.NewRequest("POST", "/", nil), p, []string{"inbox", "legacy"})
	if w.Code != 200 || !s.store.State.Runs["legacy"].NotificationRead {
		t.Fatal("legacy notification not marked read")
	}
}
