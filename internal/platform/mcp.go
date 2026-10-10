package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Stateless servers are rebuilt for each authenticated request so a cached
// session never retains another credential's catalog or revoked permissions.
func (s *Server) mcpHandler() http.Handler {
	transport := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		p := r.Context().Value(mcpPrincipalKey{}).(Principal)
		server := mcp.NewServer(&mcp.Implementation{Name: "tooldeck", Version: "1.0.0"}, &mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
		})
		s.store.Lock()
		available := []Tool{}
		for _, t := range s.store.State.Tools {
			if canUseTool(p, t) {
				available = append(available, t)
			}
		}
		s.store.Unlock()
		// Apply release selection before the API switch: disabling the selected
		// release must not silently expose an older version.
		for _, t := range s.releasedCatalog(p, available) {
			if t.APIEnabled != nil && !*t.APIEnabled {
				continue
			}
			id := t.ID
			envProperties := map[string]any{}
			for _, field := range t.Manifest.Env {
				envProperties[field.Name] = map[string]any{"type": "string", "description": field.Description, "maxLength": 16384}
			}
			server.AddTool(&mcp.Tool{
				Name:        "tooldeck_run_" + id,
				Title:       t.Manifest.Title,
				Description: fmt.Sprintf("%s (%s v%s). Submit a job and return run_id/status. Poll tooldeck_get_run for the result; queued is not completion.", t.Manifest.Description, t.Manifest.Name, t.Manifest.Version),
				InputSchema: map[string]any{
					"type": "object", "additionalProperties": false,
					"properties": map[string]any{
						"input":           t.Manifest.Input,
						"env":             map[string]any{"type": "object", "properties": envProperties, "additionalProperties": false, "description": "Optional declared environment overrides for this run only. Omit to use the credential owner's saved configuration."},
						"idempotency_key": map[string]any{"type": "string", "maxLength": 128, "description": "Reuse for retries of the same input/env/callback; use a new key for a new job."},
						"callback_url":    map[string]any{"type": "string", "description": "Optional public HTTPS callback for async tools. Otherwise poll tooldeck_get_run."},
						"callback_secret": map[string]any{"type": "string", "maxLength": 512},
					},
				},
			}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args struct {
					Input          map[string]any    `json:"input"`
					Env            map[string]string `json:"env"`
					IdempotencyKey string            `json:"idempotency_key"`
					CallbackURL    string            `json:"callback_url"`
					CallbackSecret string            `json:"callback_secret"`
				}
				if err := decodeMCPArguments(req, &args); err != nil {
					return mcpToolError(400, err.Error()), nil
				}
				body, err := json.Marshal(map[string]any{"input": args.Input, "env": args.Env, "callback_url": args.CallbackURL, "callback_secret": args.CallbackSecret})
				if err != nil {
					return mcpToolError(400, "invalid arguments"), nil
				}
				call := mcpRequest(r, ctx, "POST", "/api/v1/tools/"+id+"/runs", body)
				call.Header.Set("Idempotency-Key", args.IdempotencyKey)
				return s.mcpAPIResult(call, p, func(w http.ResponseWriter, r *http.Request) {
					s.createRunForTransport(w, r, p, id, true)
				}), nil
			})
		}
		for _, action := range []string{"get", "cancel"} {
			name, method, suffix := "tooldeck_get_run", "GET", ""
			description := "Query compact job status, small completed results and artifact metadata using run_id. Logs/input/stream events are omitted. Large results set result_truncated; use tooldeck_read_run_output to read results or logs in pages, or details_url with the same API Key. Poll with a delay while queued/running/canceling."
			if action == "cancel" {
				name, method, suffix = "tooldeck_cancel_run", "POST", "/cancel"
				description = "Cancel a job using run_id. Running jobs execute the existing provider cancellation hook. Check the returned status for cancel_failed."
			}
			server.AddTool(&mcp.Tool{Name: name, Description: description, InputSchema: map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"run_id"},
				"properties": map[string]any{"run_id": map[string]any{"type": "string", "minLength": 1}},
			}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args struct {
					RunID string `json:"run_id"`
				}
				if err := decodeMCPArguments(req, &args); err != nil || args.RunID == "" {
					return mcpToolError(400, "run_id is required and must be a string"), nil
				}
				call := mcpRequest(r, ctx, method, "/api/v1/runs/"+url.PathEscape(args.RunID)+suffix, nil)
				parts := []string{"runs", args.RunID}
				if suffix != "" {
					parts = append(parts, "cancel")
				}
				return s.mcpAPIResult(call, p, func(w http.ResponseWriter, r *http.Request) {
					s.runEndpoint(w, r, p, parts)
				}), nil
			})
		}
		server.AddTool(&mcp.Tool{Name: "tooldeck_read_run_output", Description: "Read completed result JSON or log text in bounded pages. offset/limit count Unicode characters. Use next_offset to continue; reading does not acknowledge or cancel a job. For full input, artifacts and streaming events, use the REST details_url with the same API Key.", InputSchema: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"run_id", "section"},
			"properties": map[string]any{
				"run_id":  map[string]any{"type": "string", "minLength": 1},
				"section": map[string]any{"type": "string", "enum": []string{"result", "logs"}},
				"offset":  map[string]any{"type": "integer", "minimum": 0, "default": 0},
				"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxOutputChars, "default": mcpDefaultOutputChars},
			},
		}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				RunID   string `json:"run_id"`
				Section string `json:"section"`
				Offset  int    `json:"offset"`
				Limit   *int   `json:"limit"`
			}
			if err := decodeMCPArguments(req, &args); err != nil {
				return mcpToolError(400, "invalid output arguments"), nil
			}
			limit := mcpDefaultOutputChars
			if args.Limit != nil {
				limit = *args.Limit
			}
			if args.RunID == "" || (args.Section != "result" && args.Section != "logs") || args.Offset < 0 || limit < 1 || limit > mcpMaxOutputChars {
				return mcpToolError(400, "run_id/section required; offset must be nonnegative; limit must be 1..4096"), nil
			}
			return s.mcpReadRunOutput(p, args.RunID, args.Section, args.Offset, limit), nil
		})
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 2 << 20})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				fail(w, 403, "cross-origin MCP requests are not allowed")
				return
			}
		}
		p, err := s.authenticate(r)
		if err != nil {
			fail(w, 401, err)
			return
		}
		auditActor(r, p.owner())
		transport.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), mcpPrincipalKey{}, p)))
	})
}

