package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaygroundPreparation(t *testing.T) {
	s := testServer(t)
	p := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	run := func(p Principal) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.playground(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"language":"php","version":"8.0","code":"<?php echo 'hello';"}`)), p)
		return w
	}
	w := run(p)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var data struct {
		Data Tool `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &data)
	tool := data.Data
	if !tool.Playground || tool.Public == nil || *tool.Public || tool.APIEnabled == nil || *tool.APIEnabled || tool.Manifest.Network.Enabled || tool.Manifest.Execution.Timeout != 10 {
		t.Fatal("unsafe playground configuration")
	}
	if run(p).Code != 200 || len(s.store.State.Tools) != 1 {
		t.Fatal("identical code not reused")
	}
	if run(Principal{ID: "api", Tools: []string{"*"}}).Code != 403 {
		t.Fatal("API credential could prepare playground")
	}
	if canUseTool(Principal{UserID: "bob", Session: true, Tools: []string{"*"}}, tool) {
		t.Fatal("code visible to another user")
	}
}
