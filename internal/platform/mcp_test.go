package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpKeyTransport struct{ key string }

func (t mcpKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-API-Key", t.key)
	return http.DefaultTransport.RoundTrip(r)
}

func seedMCP(t *testing.T, s *Server) Tool {
	t.Helper()
	var m Manifest
	if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
		t.Fatal(err)
	}
	m.Execution.Mode = "async"
	tool := Tool{ID: "tool_echo", Manifest: m, BuildStatus: "ready", ReviewStatus: "approved", Owner: "author"}
	s.store.State.Tools[tool.ID] = tool
	s.store.State.Keys["key_mcp"] = Credential{ID: "key_mcp", Hash: hash("td_mcp_test"), UserID: "admin", Tools: []string{"echo"}, NeverExpires: true}
	s.store.State.Keys["key_other"] = Credential{ID: "key_other", Hash: hash("td_other_test"), Tools: []string{"*"}, NeverExpires: true}
	return tool
}

func TestMCPSessionSubmissionsDoNotUseBrowserSerialization(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	p := Principal{Session: true, UserID: "admin", Tools: []string{"echo"}}
	create := func(mcpCall bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/tools/"+tool.ID+"/runs", strings.NewReader(`{"input":{"text":"hello"}}`))
		w := httptest.NewRecorder()
		s.createRunForTransport(w, r, p, tool.ID, mcpCall)
		return w
	}
	if w := create(false); w.Code != 202 {
		t.Fatalf("first browser submission: %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		if w := create(true); w.Code != 202 || !strings.Contains(w.Body.String(), `"source":"mcp"`) {
			t.Fatalf("MCP session submission blocked by browser run: %d %s", w.Code, w.Body.String())
		}
	}
	if w := create(false); w.Code != 409 {
		t.Fatalf("browser serialization no longer enforced: %d %s", w.Code, w.Body.String())
	}
}

