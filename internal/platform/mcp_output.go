package platform

import (
	"encoding/json"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpInlineResultBytes  = 16 << 10
	mcpMaxOutputChars     = 4096
	mcpDefaultOutputChars = 2048
	mcpMaxArtifacts       = 16
)

// Project before serialization: polling must not encode the full log, input or
// event history just to discard it later. REST responses retain their shape.
func runResponse(w http.ResponseWriter, r *http.Request, status int, run Run) {
	if compact, _ := r.Context().Value(mcpCompactRunKey{}).(bool); compact {
		jsonResponse(w, status, mcpRunSummary(run))
		return
	}
	jsonResponse(w, status, run)
}

func mcpRunSummary(run Run) map[string]any {
	data := map[string]any{
		"run_id": run.ID, "tool_id": run.ToolID, "owner": run.Owner,
		"status": run.Status, "source": run.Source, "output_type": run.OutputType,
		"env_overridden": run.EnvOverridden, "created_at": run.Created,
		"duration_ms": run.Duration, "details_url": "/api/v1/runs/" + url.PathEscape(run.ID),
		"result": nil, "result_truncated": false, "result_bytes": 0,
		"artifacts_count": len(run.Artifacts), "artifacts_truncated": len(run.Artifacts) > mcpMaxArtifacts,
	}
	if run.Started != nil {
		data["started_at"] = run.Started
	}
	if run.CallbackURL != "" {
		data["callback_url"] = run.CallbackURL
	}
	if run.Error != "" {
		data["error"] = mcpTextPrefix(run.Error, 1024)
	}
	if run.CancelError != "" {
		data["cancel_error"] = mcpTextPrefix(run.CancelError, 1024)
	}
	if !runActive(run.Status) && run.Result != nil {
		if raw, err := json.Marshal(run.Result); err == nil {
			data["result_bytes"] = len(raw)
			if len(raw) <= mcpInlineResultBytes {
				data["result"] = json.RawMessage(raw)
			} else {
				data["result_truncated"] = true
			}
		}
	}
	artifacts := []map[string]any{}
	for i, f := range run.Artifacts {
		if i == mcpMaxArtifacts {
			break
		}
		artifacts = append(artifacts, map[string]any{
			"file_id": f.ID, "name": mcpTextPrefix(f.Name, 128), "mime": mcpTextPrefix(f.MIME, 128), "size": f.Size,
			"download_url": "/api/v1/files/" + url.PathEscape(f.ID),
		})
	}
	data["artifacts"] = artifacts
	return data
}

func (s *Server) mcpReadRunOutput(p Principal, id, section string, offset, limit int) *mcp.CallToolResult {
	s.store.Lock()
	run, exists := s.store.State.Runs[id]
	allowed := exists && s.canReadRun(p, run)
	s.store.Unlock()
	if !allowed {
		return mcpToolError(404, "run unavailable")
	}
	text := run.Logs
	if section == "result" {
		if runActive(run.Status) {
			return mcpToolError(409, "result is not ready; poll tooldeck_get_run")
		}
		raw, err := json.Marshal(run.Result)
		if err != nil {
			return mcpToolError(500, "unable to read result")
		}
		text = string(raw)
	}
	// Unicode character offsets preserve Chinese/emoji across page boundaries.
	// Count without allocating a rune slice proportional to the full output.
	total := utf8.RuneCountInString(text)
	if offset > total {
		return mcpToolError(400, "offset exceeds output length")
	}
	start, end := len(text), len(text)
	index := 0
	for byteIndex := range text {
		if index == offset {
			start = byteIndex
		}
		if index >= offset && index-offset == limit {
			end = byteIndex
			break
		}
		index++
	}
	consumed := limit
	if remaining := total - offset; consumed > remaining {
		consumed = remaining
	}
	var nextOffset *int
	if offset+consumed < total {
		next := offset + consumed
		nextOffset = &next
	}
	return mcpDataResult(map[string]any{
		"run_id": id, "section": section, "offset": offset, "limit": limit,
		"total_chars": total, "next_offset": nextOffset, "has_more": nextOffset != nil,
		"text": text[start:end],
	})
}

func mcpTextPrefix(text string, maxChars int) string {
	count := 0
	for byteIndex := range text {
		if count == maxChars {
			return text[:byteIndex] + "…"
		}
		count++
	}
	return text
}
