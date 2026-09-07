package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mailSender                   func(string, string) error
	bos                          *BOSStore
	store                        *Store
	hostData, oauthURL, password string
	cancelMu                     sync.Mutex
	cancels                      map[string]context.CancelFunc
	loginMu                      sync.Mutex
	attempts                     map[string][]time.Time
}

func New(root, password string) (*Server, error) {
	if len(password) < 12 {
		return nil, errors.New("TOOLDECK_ADMIN_PASSWORD must contain at least 12 characters")
	}
	st, e := OpenStore(root)
	if e != nil {
		return nil, e
	}
	if e = bootstrapUser(st, password); e != nil {
		return nil, e
	}
	s := &Server{store: st, password: password, hostData: os.Getenv("TOOLDECK_HOST_DATA"), oauthURL: os.Getenv("TOOLDECK_OAUTH_INTROSPECTION_URL"), cancels: map[string]context.CancelFunc{}, attempts: map[string][]time.Time{}}
	if os.Getenv("TOOLDECK_STORAGE") == "bos" {
		s.bos, e = newBOSStore()
		if e != nil {
			return nil, e
		}
	} else if mode := os.Getenv("TOOLDECK_STORAGE"); mode != "" && mode != "local" {
		return nil, errors.New("TOOLDECK_STORAGE must be bos or local")
	}
	if s.oauthURL != "" {
		u, e := url.Parse(s.oauthURL)
		if e != nil || u.Scheme != "https" {
			return nil, errors.New("OAuth introspection requires HTTPS")
		}
	}
	if _, e = s.crypt(); e != nil {
		return nil, e
	}
	for _, d := range []string{"packages", "files", "jobs", "uploads"} {
		if e = os.MkdirAll(filepath.Join(st.Root, d), 0755); e != nil {
			return nil, e
		}
	}
	if s.hostData == "" {
		s.hostData = st.Root
	}
	if volume := os.Getenv("TOOLDECK_DATA_VOLUME"); volume != "" {
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		b, e := exec.CommandContext(ctx, "docker", "volume", "inspect", volume, "--format", "{{.Mountpoint}}").Output()
		if e != nil {
			return nil, e
		}
		s.hostData = strings.TrimSpace(string(b))
	}
	return s, nil
}
func (s *Server) Start(ctx context.Context) {
	go s.worker(ctx)
	go s.notificationWorker(ctx)
	go s.buildWorker(ctx)
	go s.artifactCleanupWorker(ctx)
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": v, "message": ""})
}
func fail(w http.ResponseWriter, status int, e any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "message": fmt.Sprint(e)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if r.URL.Path == "/healthz" {
			jsonResponse(w, 200, map[string]string{"status": "ok"})
			return
		}
		if r.URL.Path == "/api/core/login" && r.Method == "POST" {
			s.login(w, r)
			return
		}
		if r.URL.Path == "/api/core/password-reset/email-code" && r.Method == "POST" {
			s.sendAccountCode(w, r, true)
			return
		}
		if r.URL.Path == "/api/core/password-reset" && r.Method == "POST" {
			s.resetPassword(w, r)
			return
		}
		if r.URL.Path == "/api/core/register/email-code" && r.Method == "POST" {
			s.sendEmailCode(w, r)
			return
		}
		if r.URL.Path == "/api/core/register" && r.Method == "POST" {
			s.register(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/public/artifacts/") && r.Method == "GET" {
			s.downloadSignedArtifact(w, r, strings.TrimPrefix(r.URL.Path, "/api/public/artifacts/"))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			s.serveWeb(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/public/") {
			s.guestEndpoint(w, r)
			return
		}
		p, e := s.authenticate(r)
		if e != nil {
			fail(w, 401, e)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/core/") {
			if !p.Session {
				fail(w, 403, "user session required")
				return
			}
			s.core(w, r, p)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		parts := strings.Split(path, "/")
		switch {
		case path == "my-tools":
			s.myTools(w, r, p)
		case len(parts) == 3 && parts[0] == "tools" && parts[2] == "publication":
			s.publication(w, r, p, parts[1])
		case path == "playground":
			s.playground(w, r, p)
		case len(parts) == 3 && parts[0] == "tools" && parts[2] == "build":
			s.buildEndpoint(w, r, p, parts[1])
		case len(parts) == 1 && parts[0] == "environment":
			s.accountEnvironment(w, r, p)
		case len(parts) == 3 && parts[0] == "tools" && parts[2] == "environment":
			s.environmentEndpoint(w, r, p, parts[1])
		case path == "runtime-versions" && r.Method == "GET":
			jsonResponse(w, 200, runtimeVersions)
		case path == "review-settings" || path == "reviews" || strings.HasPrefix(path, "reviews/"):
			s.reviewEndpoint(w, r, p, parts)
		case path == "notifications" || strings.HasPrefix(path, "notifications/"):
			s.notifications(w, r, p, parts)
		case path == "tools" && r.Method == "GET":
			s.store.Lock()
			list := []Tool{}
			for _, t := range s.store.State.Tools {
				if !t.Playground && !t.Withdrawn && canUseTool(p, t) {
					visible := t
					if !p.Admin && toolOwner(t) != p.owner() {
						visible.BuildLog = ""
						visible.BuildError = ""
						visible.BuildImage = ""
						visible.Artifact = ""
					}
					list = append(list, visible)
				}
			}
			s.store.Unlock()
			sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
			jsonResponse(w, 200, list)
		case path == "tools" && r.Method == "POST":
			if !p.Session {
				fail(w, 403, "admin session required")
				return
			}
			s.uploadTool(w, r, p)
		case len(parts) == 3 && parts[0] == "tools" && parts[2] == "runs" && r.Method == "POST":
			s.createRun(w, r, p, parts[1])
		case path == "runs" && r.Method == "GET":
			s.store.Lock()
			list := []Run{}
			for _, run := range s.store.State.Runs {
				if s.canReadRun(p, run) && (r.URL.Query().Get("mine") != "1" || run.Owner == p.owner()) {
					list = append(list, run)
				}
			}
			s.store.Unlock()
			sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
			if len(list) > 200 {
				list = list[:200]
			}
			jsonResponse(w, 200, list)
		case len(parts) == 3 && parts[0] == "runs" && parts[2] == "events":
			s.streamRun(w, r, p, parts[1])
		case len(parts) >= 2 && parts[0] == "runs":
			s.runEndpoint(w, r, p, parts)
		case path == "files" && r.Method == "POST":
			s.uploadFile(w, r, p)
		case len(parts) == 2 && parts[0] == "files" && r.Method == "GET":
			s.download(w, r, p, parts[1])
		case path == "keys" || strings.HasPrefix(path, "keys/"):
			if !p.Session {
				fail(w, 403, "admin session required")
				return
			}
			s.keys(w, r, p, parts)
		case path == "secrets" || strings.HasPrefix(path, "secrets/"):
			if !p.Admin {
				fail(w, 403, "admin session required")
				return
			}
			s.secrets(w, r, parts)
		case path == "nodes" && r.Method == "GET":
			if !p.Admin {
				fail(w, 403, "admin session required")
				return
			}
			ctx, c := context.WithTimeout(r.Context(), 3*time.Second)
			defer c()
			e := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").Run()
			jsonResponse(w, 200, []any{map[string]any{"name": "local-worker", "online": e == nil, "concurrency": 1, "runtimes": []string{"php", "js", "node", "python", "go"}}})
		default:
			fail(w, 404, "not found")
		}
	})
}
func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	root := os.Getenv("TOOLDECK_WEB_DIR")
	if root == "" {
		root = "web/dist"
	}
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	p := filepath.Join(root, filepath.Clean("/"+r.URL.Path))
	if st, e := os.Stat(p); e == nil && !st.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	http.ServeFile(w, r, filepath.Join(root, "index.html"))
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
		UUID     string `json:"uuid"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	if !s.accountRate(w) {
		return
	}
	s.store.Lock()
	var user User
	for _, u := range s.store.State.Users {
		if strings.EqualFold(u.Username, strings.TrimSpace(b.Username)) || u.EmailVerified && strings.EqualFold(u.Email, strings.TrimSpace(b.Username)) {
			user = u
			break
		}
	}
	s.store.Unlock()
	if !verifyPassword(user.PasswordHash, b.Password) {
		fail(w, 401, "用户名或密码错误")
		return
	}
	token := ID("td_session_")
	k := Credential{ID: ID("session_"), UserID: user.ID, Name: user.Username, Hash: hash(token), Expires: time.Now().Add(12 * time.Hour), Session: true}
	s.store.Lock()
	if s.store.State.Users[user.ID].PasswordHash != user.PasswordHash {
		s.store.Unlock()
		fail(w, 401, "账号凭证已变更，请重新登录")
		return
	}
	s.store.State.Keys[k.ID] = k
	e := s.store.save()
	s.store.Unlock()
	if e != nil {
		fail(w, 500, "cannot persist session")
		return
	}
	jsonResponse(w, 200, map[string]string{"access_token": token, "refresh_token": ""})
}
func (s *Server) core(w http.ResponseWriter, r *http.Request, p Principal) {
	switch r.URL.Path {
	case "/api/core/logout":
		if r.Method != "POST" {
			fail(w, 405, "POST required")
			return
		}
		s.store.Lock()
		delete(s.store.State.Keys, p.ID)
		e := s.store.save()
		s.store.Unlock()
		if e != nil {
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
	case "/api/core/system/user":
		s.profile(w, r, p)
	case "/api/core/account/password":
		s.changePassword(w, r, p)
	case "/api/core/account/stats":
		s.statistics(w, r, p)
	case "/api/core/system/dictAll":
		jsonResponse(w, 200, map[string]any{})
	case "/api/core/system/menu":
		children := []any{menu("my-tools", "MyTools", "我上传的工具", "ri:folder-user-line"), menu("tools", "Tools", "发现工具", "ri:apps-line"), menu("playground", "Playground", "在线运行", "ri:code-line"), menu("profile", "Profile", "个人中心", "ri:user-line"), menu("guide", "Guide", "工具开发指引", "ri:book-line"), menu("runs", "Runs", "我的记录", "ri:history-line"), menu("credentials", "Credentials", "API 接入", "ri:key-2-line")}
		if p.Admin {
			children = append(children, menu("review", "Review", "工具审核", "ri:shield-check-line"), menu("nodes", "Nodes", "执行节点", "ri:server-line"))
		}
		jsonResponse(w, 200, children)
	default:
		fail(w, 404, "not found")
	}
}
func menu(path, name, title, icon string) any {
	return map[string]any{"path": "/tooldeck/" + path, "name": name, "component": "/tooldeck/" + path + "/index", "meta": map[string]any{"title": title, "icon": icon, "keepAlive": false}}
}
func (s *Server) uploadTool(w http.ResponseWriter, r *http.Request, p Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, 65<<20)
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		fail(w, 400, e)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, e)
		return
	}
	defer f.Close()
	id := ID("tool_")
	zipPath := filepath.Join(s.store.Root, "uploads", id+".zip")
	out, e := os.Create(zipPath)
	if e != nil {
		fail(w, 500, e)
		return
	}
	_, e = io.Copy(out, f)
	out.Close()
	defer os.Remove(zipPath)
	if e != nil {
		fail(w, 400, e)
		return
	}
	dest := filepath.Join(s.store.Root, "packages", id)
	published := false
	defer func() {
		if !published {
			os.RemoveAll(dest)
		}
	}()
	m, e := ExtractPackage(zipPath, dest)
	if e != nil {
		fail(w, 400, e)
		return
	}
	if mode := r.FormValue("mode"); mode != "" {
		m.Execution.Mode = mode
		if e := m.Validate(); e != nil {
			fail(w, 422, e)
			return
		}
	}
	public := true
	if selectedRuntime := r.FormValue("build_runtime"); selectedRuntime != "" && runtimeName(m.Runtime) != selectedRuntime {
		fail(w, 422, "所选环境与代码包 runtime 不一致")
		return
	}
	if values, present := r.Form["runtime_version"]; present && len(values) > 0 {
		m.RuntimeVersion = values[0]
	}
	if values, present := r.Form["build_command"]; present && len(values) > 0 {
		m.BuildCommand = values[0]
	}
	if value := r.FormValue("stream"); value != "" {
		if value != "true" && value != "false" {
			fail(w, 422, "invalid stream option")
			return
		}
		m.Execution.Stream = value == "true"
	}
	apiEnabled := true
	notify := false
	for field, target := range map[string]*bool{"public": &public, "api_enabled": &apiEnabled, "notify_result": &notify} {
		if value := r.FormValue(field); value != "" {
			if value != "true" && value != "false" {
				fail(w, 422, "invalid boolean option")
				return
			}
			*target = value == "true"
		}
	}
	if third := r.FormValue("third_party"); third != "" {
		if third != "true" && third != "false" {
			fail(w, 422, "invalid third_party option")
			return
		}
		m.Network.Enabled = third == "true"
		if !m.Network.Enabled {
			m.Network.AllowedHosts = nil
			if len(m.Secrets) > 0 {
				fail(w, 422, "不使用第三方服务时不能声明第三方密钥")
				return
			}
		}
	}
	if values, present := r.Form["allowed_hosts"]; present && len(values) > 0 {
		m.Network.AllowedHosts = strings.FieldsFunc(values[0], func(r rune) bool { return r == ',' || r == '\n' || r == ' ' })
	}
	if notify {
		m.Execution.Mode = "async"
	}
	if !p.Admin && len(m.Secrets) > 0 {
		fail(w, 403, "个人上传工具暂不支持引用平台第三方密钥，请通过输入参数提供自己的凭证")
		return
	}
	if e := m.Validate(); e != nil {
		fail(w, 422, e)
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	for _, t := range s.store.State.Tools {
		if t.Manifest.Name == m.Name && t.Owner != p.owner() && !(p.Admin && t.Owner == "") {
			fail(w, 409, "该工具标识已被其他用户使用，请修改 name")
			return
		}
		if t.Manifest.Name == m.Name && t.Manifest.Version == m.Version {
			fail(w, 409, "this tool version already exists")
			return
		}
	}
	// Environment requirements belong to the uploaded version; absent or empty means none.

	if e := m.Validate(); e != nil {
		fail(w, 422, e)
		return
	}
	m.RuntimeVersion, _ = runtimeVersion(m)
	tool := Tool{BuildStatus: "pending", Public: &public, ReviewStatus: "draft", ID: id, Manifest: m, Created: time.Now(), Owner: p.owner(), APIEnabled: &apiEnabled, Notify: notify}
	s.store.State.Tools[id] = tool
	if e = s.store.save(); e != nil {
		delete(s.store.State.Tools, id)
		fail(w, 500, e)
		return
	}
	published = true
	jsonResponse(w, 201, tool)
}
func (s *Server) canReadRun(p Principal, r Run) bool {
	return p.Admin || r.Owner == p.owner() && p.allows(s.store.State.Tools[r.ToolID].Manifest.Name)
}
func (s *Server) createRun(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	var b struct {
		Input map[string]any `json:"input"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	if b.Input == nil {
		b.Input = map[string]any{}
	}
	s.store.Lock()
	t, ok := s.store.State.Tools[id]
	if !ok || !canUseTool(p, t) {
		s.store.Unlock()
		fail(w, 404, "tool unavailable")
		return
	}
	if !p.Session && !p.Guest && t.APIEnabled != nil && !*t.APIEnabled {
		s.store.Unlock()
		fail(w, 403, "此工具未开放 API 调用，请在网页中使用")
		return
	}
	if t.BuildStatus != "" && t.BuildStatus != "ready" {
		s.store.Unlock()
		fail(w, 409, "工具尚未构建成功")
		return
	}
	if _, _, e := s.toolEnvironment(t, p.owner()); e != nil {
		s.store.Unlock()
		fail(w, 422, e)
		return
	}
	for name, field := range t.Manifest.Input.Properties {
		if _, ok := b.Input[name]; !ok && field.Default != nil {
			b.Input[name] = field.Default
		}
	}
	if e := validateInput(t.Manifest.Input, b.Input, "input"); e != nil {
		s.store.Unlock()
		fail(w, 422, e)
		return
	}
	if e := s.validateFileInput(t.Manifest.Input, b.Input, p.owner(), t.Manifest.UI, ""); e != nil {
		s.store.Unlock()
		fail(w, 422, e)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) > 128 {
		s.store.Unlock()
		fail(w, 400, "idempotency key too long")
		return
	}
	index := p.owner() + ":" + id + ":" + key
	var run Run
	if key != "" && s.store.State.Idempotency[index] != "" {
		run = s.store.State.Runs[s.store.State.Idempotency[index]]
		a, _ := json.Marshal(run.Input)
		bb, _ := json.Marshal(b.Input)
		if string(a) != string(bb) {
			s.store.Unlock()
			fail(w, 409, "idempotency key was used with a different input")
			return
		}
	} else {
		queued := 0
		for _, x := range s.store.State.Runs {
			if x.Status == "queued" {
				queued++
			}
		}
		if queued >= 100 {
			s.store.Unlock()
			fail(w, 429, "queue is full")
			return
		}
		run = Run{ID: ID("run_"), ToolID: id, Owner: p.owner(), Status: "queued", Input: b.Input, Artifacts: []File{}, Created: time.Now()}
		s.store.State.Runs[run.ID] = run
		if key != "" {
			s.store.State.Idempotency[index] = run.ID
		}
		if e := s.store.save(); e != nil {
			s.store.Unlock()
			fail(w, 500, e)
			return
		}
	}
	s.store.Unlock()
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		s.streamRun(w, r, p, run.ID)
		return
	}
	if t.Manifest.Execution.Mode == "sync" && !t.Manifest.Execution.Stream {
		timer := time.NewTimer(20 * time.Second)
		defer timer.Stop()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
	wait:
		for {
			select {
			case <-r.Context().Done():
				return
			case <-timer.C:
				break wait
			case <-ticker.C:
				s.store.Lock()
				run = s.store.State.Runs[run.ID]
				s.store.Unlock()
				if run.Status != "queued" && run.Status != "running" {
					break wait
				}
			}
		}
	}
	code := 200
	if run.Status == "queued" || run.Status == "running" {
		code = 202
	}
	jsonResponse(w, code, run)
}
func (s *Server) validateFileInput(sc Schema, v any, owner string, ui map[string]UI, field string) error {
	if sc.Format == "tooldeck-file" {
		id, _ := v.(string)
		f, ok := s.store.State.Files[id]
		if !ok || s.store.State.Owners[id] != owner {
			return errors.New("file is unavailable")
		}
		u := ui[field]
		if u.MaxFileSizeMB > 0 && f.Size > int64(u.MaxFileSizeMB)<<20 {
			return errors.New("file exceeds field size limit")
		}
		if len(u.Accept) > 0 {
			ok = false
			for _, mime := range u.Accept {
				if strings.Split(f.MIME, ";")[0] == mime {
					ok = true
				}
			}
			if !ok {
				return errors.New("file MIME is not allowed by field")
			}
		}
		return nil
	}
	if sc.Type == "object" {
		for k, x := range v.(map[string]any) {
			if e := s.validateFileInput(sc.Properties[k], x, owner, ui, k); e != nil {
				return e
			}
		}
	}
	if sc.Type == "array" {
		for _, x := range v.([]any) {
			if e := s.validateFileInput(*sc.Items, x, owner, ui, field); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Server) runEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	s.store.Lock()
	run, ok := s.store.State.Runs[parts[1]]
	if !ok || !s.canReadRun(p, run) {
		s.store.Unlock()
		fail(w, 404, "run unavailable")
		return
	}
	if len(parts) == 3 && parts[2] == "cancel" && r.Method == "POST" {
		if run.Status == "queued" || run.Status == "running" {
			run.Status = "canceled"
			s.store.State.Runs[run.ID] = run
			_ = s.store.save()
		}
		s.store.Unlock()
		s.cancelMu.Lock()
		if c := s.cancels[run.ID]; c != nil {
			c()
		}
		s.cancelMu.Unlock()
		jsonResponse(w, 200, run)
		return
	}
	s.store.Unlock()
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	if len(parts) == 2 {
		jsonResponse(w, 200, run)
	} else if len(parts) == 3 && parts[2] == "logs" {
		jsonResponse(w, 200, map[string]string{"logs": run.Logs})
	} else {
		fail(w, 404, "not found")
	}
}
func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request, p Principal) {
	r.Body = http.MaxBytesReader(w, r.Body, 21<<20)
	if e := r.ParseMultipartForm(4 << 20); e != nil {
		fail(w, 400, e)
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, h, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, e)
		return
	}
	defer f.Close()
	id := ID("file_")
	path := filepath.Join(s.store.Root, "files", id)
	out, e := os.Create(path)
	if e != nil {
		fail(w, 500, e)
		return
	}
	n, e := io.Copy(out, io.LimitReader(f, 20<<20+1))
	out.Close()
	if e != nil || n > 20<<20 {
		os.Remove(path)
		fail(w, 413, "file too large")
		return
	}
	file := File{ID: id, Name: filepath.Base(h.Filename), Size: n, MIME: detectMIME(path)}
	if e = s.persistFile(r.Context(), &file, path); e != nil {
		os.Remove(path)
		fail(w, 502, "object storage upload failed")
		return
	}
	if file.Storage == "bos" {
		defer os.Remove(path)
	}
	s.store.Lock()
	s.store.State.Files[id] = file
	s.store.State.Owners[id] = p.owner()
	e = s.store.save()
	s.store.Unlock()
	if e != nil {
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 201, file)
}
func (s *Server) download(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	s.store.Lock()
	f, ok := s.store.State.Files[id]
	owner := s.store.State.Owners[id]
	allowed := p.Admin || owner == p.owner()
	if f.RunID != "" {
		allowed = s.canReadRun(p, s.store.State.Runs[f.RunID])
	}
	s.store.Unlock()
	if !ok || !allowed {
		fail(w, 404, "file unavailable")
		return
	}
	if f.RunID != "" {
		s.serveRunArtifact(w, r, f)
		return
	}
	s.writeDownloadHeaders(w, f)
	body, e := s.openFile(r.Context(), f)
	if e != nil {
		fail(w, 502, "object storage download failed")
		return
	}
	defer body.Close()
	_, _ = io.Copy(w, body)
}
func (s *Server) writeDownloadHeaders(w http.ResponseWriter, f File) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Type", f.MIME)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(f.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprint(f.Size))
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	s.store.Lock()
	defer s.store.Unlock()
	switch r.Method {
	case "GET":
		list := []any{}
		for _, k := range s.store.State.Keys {
			if !k.Session && (k.UserID == p.owner() || p.Admin && k.UserID == "") {
				list = append(list, map[string]any{"id": k.ID, "name": k.Name, "tools": k.Tools, "expires_at": k.Expires, "never_expires": k.NeverExpires})
			}
		}
		jsonResponse(w, 200, list)
	case "POST":
		var b struct {
			Name         string   `json:"name"`
			Tools        []string `json:"tools"`
			Days         int      `json:"days"`
			NeverExpires bool     `json:"never_expires"`
		}
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		if b.Name == "" || len(b.Tools) == 0 || (!b.NeverExpires && (b.Days < 1 || b.Days > 365)) {
			fail(w, 422, "name, tools and days (1..365) required")
			return
		}
		token := ID("td_key_")
		k := Credential{UserID: p.owner(), ID: ID("key_"), Name: b.Name, Tools: b.Tools, Hash: hash(token), Expires: time.Now().Add(time.Duration(b.Days) * 24 * time.Hour)}
		k.NeverExpires = b.NeverExpires
		if b.NeverExpires {
			k.Expires = time.Time{}
		}
		s.store.State.Keys[k.ID] = k
		if e := s.store.save(); e != nil {
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 201, map[string]string{"id": k.ID, "key": token})
	case "DELETE":
		if len(parts) != 2 {
			fail(w, 400, "key ID required")
			return
		}
		k, ok := s.store.State.Keys[parts[1]]
		if !ok || k.Session || !(k.UserID == p.owner() || p.Admin && k.UserID == "") {
			fail(w, 404, "key not found")
			return
		}
		delete(s.store.State.Keys, parts[1])
		if e := s.store.save(); e != nil {
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
	default:
		fail(w, 405, "method not allowed")
	}
}
func (s *Server) secrets(w http.ResponseWriter, r *http.Request, parts []string) {
	s.store.Lock()
	defer s.store.Unlock()
	switch r.Method {
	case "GET":
		list := []any{}
		for _, x := range s.store.State.Secrets {
			list = append(list, map[string]any{"name": x.Name, "tools": x.Tools})
		}
		jsonResponse(w, 200, list)
	case "POST":
		var b struct {
			Name  string   `json:"name"`
			Value string   `json:"value"`
			Tools []string `json:"tools"`
		}
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		if len(b.Name) > 64 || b.Name == "" || b.Value == "" || len(b.Tools) == 0 {
			fail(w, 422, "name, value and tool grants required")
			return
		}
		c, e := s.encrypt(b.Value)
		if e != nil {
			fail(w, 500, e)
			return
		}
		s.store.State.Secrets[b.Name] = Secret{b.Name, c, b.Tools}
		if e = s.store.save(); e != nil {
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
	case "DELETE":
		if len(parts) != 2 {
			fail(w, 400, "name required")
			return
		}
		delete(s.store.State.Secrets, parts[1])
		if e := s.store.save(); e != nil {
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
	default:
		fail(w, 405, "method not allowed")
	}
}
