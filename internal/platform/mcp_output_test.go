package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

type mcpCountingBackend struct {
	StateBackend
	saves atomic.Int64
}

func (b *mcpCountingBackend) Save(state State) error {
	b.saves.Add(1)
	return b.StateBackend.Save(state)
}

func postMCP(t *testing.T, s *Server, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	r.Header.Set("X-API-Key", key)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-11-25")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestMCPReadsNeverPersistButMutationsAreAudited(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	s.store.State.Runs["run_read"] = Run{ID: "run_read", ToolID: tool.ID, Owner: "admin", Status: "succeeded", Result: map[string]any{"ok": true}, Logs: "hello"}
	b := &mcpCountingBackend{StateBackend: s.store.backend}
	s.store.backend = b
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"tooldeck_get_run","arguments":{"run_id":"run_read"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"tooldeck_read_run_output","arguments":{"run_id":"run_read","section":"logs"}}}`,
	} {
		w := postMCP(t, s, "td_mcp_test", body)
		if w.Code != 200 && w.Code != 202 {
			t.Fatalf("read failed: %d %s", w.Code, w.Body.String())
		}
	}
	if got := b.saves.Load(); got != 0 {
		t.Fatalf("read requests persisted state %d times", got)
	}
	if len(s.store.State.Audit) != 0 {
		t.Fatal("read requests created audit records")
	}
	w := postMCP(t, s, "td_mcp_test", `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"tooldeck_run_tool_echo","arguments":{"input":{"text":"hello"}}}}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		Result struct {
			Data Run `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	run := response.Result.Data
	if run.ID == "" || run.Status != "queued" {
		t.Fatal("job was not submitted")
	}
	if b.saves.Load() == 0 || len(s.store.State.Audit) != 1 {
		t.Fatal("creation lost persistence or audit")
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{"name": "tooldeck_cancel_run", "arguments": map[string]any{"run_id": run.ID}}})
	w = postMCP(t, s, "td_mcp_test", string(body))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"canceled"`) {
		t.Fatal("cancel failed", w.Body.String())
	}
	if len(s.store.State.Audit) != 2 {
		t.Fatal("cancel must produce exactly one audit event")
	}
	for _, event := range s.store.State.Audit {
		if event.Actor != "admin" || (event.Action != "POST v1/tools/runs" && event.Action != "POST v1/runs/cancel") {
			t.Fatalf("wrong mutation audit: %+v", event)
		}
	}
}

