package platform

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Schema struct {
	Type        string            `json:"type"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Format      string            `json:"format,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	Enum        []any             `json:"enum,omitempty"`
	Default     any               `json:"default,omitempty"`
	MinLength   *int              `json:"minLength,omitempty"`
	MaxLength   *int              `json:"maxLength,omitempty"`
	MinItems    *int              `json:"minItems,omitempty"`
	MaxItems    *int              `json:"maxItems,omitempty"`
	Minimum     *float64          `json:"minimum,omitempty"`
	Maximum     *float64          `json:"maximum,omitempty"`
}
type UI struct {
	Order         int      `json:"order,omitempty"`
	Widget        string   `json:"widget"`
	Placeholder   string   `json:"placeholder,omitempty"`
	Accept        []string `json:"accept,omitempty"`
	MaxFileSizeMB int      `json:"max_file_size_mb,omitempty"`
	Rows          int      `json:"rows,omitempty"`
}
type Manifest struct {
	RuntimeVersion string     `json:"runtime_version,omitempty"`
	BuildCommand   string     `json:"build_command,omitempty"`
	Env            []EnvField `json:"env,omitempty"`
	EnvMode        string     `json:"env_mode,omitempty"`
	SchemaVersion  int        `json:"schema_version"`
	Name           string     `json:"name"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Version        string     `json:"version"`
	Runtime        string     `json:"runtime"`
	Entrypoint     string     `json:"entrypoint"`
	Execution      struct {
		Mode    string `json:"mode"`
		Timeout int    `json:"timeout_seconds"`
		Memory  int    `json:"memory_mb"`
	} `json:"execution"`
	Network struct {
		Enabled      bool     `json:"enabled"`
		AllowedHosts []string `json:"allowed_hosts"`
	} `json:"network"`
	Secrets []string      `json:"secrets"`
	Input   Schema        `json:"input_schema"`
	UI      map[string]UI `json:"ui_schema"`
	Output  struct {
		Type string `json:"type"`
	} `json:"output_schema"`
}
type Tool struct {
	Playground   bool      `json:"playground,omitempty"`
	BuildStatus  string    `json:"build_status,omitempty"`
	BuildLog     string    `json:"build_log,omitempty"`
	BuildError   string    `json:"build_error,omitempty"`
	BuildImage   string    `json:"build_image,omitempty"`
	Artifact     string    `json:"artifact,omitempty"`
	Public       *bool     `json:"public,omitempty"`
	ReviewStatus string    `json:"review_status,omitempty"`
	ReviewNote   string    `json:"review_note,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	APIEnabled   *bool     `json:"api_enabled,omitempty"`
	Notify       bool      `json:"notify_result"`
	ID           string    `json:"id"`
	Manifest     Manifest  `json:"manifest"`
	Created      time.Time `json:"created_at"`
}
type Run struct {
	EmailSent        bool           `json:"email_sent"`
	EmailAttempts    int            `json:"email_attempts"`
	EmailNext        time.Time      `json:"email_next,omitempty"`
	NotificationRead bool           `json:"notification_read"`
	ID               string         `json:"run_id"`
	ToolID           string         `json:"tool_id"`
	Owner            string         `json:"owner"`
	Status           string         `json:"status"`
	Input            map[string]any `json:"input"`
	Result           any            `json:"result"`
	Error            string         `json:"error,omitempty"`
	Logs             string         `json:"logs"`
	Artifacts        []File         `json:"artifacts"`
	Created          time.Time      `json:"created_at"`
	Started          *time.Time     `json:"started_at,omitempty"`
	Duration         int64          `json:"duration_ms"`
}
type File struct {
	Storage   string `json:"storage,omitempty"`
	ObjectKey string `json:"object_key,omitempty"`
	ID        string `json:"file_id"`
	Name      string `json:"name"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	Owner     string `json:"-"`
	RunID     string `json:"run_id,omitempty"`
}

// FileOwner is persisted separately: API serialization never exposes credential identities.
type Credential struct {
	NeverExpires bool      `json:"never_expires,omitempty"`
	UserID       string    `json:"user_id,omitempty"`
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Hash         string    `json:"hash"`
	Tools        []string  `json:"tools"`
	Expires      time.Time `json:"expires_at"`
	Session      bool      `json:"session"`
}
type Secret struct {
	Name   string   `json:"name"`
	Cipher string   `json:"cipher"`
	Tools  []string `json:"tools"`
}
type State struct {
	AccountEnv     map[string]map[string]string `json:"account_env,omitempty"`
	ToolEnv        map[string]map[string]string `json:"tool_env,omitempty"`
	ReviewRequired *bool                        `json:"review_required,omitempty"`
	EmailCodes     map[string]EmailCode         `json:"email_codes"`
	Users          map[string]User              `json:"users"`
	Tools          map[string]Tool              `json:"tools"`
	Runs           map[string]Run               `json:"runs"`
	Files          map[string]File              `json:"files"`
	Owners         map[string]string            `json:"owners"`
	Keys           map[string]Credential        `json:"keys"`
	Secrets        map[string]Secret            `json:"secrets"`
	Idempotency    map[string]string            `json:"idempotency"`
}
type Store struct {
	sync.Mutex
	Root  string
	State State
}

func OpenStore(root string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	s := &Store{Root: root, State: State{Tools: map[string]Tool{}, Runs: map[string]Run{}, Files: map[string]File{}, Owners: map[string]string{}, Keys: map[string]Credential{}, Secrets: map[string]Secret{}, Idempotency: map[string]string{}, Users: map[string]User{}}}
	b, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err == nil {
		err = json.Unmarshal(b, &s.State)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for id, t := range s.State.Tools {
		if t.BuildStatus == "building" {
			t.BuildStatus = "failed"
			t.BuildError = "构建被服务重启中断，请重试"
			s.State.Tools[id] = t
		}
		if t.Public == nil && t.Owner != "" && t.Owner != "admin" {
			private := false
			t.Public = &private
			s.State.Tools[id] = t
		}
	}
	for id, r := range s.State.Runs {
		if r.Status == "running" {
			r.Status = "failed"
			r.Error = "Worker restarted; task was not retried to avoid duplicate side effects"
			s.State.Runs[id] = r
		}
	}
	return s, s.save()
}
func (s *Store) save() error {
	b, e := json.Marshal(s.State)
	if e != nil {
		return e
	}
	p := filepath.Join(s.Root, "state.json.tmp")
	f, e := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(p, filepath.Join(s.Root, "state.json"))
}
func ID(prefix string) string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return prefix + hex.EncodeToString(b)
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,79}$`)

