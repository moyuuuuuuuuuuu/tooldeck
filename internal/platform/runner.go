package platform

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type cappedBuffer struct {
	bytes.Buffer
	Limit    int
	Exceeded bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := b.Limit - b.Len()
	if left < n {
		b.Exceeded = true
		p = p[:max(0, left)]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func (s *Server) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
		s.store.Lock()
		var next *Run
		for _, r := range s.store.State.Runs {
			if r.Status == "queued" && (next == nil || r.Created.Before(next.Created)) {
				copy := r
				next = &copy
			}
		}
		if next != nil {
			now := time.Now()
			next.Started = &now
			next.Status = "running"
			s.store.State.Runs[next.ID] = *next
			if e := s.store.save(); e != nil {
				s.store.Unlock()
				continue
			}
		}
		s.store.Unlock()
		if next != nil {
			s.execute(ctx, *next)
		}
	}
}
func (s *Server) execute(parent context.Context, r Run) {
	s.store.Lock()
	tool := s.store.State.Tools[r.ToolID]
	s.store.Unlock()
	m := tool.Manifest
	ctx, cancel := context.WithTimeout(parent, time.Duration(m.Execution.Timeout)*time.Second)
	defer cancel()
	s.cancelMu.Lock()
	s.cancels[r.ID] = cancel
	s.cancelMu.Unlock()
	defer func() { s.cancelMu.Lock(); delete(s.cancels, r.ID); s.cancelMu.Unlock() }()
	s.store.Lock()
	wasCanceled := s.store.State.Runs[r.ID].Status == "canceled"
	s.store.Unlock()
	if wasCanceled {
		return
	}
	job := filepath.Join(s.store.Root, "jobs", r.ID)
	defer os.RemoveAll(job)
	err := os.MkdirAll(filepath.Join(job, "output"), 0777)
	_ = os.Chmod(filepath.Join(job, "output"), 0777)
	finish := func(e error) {
		r.Duration = time.Since(*r.Started).Milliseconds()
		if e != nil {
			r.Status = "failed"
			r.Error = e.Error()
		} else {
			r.Status = "succeeded"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.Status = "timed_out"
		}
		s.store.Lock()
		defer s.store.Unlock()
		if s.store.State.Runs[r.ID].Status == "canceled" {
			r.Status = "canceled"
		}
		r.Events = s.store.State.Runs[r.ID].Events
		s.store.State.Runs[r.ID] = r
		if e := s.store.save(); e != nil {
			fmt.Fprintln(os.Stderr, e)
		}
	}
	if err != nil {
		finish(err)
		return
	}
	if strings.HasPrefix(r.Owner, "guest:") && !guestTool(tool) {
		finish(fmt.Errorf("工具已不支持匿名运行，请登录后重试"))
		return
	}
	inputBytes, _ := json.Marshal(r.Input)
	var input map[string]any
	_ = json.Unmarshal(inputBytes, &input)
	if err = s.resolveFiles(ctx, m.Input, input, r.Owner, job); err != nil {
		finish(err)
		return
	}
	env := []string{"TOOLDECK_OUTPUT_DIR=/job/output", "HOME=/tmp", "TMPDIR=/tmp", "GOCACHE=/tmp/go-cache", "GOMODCACHE=/tmp/go-mod", "PYTHONDONTWRITEBYTECODE=1"}
	redactions := []string{}
	s.store.Lock()
	for _, name := range m.Secrets {
		secret, ok := s.store.State.Secrets[name]
		if !ok || !(Principal{Tools: secret.Tools}).allows(m.Name) {
			err = fmt.Errorf("secret %s is not configured or not granted", name)
			break
		}
		value, e := s.decrypt(secret.Cipher)
		if e != nil {
			err = e
			break
		}
		env = append(env, name+"="+value)
		redactions = append(redactions, value)
	}
	if err == nil {
		var values, hidden []string
		values, hidden, err = s.toolEnvironment(tool, r.Owner)
		env = append(env, values...)
		redactions = append(redactions, hidden...)
	}
	s.store.Unlock()
	if err != nil {
		finish(err)
		return
	}
	if m.Network.Enabled {
		socket := filepath.Join(job, "proxy.sock")
		listener, e := net.Listen("unix", socket)
		if e != nil {
			finish(e)
			return
		}
		_ = os.Chmod(socket, 0666)
		proxy := &http.Server{Handler: egressProxy(m.Network.AllowedHosts), ReadHeaderTimeout: 10 * time.Second}
		go proxy.Serve(listener)
		defer proxy.Close()
		defer os.Remove(socket)
		env = append(env, "HTTP_PROXY=http://127.0.0.1:18081", "HTTPS_PROXY=http://127.0.0.1:18081", "http_proxy=http://127.0.0.1:18081", "https_proxy=http://127.0.0.1:18081", "NODE_USE_ENV_PROXY=1")
	}
	runtime := m.Runtime
	switch runtime {
	case "js":
		runtime = "node"
	case "py":
		runtime = "python"
	case "golang":
		runtime = "go"
	}
	packagePath := filepath.Join(s.hostData, "packages", tool.ID)
	runtimeImage := "tooldeck-runtime-" + runtime + ":1"
	if tool.Artifact != "" {
		packagePath = filepath.Join(s.hostData, "artifacts", tool.Artifact)
		runtimeImage = tool.BuildImage
	}
	name := "tooldeck-run-" + r.ID
	args := []string{"run", "--name", name, "--network", "none", "--read-only", "--user", "65534:65534", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "128", "--memory", fmt.Sprintf("%dm", m.Execution.Memory), "--memory-swap", fmt.Sprintf("%dm", m.Execution.Memory), "--cpus", "1", "--tmpfs", "/tmp:rw,nosuid,nodev,exec,size=268435456", "--mount", "type=bind,src=" + packagePath + ",dst=/tool,readonly", "--mount", "type=bind,src=" + filepath.Join(s.hostData, "jobs", r.ID) + ",dst=/job,readonly", "--tmpfs", "/job/output:rw,nosuid,nodev,noexec,size=67108864,mode=1777", "--workdir", "/tool", "-i"}
	if runtime != "go" || tool.Artifact != "" {
		for i := range args {
			if strings.HasPrefix(args[i], "/tmp:rw,") {
				args[i] = strings.Replace(args[i], ",exec,", ",noexec,", 1)
			}
		}
	}
	for _, v := range env {
		args = append(args, "--env", strings.SplitN(v, "=", 2)[0])
	}
	if tool.Playground {
		if e := os.WriteFile(filepath.Join(job, "playground.sh"), []byte(playgroundWrapper), 0644); e != nil {
			finish(e)
			return
		}
		args = append(args, "--entrypoint", "/bin/sh", runtimeImage, "/job/playground.sh", m.Entrypoint)
	} else {
		args = append(args, runtimeImage, m.Entrypoint)
	}
	args = sandboxArgs(args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(os.Environ(), env...)
	b, _ := json.Marshal(input)
	cmd.Stdin = bytes.NewReader(b)
	if tool.Playground {
		stdin, _ := r.Input["stdin"].(string)
		cmd.Stdin = strings.NewReader(stdin)
	}
	stdout := &cappedBuffer{Limit: 68 << 20}
	stderr := &cappedBuffer{Limit: 1 << 20}
	cmd.Stdout = stdout
	var stream *eventWriter
	if m.Execution.Stream {
		stream = &eventWriter{logs: stderr, emit: func(e StreamEvent) {
			s.store.Lock()
			defer s.store.Unlock()
			current := s.store.State.Runs[r.ID]
			if len(current.Events) < 4096 {
				current.Events = append(current.Events, e)
				s.store.State.Runs[r.ID] = current
			} else {
				cancel()
			}
		}}
		cmd.Stderr = stream
	} else {
		cmd.Stderr = stderr
	}
	err = cmd.Run()
	if stream != nil {
		if e := stream.finish(); err == nil && e != nil {
			err = e
		}
	}

	cleanup, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = exec.CommandContext(cleanup, "docker", "rm", "-f", name).Run()
	cleanupCancel()
	r.Logs = stderr.String()
	for _, v := range redactions {
		if v != "" {
			r.Logs = strings.ReplaceAll(r.Logs, v, "[REDACTED]")
		}
	}
	if stdout.Exceeded || stderr.Exceeded {
		err = errors.New("execution output exceeded limit")
	}
	var codeError error
	if err == nil {
		var resultBytes []byte
		resultBytes, err = unpackExecution(stdout.Bytes(), job)
		if err == nil {
			if tool.Playground {
				parts := strings.SplitN(string(resultBytes), "\n", 2)
				if len(parts) != 2 {
					err = errors.New("invalid playground output")
				} else {
					exitCode, e := strconv.Atoi(parts[0])
					if e != nil {
						err = errors.New("invalid exit status")
					} else {
						r.Result = parts[1]
						if exitCode != 0 {
							codeError = fmt.Errorf("程序退出码：%d", exitCode)
						}
					}
				}
			} else if e := json.Unmarshal(resultBytes, &r.Result); e != nil {
				err = errors.New("stdout must contain exactly one JSON result; write logs to stderr")
			}
		}
	}
	if err == nil {
		r.Artifacts, err = s.collectArtifacts(ctx, job, r)
	}
	if err == nil {
		err = codeError
	}
	finish(err)
}
func (s *Server) resolveFiles(ctx context.Context, schema Schema, value any, owner, job string) error {
	switch schema.Type {
	case "object":
		m, _ := value.(map[string]any)
		for k, x := range m {
			field := schema.Properties[k]
			if field.Type == "string" && field.Format == "tooldeck-file" {
				p, e := s.materialize(ctx, x, owner, job)
				if e != nil {
					return e
				}
				m[k] = p
			} else {
				if e := s.resolveFiles(ctx, field, x, owner, job); e != nil {
					return e
				}
			}
		}
	case "array":
		a, _ := value.([]any)
		for i, x := range a {
			if schema.Items.Format == "tooldeck-file" {
				p, e := s.materialize(ctx, x, owner, job)
				if e != nil {
					return e
				}
				a[i] = p
			} else if e := s.resolveFiles(ctx, *schema.Items, x, owner, job); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Server) materialize(ctx context.Context, v any, owner, job string) (string, error) {
	id, ok := v.(string)
	if !ok {
		return "", errors.New("invalid file ID")
	}
	s.store.Lock()
	f, exists := s.store.State.Files[id]
	actualOwner := s.store.State.Owners[id]
	s.store.Unlock()
	if !exists || actualOwner != owner {
		return "", errors.New("file is unavailable")
	}
	dir := filepath.Join(job, "input")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	dest := filepath.Join(dir, id+filepath.Ext(f.Name))
	if e := s.fetchFile(ctx, f, dest); e != nil {
		return "", e
	}
	return "/job/input/" + filepath.Base(dest), nil
}
func copyFile(src, dst string) error {
	r, e := os.Open(src)
	if e != nil {
		return e
	}
	defer r.Close()
	w, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0444)
	if e != nil {
		return e
	}
	_, e = io.Copy(w, r)
	ce := w.Close()
	if e != nil {
		return e
	}
	return ce
}
func (s *Server) collectArtifacts(ctx context.Context, job string, r Run) ([]File, error) {
	var files []File
	var total int64
	err := filepath.WalkDir(filepath.Join(job, "output"), func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("artifact links are forbidden")
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("invalid artifact")
		}
		total += info.Size()
		if total > 64<<20 || len(files) >= 20 {
			return errors.New("artifact limit exceeded")
		}
		id := ID("file_")
		f := File{ID: id, Name: d.Name(), Size: info.Size(), RunID: r.ID, MIME: detectMIME(p)}
		if e = s.persistFile(ctx, &f, p); e != nil {
			return errors.New("artifact upload to object storage failed")
		}
		s.store.Lock()
		s.store.State.Files[id] = f
		s.store.State.Owners[id] = r.Owner
		s.store.Unlock()
		files = append(files, f)
		return nil
	})
	return files, err
}
func detectMIME(p string) string {
	f, e := os.Open(p)
	if e != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	b := make([]byte, 512)
	n, _ := f.Read(b)
	return http.DetectContentType(b[:n])
}

func unpackExecution(b []byte, job string) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(b))
	var result []byte
	seen := map[string]bool{}
	var total int64
	count := 0
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		name := strings.TrimSuffix(h.Name, "/")
		if !validPath(name) || seen[name] {
			return nil, errors.New("invalid execution archive")
		}
		seen[name] = true
		if h.Typeflag == tar.TypeDir {
			if name != "output" && !strings.HasPrefix(name, "output/") {
				return nil, errors.New("invalid output directory")
			}
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return nil, errors.New("artifact links and special files are forbidden")
		}
		if name == "tooldeck-result.json" {
			if h.Size > 2<<20 {
				return nil, errors.New("result exceeds 2 MB")
			}
			result, e = io.ReadAll(tr)
			if e != nil {
				return nil, e
			}
			continue
		}
		if !strings.HasPrefix(name, "output/") {
			return nil, errors.New("unexpected artifact path")
		}
		total += h.Size
		count++
		if total > 64<<20 || count > 20 {
			return nil, errors.New("artifact limit exceeded")
		}
		p := filepath.Join(job, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, e = io.Copy(f, tr)
		f.Close()
		if e != nil {
			return nil, e
		}
	}
	if result == nil {
		return nil, errors.New("missing execution result")
	}
	return result, nil
}