func TestMCPDefaultResponseBoundsAndRESTCompatibility(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	run := Run{ID: "run_large", ToolID: tool.ID, Owner: "admin", Status: "succeeded",
		Input: map[string]any{"text": strings.Repeat("input", 100000)}, Logs: strings.Repeat("log", 300000),
		Result: strings.Repeat("结果🙂", 200000), Error: strings.Repeat("error", 20000),
		Events: []StreamEvent{{Text: "delta"}}, Artifacts: []File{},
	}
	for i := 0; i < 50; i++ {
		run.Artifacts = append(run.Artifacts, File{ID: ID("file_"), Name: strings.Repeat("产物", 4000)})
	}
	s.store.State.Runs[run.ID] = run
	w := postMCP(t, s, "td_mcp_test", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tooldeck_get_run","arguments":{"run_id":"run_large"}}}`)
	if w.Code != 200 || w.Body.Len() > 64<<10 {
		t.Fatalf("unbounded summary: %d bytes", w.Body.Len())
	}
	var response struct {
		Result struct {
			Data map[string]json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	data := response.Result.Data
	for _, field := range []string{"logs", "input", "stream_events"} {
		if _, exists := data[field]; exists {
			t.Fatalf("summary leaked bulk field %s", field)
		}
	}
	if string(data["result_truncated"]) != "true" || string(data["artifacts_truncated"]) != "true" || string(data["result"]) != "null" {
		t.Fatal("missing truncation markers")
	}
	r := httptest.NewRequest("GET", "/api/v1/runs/"+run.ID, nil)
	r.Header.Set("X-API-Key", "td_mcp_test")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	var rest struct {
		Data Run `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	if rest.Data.Logs != run.Logs || rest.Data.Result != run.Result || len(rest.Data.Events) != 1 || len(rest.Data.Artifacts) != 50 {
		t.Fatal("REST response was truncated")
	}
	// Completed small results still arrive inline, preserving their JSON type.
	s.store.State.Runs[run.ID] = Run{ID: run.ID, ToolID: tool.ID, Owner: "admin", Status: "succeeded", Result: map[string]any{"answer": 42}}
	w = postMCP(t, s, "td_mcp_test", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tooldeck_get_run","arguments":{"run_id":"run_large"}}}`)
	if !strings.Contains(w.Body.String(), `"answer":42`) || !strings.Contains(w.Body.String(), `"result_truncated":false`) {
		t.Fatal("small result missing")
	}
}

func TestMCPOutputPaginationAndAuthorization(t *testing.T) {
	s := testServer(t)
	tool := seedMCP(t, s)
	text := "你好🙂abc\n再见"
	s.store.State.Runs["run_unicode"] = Run{ID: "run_unicode", ToolID: tool.ID, Owner: "admin", Status: "succeeded", Logs: text, Result: map[string]any{"text": text}}
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	session, ctx := connectMCP(t, httpServer.URL, "td_mcp_test")
	for _, section := range []string{"logs", "result"} {
		offset := 0
		var joined strings.Builder
		for {
			result := mcpCall(t, session, ctx, "tooldeck_read_run_output", map[string]any{"run_id": "run_unicode", "section": section, "offset": offset, "limit": 3})
			if result.IsError {
				t.Fatal("page failed", result)
			}
			raw, _ := json.Marshal(result.StructuredContent)
			var page struct {
				Text string `json:"text"`
				Next *int   `json:"next_offset"`
				More bool   `json:"has_more"`
			}
			if err := json.Unmarshal(raw, &page); err != nil {
				t.Fatal(err)
			}
			if !utf8.ValidString(page.Text) || utf8.RuneCountInString(page.Text) > 3 {
				t.Fatal("invalid Unicode page")
			}
			joined.WriteString(page.Text)
			if !page.More {
				if page.Next != nil {
					t.Fatal("unexpected cursor")
				}
				break
			}
			if page.Next == nil || *page.Next <= offset {
				t.Fatal("cursor did not advance")
			}
			offset = *page.Next
		}
		want := text
		if section == "result" {
			raw, _ := json.Marshal(map[string]any{"text": text})
			want = string(raw)
		}
		if joined.String() != want {
			t.Fatalf("missing/duplicated characters for %s: %q", section, joined.String())
		}
	}
	for _, change := range []map[string]any{{"offset": -1}, {"offset": 9999}, {"offset": int(^uint(0) >> 1)}, {"limit": 0}, {"limit": 4097}, {"section": "input"}} {
		args := map[string]any{"run_id": "run_unicode", "section": "logs"}
		for k, v := range change {
			args[k] = v
		}
		if !mcpCall(t, session, ctx, "tooldeck_read_run_output", args).IsError {
			t.Fatalf("accepted invalid args: %+v", args)
		}
	}
	other, otherCtx := connectMCP(t, httpServer.URL, "td_other_test")
	if !mcpCall(t, other, otherCtx, "tooldeck_read_run_output", map[string]any{"run_id": "run_unicode", "section": "logs"}).IsError {
		t.Fatal("another identity read output")
	}
	s.store.Lock()
	key := s.store.State.Keys["key_mcp"]
	key.Tools = []string{"other"}
	s.store.State.Keys[key.ID] = key
	s.store.Unlock()
	if !mcpCall(t, session, ctx, "tooldeck_read_run_output", map[string]any{"run_id": "run_unicode", "section": "result"}).IsError {
		t.Fatal("output bypassed changed scopes")
	}
}

func TestMCPDoesNotAdvertiseListChangeNotifications(t *testing.T) {
	s := testServer(t)
	seedMCP(t, s)
	w := postMCP(t, s, "td_mcp_test", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	var response struct {
		Result struct {
			Capabilities map[string]json.RawMessage `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	tools, exists := response.Result.Capabilities["tools"]
	if w.Code != 200 || !exists || strings.Contains(string(tools), `"listChanged":true`) {
		t.Fatal("invalid tools capability", w.Body.String())
	}
	if _, exists := response.Result.Capabilities["logging"]; exists {
		t.Fatal("unsupported logging capability advertised")
	}
}