func validPath(p string) bool {
	return p != "" && !strings.Contains(p, "\\") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../")
}
func (m *Manifest) Validate() error {
	if _, e := runtimeVersion(*m); e != nil {
		return e
	}
	if len(m.BuildCommand) > 512 || strings.ContainsRune(m.BuildCommand, 0) {
		return errors.New("invalid build_command")
	}
	if e := validEnv(m.Env); e != nil {
		return e
	}
	if m.EnvMode != "" && m.EnvMode != "developer" && m.EnvMode != "user" {
		return errors.New("invalid env_mode")
	}
	for _, f := range m.Env {
		for _, n := range m.Secrets {
			if f.Name == n {
				return errors.New("env conflicts with secrets")
			}
		}
	}
	if !namePattern.MatchString(m.Name) || !namePattern.MatchString(m.Version) {
		return errors.New("name/version must use lowercase letters, numbers, dots, underscores or hyphens")
	}
	if !validPath(m.Entrypoint) {
		return errors.New("invalid entrypoint")
	}
	if m.SchemaVersion != 1 {
		return errors.New("schema_version must be 1")
	}
	switch m.Runtime {
	case "php", "js", "node", "python", "py", "go", "golang":
	default:
		return errors.New("unsupported runtime")
	}
	if m.Execution.Mode != "sync" && m.Execution.Mode != "async" {
		return errors.New("execution.mode must be sync or async")
	}
	if m.Execution.Timeout < 1 || m.Execution.Timeout > 900 {
		return errors.New("timeout must be 1..900 seconds")
	}
	if m.Execution.Memory < 64 || m.Execution.Memory > 2048 {
		return errors.New("memory_mb must be 64..2048")
	}
	if m.Input.Type != "object" {
		return errors.New("input_schema must be an object")
	}
	for _, h := range m.Network.AllowedHosts {
		if strings.ContainsAny(h, "/:* \\@") || h == "" {
			return errors.New("allowed_hosts must contain exact domain names")
		}
	}
	if m.Network.Enabled && len(m.Network.AllowedHosts) == 0 {
		return errors.New("network requires an explicit allowed_hosts list")
	}
	for _, n := range m.Secrets {
		if !regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`).MatchString(n) || strings.HasPrefix(n, "TOOLDECK_") || strings.Contains(n, "PROXY") || n == "PATH" || n == "LD_PRELOAD" || n == "NODE_OPTIONS" || n == "PYTHONPATH" {
			return errors.New("invalid or reserved secret name")
		}
	}
	return validateSchema(m.Input, 0)
}
func validateSchema(s Schema, depth int) error {
	if depth > 8 {
		return errors.New("schema nesting too deep")
	}
	switch s.Type {
	case "object":
		for _, v := range s.Properties {
			if e := validateSchema(v, depth+1); e != nil {
				return e
			}
		}
	case "array":
		if s.Items == nil {
			return errors.New("array requires items")
		}
		return validateSchema(*s.Items, depth+1)
	case "string", "number", "integer", "boolean":
	default:
		return errors.New("unsupported schema type")
	}
	return nil
}
func validateInput(s Schema, v any, path string) error {
	fail := func() error { return fmt.Errorf("invalid input: %s (%s)", path, s.Type) }
	if len(s.Enum) > 0 {
		b, _ := json.Marshal(v)
		found := false
		for _, a := range s.Enum {
			ab, _ := json.Marshal(a)
			if string(ab) == string(b) {
				found = true
			}
		}
		if !found {
			return fail()
		}
	}
	switch s.Type {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		for _, k := range s.Required {
			if _, ok = m[k]; !ok {
				return fmt.Errorf("required field: %s.%s", path, k)
			}
		}
		for k, x := range m {
			p, ok := s.Properties[k]
			if !ok {
				return fmt.Errorf("unknown field: %s", k)
			}
			if e := validateInput(p, x, path+"."+k); e != nil {
				return e
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return fail()
		}
		if s.MinItems != nil && len(a) < *s.MinItems || s.MaxItems != nil && len(a) > *s.MaxItems {
			return fail()
		}
		for _, x := range a {
			if e := validateInput(*s.Items, x, path+"[]"); e != nil {
				return e
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok {
			return fail()
		}
		n := len([]rune(x))
		if s.MinLength != nil && n < *s.MinLength || s.MaxLength != nil && n > *s.MaxLength {
			return fail()
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fail()
		}
	case "integer", "number":
		x, ok := v.(float64)
		if !ok {
			return fail()
		}
		if s.Type == "integer" && x != float64(int64(x)) {
			return fail()
		}
		if s.Minimum != nil && x < *s.Minimum || s.Maximum != nil && x > *s.Maximum {
			return fail()
		}
	}
	return nil
}
