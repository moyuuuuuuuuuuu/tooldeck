package platform

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) playground(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "POST required")
		return
	}
	var b struct {
		Language string `json:"language"`
		Version  string `json:"version"`
		Code     string `json:"code"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	if len(b.Code) == 0 || len(b.Code) > 65536 || strings.ContainsRune(b.Code, 0) {
		fail(w, 422, "代码需1–65536字节")
		return
	}
	m := Manifest{SchemaVersion: 1, Runtime: b.Language, RuntimeVersion: b.Version, Version: "1.0.0", Title: "在线运行 · " + b.Language}
	version, e := runtimeVersion(m)
	if e != nil {
		fail(w, 422, e)
		return
	}
	m.RuntimeVersion = version
	ext := map[string]string{"php": "php", "node": "js", "python": "py", "go": "go"}[runtimeName(b.Language)]
	if ext == "" {
		fail(w, 422, "不支持的语言")
		return
	}
	m.Entrypoint = "main." + ext
	m.Name = "play-" + hash(p.owner() + "\x00" + b.Language + "\x00" + version + "\x00" + b.Code)[:48]
	m.Execution.Mode = "async"
	m.Execution.Timeout = 10
	m.Execution.Memory = 256
	limit := 65536
	m.Input = Schema{Type: "object", Properties: map[string]Schema{"stdin": {Type: "string", MaxLength: &limit}}}
	m.Output.Type = "text"
	s.store.Lock()
	defer s.store.Unlock()
	count := 0
	for _, t := range s.store.State.Tools {
		if t.Playground && t.Owner == p.owner() {
			if time.Since(t.Created) < 24*time.Hour {
				count++
			}
			if t.Manifest.Name == m.Name {
				jsonResponse(w, 200, t)
				return
			}
		}
	}
	if count >= 200 {
		fail(w, 429, "当天新增在线代码已达200份，请稍后再试")
		return
	}
	id := ID("tool_")
	root := filepath.Join(s.store.Root, "packages", id)
	if e = os.MkdirAll(root, 0755); e != nil {
		fail(w, 500, e)
		return
	}
	if e = os.WriteFile(filepath.Join(root, m.Entrypoint), []byte(b.Code), 0644); e != nil {
		os.RemoveAll(root)
		fail(w, 500, e)
		return
	}
	no := false
	t := Tool{ID: id, Owner: p.owner(), Public: &no, APIEnabled: &no, ReviewStatus: "approved", Playground: true, BuildStatus: "queued", Manifest: m, Created: time.Now()}
	s.store.State.Tools[id] = t
	if e = s.store.save(); e != nil {
		delete(s.store.State.Tools, id)
		os.RemoveAll(root)
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 202, t)
}

const playgroundWrapper = `#!/bin/sh
set +e
case "$TOOLDECK_RUNTIME" in
 php) php "$1" > /tmp/playground-stdout ;;
 node) node "$1" > /tmp/playground-stdout ;;
 python) python "$1" > /tmp/playground-stdout ;;
 go) /tool/.tooldeck-bin > /tmp/playground-stdout ;;
esac
status=$?
printf '%s\n' "$status" > /tmp/tooldeck-result.json
cat /tmp/playground-stdout >> /tmp/tooldeck-result.json
tar -cf - -C /tmp tooldeck-result.json -C /job output
`
