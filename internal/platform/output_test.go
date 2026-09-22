package platform

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOutputDeclarations(t *testing.T) {
	for _, test := range []struct {
		input, want     string
		stream, invalid bool
	}{
		{"", "json", false, false}, {"application/json", "json", false, false},
		{"text/plain", "text", false, false}, {"text/html", "html", false, false},
		{"md", "markdown", false, false}, {"markdown", "markdown", false, false},
		{"text/markdown", "markdown", false, false}, {"text/csv", "csv", false, false},
		{"application/xml", "xml", false, false}, {"image-gallery", "image-gallery", false, false},
		{"stream", "stream", true, false}, {"text/event-stream", "stream", true, false},
		{"json", "", true, true}, {"html", "", true, true}, {"md", "", true, true},
		{"", "", true, true}, {"stream", "", false, true}, {"text/event-stream", "", false, true},
		{"text/plan", "", false, true}, {"unknown", "", false, true},
	} {
		t.Run(test.input, func(t *testing.T) {
			var m Manifest
			if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
				t.Fatal(err)
			}
			m.Output.Type, m.Execution.Stream = test.input, test.stream
			err := m.Validate()
			if (err != nil) != test.invalid {
				t.Fatalf("unexpected validation: %v", err)
			}
			if !test.invalid && m.Output.Type != test.want {
				t.Fatalf("got %s, want %s", m.Output.Type, test.want)
			}
		})
	}
}

func TestRunOutputSnapshot(t *testing.T) {
	s := testServer(t)
	p := Principal{UserID: "alice", Session: true, Tools: []string{"*"}}
	for _, format := range []string{"", "html", "md", "text", "csv", "xml", "stream"} {
		var m Manifest
		json.Unmarshal([]byte(manifestJSON), &m)
		m.Execution.Mode = "async"
		m.Execution.Stream = format == "stream"
		m.Output.Type = format
		id := "tool-" + format
		s.store.State.Tools[id] = Tool{ID: id, Owner: "alice", Manifest: m, BuildStatus: "ready"}
		w := httptest.NewRecorder()
		s.createRun(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"input":{"text":"hello"}}`)), p, id)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var response struct {
			Data Run `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		want, _ := outputType(format)
		if response.Data.OutputType != want {
			t.Fatalf("run format %q != %q", response.Data.OutputType, want)
		}
		tool := s.store.State.Tools[id]
		tool.Manifest.Output.Type = "json"
		s.store.State.Tools[id] = tool
		if s.store.State.Runs[response.Data.ID].OutputType != want {
			t.Fatal("run changed with tool declaration")
		}
		// The browser serializes versions of the same tool. Complete this case
		// before submitting the next output format.
		completed := s.store.State.Runs[response.Data.ID]
		completed.Status = "succeeded"
		s.store.State.Runs[response.Data.ID] = completed
	}
	legacy := Manifest{}
	legacy.Execution.Stream = true
	legacy.Output.Type = "json"
	if runOutputType(legacy) != "stream" {
		t.Fatal("legacy SSE compatibility lost")
	}
	legacy.Execution.Stream = false
	legacy.Output.Type = "unknown-legacy-type"
	if runOutputType(legacy) != "json" {
		t.Fatal("legacy fallback must be JSON")
	}
}
