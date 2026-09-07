package platform

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type EnvField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
}

func validEnv(fields []EnvField) error {
	if len(fields) > 50 {
		return fmt.Errorf("最多50个环境变量")
	}
	seen := map[string]bool{}
	for _, f := range fields {
		n := f.Name
		if !regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`).MatchString(n) || seen[n] || strings.HasPrefix(n, "TOOLDECK_") || strings.Contains(n, "PROXY") || strings.HasPrefix(n, "LD_") || strings.HasPrefix(n, "DYLD_") {
			return fmt.Errorf("无效或保留的变量名：%s", n)
		}
		switch n {
		case "PATH", "HOME", "TMPDIR", "GOCACHE", "GOMODCACHE", "NODE_OPTIONS", "NODE_USE_ENV_PROXY", "PYTHONPATH", "PYTHONHOME", "PYTHONDONTWRITEBYTECODE", "BASH_ENV", "ENV", "SHELLOPTS", "PHPRC", "PHP_INI_SCAN_DIR":
			return fmt.Errorf("保留的变量名：%s", n)
		}
		seen[n] = true
		if len([]rune(f.Description)) > 300 {
			return fmt.Errorf("变量说明最多300字")
		}
	}
	return nil
}
func toolOwner(t Tool) string {
	if t.Owner == "" {
		return "admin"
	}
	return t.Owner
}
func envKey(t Tool, subject string) string {
	mode := t.Manifest.EnvMode
	if mode == "" {
		mode = "developer"
	}
	return toolOwner(t) + ":" + t.Manifest.Name + ":" + mode + ":" + subject
}

// Caller holds the store lock. Values are encrypted even for non-sensitive fields.
func (s *Server) toolEnvironment(t Tool, owner string) ([]string, []string, error) {
	personal := t
	personal.Manifest.EnvMode = "user"
	values := s.store.State.ToolEnv[envKey(personal, owner)]
	shared := t
	shared.Manifest.EnvMode = "developer"
	defaults := s.store.State.ToolEnv[envKey(shared, toolOwner(t))]
	env := []string{}
	redactions := []string{}
	for _, f := range t.Manifest.Env {
		cipher := values[f.Name]
		if cipher == "" {
			cipher = s.store.State.AccountEnv[owner][f.Name]
		}
		if cipher == "" && t.Manifest.EnvMode != "user" {
			cipher = defaults[f.Name]
		}
		if cipher == "" {
			if f.Required {
				return nil, nil, fmt.Errorf("请先配置环境变量 %s", f.Name)
			}
			continue
		}
		v, e := s.decrypt(cipher)
		if e != nil {
			return nil, nil, fmt.Errorf("环境变量解密失败：%s", f.Name)
		}
		if f.Required && v == "" {
			return nil, nil, fmt.Errorf("请先配置环境变量 %s", f.Name)
		}
		env = append(env, f.Name+"="+v)
		if v != "" {
			redactions = append(redactions, v)
		}
	}
	return env, redactions, nil
}
func (s *Server) environmentEndpoint(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	t, ok := s.store.State.Tools[id]
	if !ok || !canUseTool(p, t) {
		fail(w, 404, "tool not found")
		return
	}
	author := p.owner() == toolOwner(t)
	sharedScope := r.URL.Query().Get("scope") == "shared"
	if sharedScope && !author {
		fail(w, 403, "只有作者能管理共享配置")
		return
	}
	scoped := t
	scoped.Manifest.EnvMode = "user"
	subject := p.owner()
	if sharedScope {
		scoped.Manifest.EnvMode = "developer"
		subject = toolOwner(t)
	}
	key := envKey(scoped, subject)
	if r.Method == "POST" {
		var b struct {
			Mode   *string           `json:"mode"`
			Fields *[]EnvField       `json:"fields"`
			Values map[string]string `json:"values"`
			Delete []string          `json:"delete"`
		}
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		if (b.Mode != nil || b.Fields != nil) && !author {
			fail(w, 403, "只有作者能调整变量定义")
			return
		}
		oldTool := t
		if b.Mode != nil {
			if *b.Mode != "developer" && *b.Mode != "user" {
				fail(w, 422, "invalid mode")
				return
			}
			t.Manifest.EnvMode = *b.Mode
		}
		if b.Fields != nil {
			if e := validEnv(*b.Fields); e != nil {
				fail(w, 422, e)
				return
			}
			t.Manifest.Env = *b.Fields
		}
		for _, f := range t.Manifest.Env {
			for _, n := range t.Manifest.Secrets {
				if f.Name == n {
					fail(w, 422, "变量与平台密钥重名")
					return
				}
			}
		}
		if s.store.State.ToolEnv == nil {
			s.store.State.ToolEnv = map[string]map[string]string{}
		}
		old := s.store.State.ToolEnv[key]
		next := map[string]string{}
		for k, v := range old {
			next[k] = v
		}
		allowed := map[string]bool{}
		for _, f := range t.Manifest.Env {
			allowed[f.Name] = true
		}
		for name, value := range b.Values {
			if !allowed[name] || len(value) > 16384 || strings.ContainsRune(value, 0) {
				fail(w, 422, "变量未声明或值无效")
				return
			}
			encrypted, e := s.encrypt(value)
			if e != nil {
				fail(w, 500, "加密失败")
				return
			}
			next[name] = encrypted
		}
		for _, name := range b.Delete {
			if !allowed[name] {
				fail(w, 422, "变量未声明")
				return
			}
			delete(next, name)
		}
		s.store.State.ToolEnv[key] = next
		s.store.State.Tools[id] = t
		if e := s.store.save(); e != nil {
			s.store.State.ToolEnv[key] = old
			s.store.State.Tools[id] = oldTool
			fail(w, 500, e)
			return
		}
	} else if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	fields := []any{}
	for _, f := range t.Manifest.Env {
		v := map[string]any{"name": f.Name, "description": f.Description, "required": f.Required, "sensitive": f.Sensitive}
		{
			cipher := s.store.State.ToolEnv[key][f.Name]
			v["configured"] = cipher != ""
		}
		fields = append(fields, v)
	}
	mode := t.Manifest.EnvMode
	if mode == "" {
		mode = "developer"
	}
	jsonResponse(w, 200, map[string]any{"mode": mode, "fields": fields, "author": author, "editable": true})
}

func (s *Server) accountEnvironment(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	owner := p.owner()
	if r.Method == "POST" {
		var b struct {
			Values map[string]string `json:"values"`
			Delete []string          `json:"delete"`
		}
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		old := s.store.State.AccountEnv[owner]
		next := map[string]string{}
		for k, v := range old {
			next[k] = v
		}
		for name, value := range b.Values {
			if e := validEnv([]EnvField{{Name: name}}); e != nil {
				fail(w, 422, e)
				return
			}
			if len(value) > 16384 || strings.ContainsRune(value, 0) {
				fail(w, 422, "变量值无效")
				return
			}
			encrypted, e := s.encrypt(value)
			if e != nil {
				fail(w, 500, "加密失败")
				return
			}
			next[name] = encrypted
		}
		for _, name := range b.Delete {
			delete(next, name)
		}
		if len(next) > 50 {
			fail(w, 422, "最多50个环境变量")
			return
		}
		if s.store.State.AccountEnv == nil {
			s.store.State.AccountEnv = map[string]map[string]string{}
		}
		s.store.State.AccountEnv[owner] = next
		if e := s.store.save(); e != nil {
			s.store.State.AccountEnv[owner] = old
			fail(w, 500, e)
			return
		}
	} else if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	names := []string{}
	for name := range s.store.State.AccountEnv[owner] {
		names = append(names, name)
	}
	sort.Strings(names)
	jsonResponse(w, 200, names)
}
