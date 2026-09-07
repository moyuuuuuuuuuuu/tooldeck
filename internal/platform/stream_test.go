package platform

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamWriter(t *testing.T) {
	var events []StreamEvent
	w := &eventWriter{logs: &cappedBuffer{Limit: 1024}, emit: func(e StreamEvent) { events = append(events, e) }}
	for _, p := range []string{"log\nTOOLDECK_EV", "ENT {\"type\":\"delta\",\"text\":\"你好\\n\"}\n"} {
		if _, e := w.Write([]byte(p)); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.finish(); e != nil || len(events) != 1 || events[0].Text != "你好\n" || w.logs.String() != "log\n" {
		t.Fatal(events, e)
	}
	if _, e := w.Write([]byte("TOOLDECK_EVENT invalid\n")); e == nil {
		t.Fatal("malformed event accepted")
	}
}
func TestStreamReplayAndOwnership(t *testing.T) {
	s := testServer(t)
	s.store.State.Tools["t"] = Tool{Manifest: Manifest{Name: "test"}}
	s.store.State.Runs["r"] = Run{ID: "r", ToolID: "t", Owner: "alice", Status: "succeeded", Events: []StreamEvent{{Text: "a"}, {Text: "b"}}, Result: "ab"}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Last-Event-ID", "1")
	w := httptest.NewRecorder()
	s.streamRun(w, r, Principal{UserID: "alice", Session: true, Tools: []string{"*"}}, "r")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "id: 2") || strings.Contains(w.Body.String(), "id: 1") || !strings.Contains(w.Body.String(), "event: done") {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	s.streamRun(w, r, Principal{UserID: "bob", Session: true, Tools: []string{"*"}}, "r")
	if w.Code != 404 {
		t.Fatal("cross-user stream exposed")
	}
}