func connectMCP(t *testing.T, base, key string) (*mcp.ClientSession, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := mcp.NewClient(&mcp.Implementation{Name: "tooldeck-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: base + "/mcp", HTTPClient: &http.Client{Transport: mcpKeyTransport{key}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

func mcpCall(t *testing.T, session *mcp.ClientSession, ctx context.Context, name string, args any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mcpRun(t *testing.T, result *mcp.CallToolResult) Run {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool failed: %+v", result)
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var run Run
	if err := json.Unmarshal(b, &run); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestMCPClientLifecycleAndSharedKey(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	session, ctx := connectMCP(t, httpServer.URL, "td_mcp_test")
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 4 {
		t.Fatalf("catalog: %+v %v", list, err)
	}
	args := map[string]any{"input": map[string]any{"text": "hello"}, "idempotency_key": "shared-retry"}
	name := "tooldeck_run_" + tool.ID
	run := mcpRun(t, mcpCall(t, session, ctx, name, args))
	if run.Status != "queued" || run.Source != "mcp" || run.Owner != "admin" || run.CallbackURL != "" {
		t.Fatalf("unexpected run: %+v", run)
	}
	retry := mcpRun(t, mcpCall(t, session, ctx, name, args))
	if retry.ID != run.ID {
		t.Fatal("MCP retry created another job")
	}
	args["input"] = map[string]any{"text": "different"}
	if !mcpCall(t, session, ctx, name, args).IsError {
		t.Fatal("conflicting retry accepted")
	}
	if !mcpCall(t, session, ctx, name, map[string]any{"input": map[string]any{"text": 123}}).IsError {
		t.Fatal("invalid input accepted")
	}
	if !mcpCall(t, session, ctx, name, map[string]any{"env": map[string]any{"UNDECLARED": "secret"}}).IsError {
		t.Fatal("undeclared env accepted")
	}
	if !mcpCall(t, session, ctx, name, map[string]any{"callback_url": "http://127.0.0.1/callback"}).IsError {
		t.Fatal("unsafe callback accepted")
	}
	if !mcpCall(t, session, ctx, name, map[string]any{"callback_secret": "secret"}).IsError {
		t.Fatal("orphan callback secret accepted")
	}
	if !mcpCall(t, session, ctx, name, map[string]any{"unexpected": true}).IsError {
		t.Fatal("unknown parameter accepted")
	}
	// REST reads the exact same job using the same credential.
	r := httptest.NewRequest("GET", "/api/v1/runs/"+run.ID, nil)
	r.Header.Set("Authorization", "Bearer td_mcp_test")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), run.ID) {
		t.Fatalf("REST cannot read MCP job: %s", w.Body.String())
	}
	// Async REST still requires its existing callback contract.
	r = httptest.NewRequest("POST", "/api/v1/tools/"+tool.ID+"/runs", strings.NewReader(`{"input":{"text":"hello"}}`))
	r.Header.Set("X-API-Key", "td_mcp_test")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 422 {
		t.Fatalf("REST async contract changed: %d", w.Code)
	}
	// Switch to sync with streaming to keep REST submission non-blocking.
	s.store.Lock()
	tool.Manifest.Execution.Mode = "sync"
	tool.Manifest.Execution.Stream = true
	s.store.State.Tools[tool.ID] = tool
	s.store.Unlock()
	r = httptest.NewRequest("POST", "/api/v1/tools/"+tool.ID+"/runs", strings.NewReader(`{"input":{"text":"hello"}}`))
	r.Header.Set("X-API-Key", "td_mcp_test")
	r.Header.Set("Idempotency-Key", "shared-retry")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 202 || !strings.Contains(w.Body.String(), run.ID) {
		t.Fatalf("cross-transport retry: %d %s", w.Code, w.Body.String())
	}
	if got := mcpRun(t, mcpCall(t, session, ctx, "tooldeck_get_run", map[string]any{"run_id": run.ID})); got.ID != run.ID {
		t.Fatal("wrong job")
	}
	canceled := mcpRun(t, mcpCall(t, session, ctx, "tooldeck_cancel_run", map[string]any{"run_id": run.ID}))
	if canceled.Status != "canceled" {
		t.Fatalf("cancel: %+v", canceled)
	}
	s.store.Lock()
	defer s.store.Unlock()
	if len(s.store.State.Runs) != 1 {
		t.Fatal("invalid calls created jobs")
	}
	found := false
	for _, event := range s.store.State.Audit {
		if event.Actor == "admin" && event.Object == run.ID && event.Action == "POST v1/runs/cancel" {
			found = true
		}
		if strings.Contains(event.Action+event.Object+event.Error, "secret") {
			t.Fatal("audit leaked a secret")
		}
	}
	if !found {
		t.Fatal("MCP cancellation was not audited")
	}
}

func TestMCPPermissionIsolationAndRevocation(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	private := false
	hidden := tool
	hidden.ID = "tool_private"
	hidden.Manifest.Name = "private"
	hidden.Public = &private
	s.store.State.Tools[hidden.ID] = hidden
	disabled := tool
	disabled.ID = "tool_disabled"
	disabled.Manifest.Name = "disabled"
	disabled.APIEnabled = &private
	s.store.State.Tools[disabled.ID] = disabled
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	session, ctx := connectMCP(t, httpServer.URL, "td_mcp_test")
	other, otherCtx := connectMCP(t, httpServer.URL, "td_other_test")
	for _, entry := range []struct {
		session *mcp.ClientSession
		ctx     context.Context
	}{{session, ctx}, {other, otherCtx}} {
		list, err := entry.session.ListTools(entry.ctx, nil)
		if err != nil || len(list.Tools) != 4 {
			t.Fatalf("leaked tool: %+v %v", list, err)
		}
	}
	run := mcpRun(t, mcpCall(t, session, ctx, "tooldeck_run_"+tool.ID, map[string]any{"input": map[string]any{"text": "hello"}}))
	for _, name := range []string{"tooldeck_get_run", "tooldeck_cancel_run"} {
		if !mcpCall(t, other, otherCtx, name, map[string]any{"run_id": run.ID}).IsError {
			t.Fatalf("another owner can %s", name)
		}
	}
	s.store.Lock()
	tool.APIEnabled = &private
	s.store.State.Tools[tool.ID] = tool
	s.store.Unlock()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 3 {
		t.Fatalf("disabled tool still listed: %+v %v", list, err)
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "tooldeck_run_" + tool.ID, Arguments: map[string]any{}}); err == nil {
		t.Fatal("disabled tool still callable")
	}
	s.store.Lock()
	delete(s.store.State.Keys, "key_mcp")
	s.store.Unlock()
	if _, err := session.ListTools(ctx, nil); err == nil {
		t.Fatal("revoked credential retains access")
	}
}

func TestMCPTransportValidation(t *testing.T) {
	s := testServer(t)
	seedMCP(t, s)
	h := s.Handler()
	for _, tc := range []struct {
		method, key, origin, version, body string
		code                               int
	}{
		{"POST", "", "", "", `{}`, 401},
		{"POST", "bad-key", "", "", `{}`, 401},
		{"POST", "td_mcp_test", "https://evil.example", "", `{}`, 403},
		{"GET", "td_mcp_test", "null", "", "", 403},
		{"GET", "td_mcp_test", "", "", "", 405},
		{"DELETE", "td_mcp_test", "", "", "", 405},
		{"POST", "td_mcp_test", "", "invalid", `{}`, 400},
		{"POST", "td_mcp_test", "http://example.com", "2025-11-25", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202},
		{"POST", "td_mcp_test", "", "2025-11-25", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, 200},
		{"POST", "td_mcp_test", "", "2025-11-25", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, 200},
		{"POST", "td_mcp_test", "", "2025-11-25", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tooldeck_run_tool_echo","arguments":{"input":{"text":"hello"}}}}`, 200},
	} {
		r := httptest.NewRequest(tc.method, "http://example.com/mcp", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.key)
		if tc.key == "" {
			r.Header.Del("Authorization")
		}
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("MCP-Protocol-Version", tc.version)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Errorf("%s origin=%q version=%q: %d want %d: %s", tc.method, tc.origin, tc.version, w.Code, tc.code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<!doctype") {
			t.Fatal("MCP fell through to SPA")
		}
	}
}

func TestMCPEnvironmentFilesAndQueueLimits(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	tool.Manifest.EnvMode = "user"
	tool.Manifest.Env = []EnvField{{Name: "API_KEY", Required: true}}
	tool.Manifest.Input = Schema{Type: "object", Properties: map[string]Schema{"file": {Type: "string", Format: "tooldeck-file"}}, Required: []string{"file"}}
	s.store.State.Tools[tool.ID] = tool
	s.store.State.Files["file_input"] = File{ID: "file_input"}
	s.store.State.Owners["file_input"] = "key_other"
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	session, ctx := connectMCP(t, httpServer.URL, "td_mcp_test")
	name := "tooldeck_run_" + tool.ID
	args := map[string]any{"input": map[string]any{"file": "file_input"}, "env": map[string]any{"API_KEY": "temporary-test-secret"}}
	if !mcpCall(t, session, ctx, name, args).IsError {
		t.Fatal("another owner's file accepted")
	}
	s.store.Lock()
	s.store.State.Owners["file_input"] = "admin"
	s.store.Unlock()
	if !mcpCall(t, session, ctx, name, map[string]any{"input": args["input"]}).IsError {
		t.Fatal("missing required environment accepted")
	}
	args["callback_url"] = "https://callback.example/result"
	run := mcpRun(t, mcpCall(t, session, ctx, name, args))
	if !run.EnvOverridden || run.CallbackURL != args["callback_url"] {
		t.Fatalf("lost overrides/callback: %+v", run)
	}
	s.store.Lock()
	cipher := s.store.State.RunEnv[run.ID]["API_KEY"]
	if cipher == "" || cipher == "temporary-test-secret" {
		t.Fatal("environment stored in plaintext")
	}
	for i := 0; i < 100; i++ {
		id := ID("run_")
		s.store.State.Runs[id] = Run{ID: id, Status: "queued"}
	}
	s.store.Unlock()
	result := mcpCall(t, session, ctx, name, args)
	if !result.IsError {
		t.Fatal("MCP bypassed the queue limit")
	}
	b, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(b), "429") {
		t.Fatalf("wrong queue error: %s", b)
	}
	if strings.Contains(string(b), "temporary-test-secret") {
		t.Fatal("error leaked environment")
	}
}