type mcpPrincipalKey struct{}
type mcpCompactRunKey struct{}

func decodeMCPArguments(req *mcp.CallToolRequest, v any) error {
	raw := req.Params.Arguments
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("arguments must be an object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	return nil
}

func mcpRequest(original *http.Request, ctx context.Context, method, path string, body []byte) *http.Request {
	r := original.Clone(context.WithValue(ctx, mcpCompactRunKey{}, true))
	r.Method = method
	r.URL.Path, r.URL.RawPath, r.URL.RawQuery = path, "", ""
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	return r
}

type mcpResponseWriter struct {
	bytes.Buffer
	header http.Header
	status int
}

func (w *mcpResponseWriter) Header() http.Header { return w.header }
func (w *mcpResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (s *Server) mcpAPIResult(r *http.Request, p Principal, handler http.HandlerFunc) *mcp.CallToolResult {
	w := &mcpResponseWriter{header: make(http.Header)}
	s.auditMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auditActor(r, p.owner())
		handler(w, r)
	})).ServeHTTP(w, r)
	var response struct {
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(w.Bytes(), &response); err != nil {
		return mcpToolError(500, "unable to read job response")
	}
	if w.status >= 400 {
		return mcpToolError(w.status, response.Message)
	}
	return mcpDataResult(response.Data)
}

func mcpDataResult(data any) *mcp.CallToolResult {
	raw, err := json.Marshal(data)
	if err != nil {
		return mcpToolError(500, "unable to encode MCP response")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: json.RawMessage(raw)}
}

func mcpToolError(status int, message string) *mcp.CallToolResult {
	message = mcpTextPrefix(message, 1024)
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}, StructuredContent: map[string]any{"status": status, "message": message}}
}
