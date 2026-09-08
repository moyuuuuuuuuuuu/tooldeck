package platform

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAsyncAPICallbackRequiredAndStoredEncrypted(t *testing.T) {
	s := testServer(t)
	yes := true
	tool := Tool{ID: "async", Owner: "alice", Public: &yes, APIEnabled: &yes, ReviewStatus: "approved", BuildStatus: "ready", Manifest: Manifest{Name: "async", Input: Schema{Type: "object"}}}
	tool.Manifest.Execution.Mode = "async"
	tool.Manifest.Execution.Timeout = 10
	tool.Manifest.Execution.Memory = 128
	s.store.State.Tools[tool.ID] = tool
	p := Principal{ID: "key", UserID: "alice", Tools: []string{"async"}}
	call := func(body string, want int) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Idempotency-Key", "same-operation")
		s.createRun(w, r, p, tool.ID)
		if w.Code != want {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		return w
	}
	call(`{"input":{}}`, 422)
	call(`{"input":{},"callback_url":"http://callback.example/result"}`, 422)
	w := call(`{"input":{},"callback_url":"https://callback.example/result","callback_secret":"sign-me"}`, 202)
	if strings.Contains(w.Body.String(), "sign-me") {
		t.Fatal("callback secret leaked")
	}
	var run Run
	for _, item := range s.store.State.Runs {
		run = item
	}
	if run.CallbackURL == "" || strings.Contains(s.store.State.CallbackSecrets[run.ID], "sign-me") {
		t.Fatal("callback configuration not safely stored")
	}
	call(`{"input":{},"callback_url":"https://callback.example/result","callback_secret":"different"}`, 409)
}

func TestCallbackURLRejectsInternalTargets(t *testing.T) {
	for _, raw := range []string{"https://127.0.0.1/callback", "https://10.0.0.1/callback", "https://[::1]/callback", "https://user:pass@example.com/callback", "https://example.com/callback#fragment"} {
		if validateCallbackURL(raw) == nil {
			t.Fatalf("unsafe callback accepted: %s", raw)
		}
	}
	if err := validateCallbackURL("https://callback.example/result"); err != nil {
		t.Fatal(err)
	}
}

func TestCallbackRetriesThenCreatesFallbackNotification(t *testing.T) {
	s := testServer(t)
	run := Run{ID: "run_callback", ToolID: "tool", Owner: "alice", Status: "succeeded", Result: map[string]any{"ok": true}, CallbackURL: "https://callback.example/result", Created: time.Now()}
	s.store.State.Runs[run.ID] = run
	s.store.State.Tools[run.ToolID] = Tool{ID: run.ToolID, Manifest: Manifest{Name: "tool"}}
	cipher, err := s.encrypt("callback-secret")
	if err != nil {
		t.Fatal(err)
	}
	s.store.State.CallbackSecrets[run.ID] = cipher
	attempts := 0
	s.callbackSender = func(rawURL, secret string, payload callbackPayload) error {
		attempts++
		if rawURL != run.CallbackURL || secret != "callback-secret" || payload.RunID != run.ID {
			t.Fatal("unexpected callback payload")
		}
		return errors.New("receiver unavailable")
	}
	for attempt := 1; attempt <= 6; attempt++ {
		current := s.store.State.Runs[run.ID]
		current.CallbackNext = time.Time{}
		s.store.State.Runs[run.ID] = current
		s.deliverNextCallback()
		current = s.store.State.Runs[run.ID]
		if current.CallbackAttempts != attempt {
			t.Fatalf("attempt %d not recorded", attempt)
		}
		if attempt <= 5 && current.CallbackNext.Sub(time.Now()) < callbackRetryDelays[attempt-1]-time.Second {
			t.Fatalf("retry %d delay incorrect", attempt)
		}
	}
	current := s.store.State.Runs[run.ID]
	if attempts != 6 || !current.CallbackFailed || current.CallbackSent || s.store.State.CallbackSecrets[run.ID] != "" {
		t.Fatal("callback retry exhaustion not persisted")
	}
	w := httptest.NewRecorder()
	s.notifications(w, httptest.NewRequest("GET", "/", nil), Principal{UserID: "alice", Session: true}, []string{"notifications"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), run.ID) {
		t.Fatal("fallback notification missing")
	}
}

func TestCallbackSuccessStopsRetries(t *testing.T) {
	s := testServer(t)
	run := Run{ID: "run_callback", ToolID: "tool", Status: "failed", CallbackURL: "https://callback.example/result", Created: time.Now()}
	s.store.State.Runs[run.ID] = run
	s.callbackSender = func(string, string, callbackPayload) error { return nil }
	s.deliverNextCallback()
	current := s.store.State.Runs[run.ID]
	if !current.CallbackSent || current.CallbackFailed || current.CallbackAttempts != 1 {
		t.Fatal("successful callback state incorrect")
	}
	s.deliverNextCallback()
	if s.store.State.Runs[run.ID].CallbackAttempts != 1 {
		t.Fatal("successful callback repeated")
	}
}
