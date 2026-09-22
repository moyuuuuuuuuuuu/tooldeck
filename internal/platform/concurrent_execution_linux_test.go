//go:build linux

package platform

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestFakeDockerProcess is invoked by a temporary docker executable in the
// integration test. It exercises the actual runner protocol without a daemon.
func TestFakeDockerProcess(t *testing.T) {
	if os.Getenv("TOOLDECK_FAKE_DOCKER") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	if args[0] == "rm" {
		return
	}
	if args[0] != "run" {
		os.Exit(2)
	}
	id := os.Getenv("TOOLDECK_RUN_ID")
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	barrier := os.Getenv("TOOLDECK_TEST_BARRIER")
	if err = os.WriteFile(filepath.Join(barrier, id), input, 0600); err != nil {
		os.Exit(2)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, _ := os.ReadDir(barrier)
		if len(files) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(3)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if os.Getenv("TOOLDECK_TEST_CANCEL_ONE") == id {
		time.Sleep(30 * time.Second)
	}
	result, _ := json.Marshal(map[string]any{"run_id": id, "input": json.RawMessage(input)})
	tw := tar.NewWriter(os.Stdout)
	if err = tw.WriteHeader(&tar.Header{Name: "tooldeck-result.json", Mode: 0600, Size: int64(len(result))}); err != nil {
		os.Exit(2)
	}
	if _, err = tw.Write(result); err != nil {
		os.Exit(2)
	}
	artifact := []byte("artifact from " + id)
	if err = tw.WriteHeader(&tar.Header{Name: "output/" + id + ".txt", Mode: 0600, Size: int64(len(artifact))}); err != nil {
		os.Exit(2)
	}
	if _, err = tw.Write(artifact); err != nil {
		os.Exit(2)
	}
	if err = tw.Close(); err != nil {
		os.Exit(2)
	}
}

func TestSameToolConcurrentExecutionAndIndependentTermination(t *testing.T) {
	s := testServer(t)
	s.queue = queueConfig{Workers: 2, PerUser: 2, Builds: 1}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	barrier := t.TempDir()
	script := "#!/bin/sh\nexec \"$TOOLDECK_TEST_BINARY\" -test.run=^TestFakeDockerProcess$ -- \"$@\"\n"
	if err = os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TOOLDECK_TEST_BINARY", bin)
	t.Setenv("TOOLDECK_FAKE_DOCKER", "1")
	t.Setenv("TOOLDECK_TEST_BARRIER", barrier)
	t.Setenv("TOOLDECK_TEST_CANCEL_ONE", "run-one")
	tool := Tool{ID: "shared", Owner: "author", Manifest: Manifest{Name: "shared", Runtime: "python", Entrypoint: "main.py"}}
	tool.Manifest.Execution.Mode = "sync"
	tool.Manifest.Execution.Timeout = 10
	tool.Manifest.Execution.Memory = 128
	s.store.State.Tools[tool.ID] = tool
	for i, id := range []string{"run-one", "run-two"} {
		s.store.State.Runs[id] = Run{ID: id, ToolID: tool.ID, Owner: "caller", Status: "queued", Input: map[string]any{"value": i}, Created: time.Now().Add(time.Duration(i) * time.Millisecond)}
	}
	first, second := s.claimRun(), s.claimRun()
	if first == nil || second == nil {
		t.Fatal("same tool executions were not both claimed")
	}
	var wg sync.WaitGroup
	for _, run := range []Run{*first, *second} {
		wg.Add(1)
		go func() { defer wg.Done(); s.execute(context.Background(), run) }()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		files, _ := os.ReadDir(barrier)
		if len(files) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both executions did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := httptest.NewRecorder()
	s.adminRunsEndpoint(w, httptest.NewRequest("POST", "/admin/runs/run-one/terminate", nil), Principal{Admin: true, Session: true}, []string{"admin", "runs", "run-one", "terminate"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	wg.Wait()
	if got := s.store.State.Runs["run-one"].Status; got != "canceled" {
		t.Fatalf("terminated run status = %s", got)
	}
	other := s.store.State.Runs["run-two"]
	if other.Status != "succeeded" {
		t.Fatalf("other run affected: %s %s", other.Status, other.Error)
	}
	result, ok := other.Result.(map[string]any)
	if !ok || result["run_id"] != "run-two" || !strings.Contains(string(mustJSON(result["input"])), `"value":1`) {
		t.Fatalf("other run result mixed: %#v", other.Result)
	}
	if len(other.Artifacts) != 1 || other.Artifacts[0].Name != "run-two.txt" || other.Artifacts[0].RunID != "run-two" {
		t.Fatalf("other run artifact mixed: %#v", other.Artifacts)
	}
	// Repeat with a timeout instead of an administrator action. A timed-out
	// execution must not change the second task's result or artifact.
	s.store.Lock()
	delete(s.activeRuns, "run-one")
	delete(s.activeRuns, "run-two")
	tool.Manifest.Execution.Timeout = 1
	s.store.State.Tools[tool.ID] = tool
	s.store.Unlock()
	secondBarrier := t.TempDir()
	t.Setenv("TOOLDECK_TEST_BARRIER", secondBarrier)
	t.Setenv("TOOLDECK_TEST_CANCEL_ONE", "run-three")
	for i, id := range []string{"run-three", "run-four"} {
		s.store.State.Runs[id] = Run{ID: id, ToolID: tool.ID, Owner: "caller", Status: "queued", Input: map[string]any{"value": i + 2}, Created: time.Now().Add(time.Duration(i) * time.Millisecond)}
	}
	third, fourth := s.claimRun(), s.claimRun()
	if third == nil || fourth == nil {
		t.Fatal("timeout scenario did not claim both runs")
	}
	for _, run := range []Run{*third, *fourth} {
		wg.Add(1)
		go func() { defer wg.Done(); s.execute(context.Background(), run) }()
	}
	wg.Wait()
	if got := s.store.State.Runs["run-three"].Status; got != "timed_out" {
		t.Fatalf("timed-out run status = %s", got)
	}
	fourthRun := s.store.State.Runs["run-four"]
	if fourthRun.Status != "succeeded" || len(fourthRun.Artifacts) != 1 || fourthRun.Artifacts[0].Name != "run-four.txt" {
		t.Fatalf("other run affected by timeout: %+v", fourthRun)
	}
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }
